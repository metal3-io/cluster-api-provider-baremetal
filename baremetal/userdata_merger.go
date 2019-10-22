package baremetal

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	// TODO Why blank import ?
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mergeList casts two interfaces into interfaces lists and appends them
func mergeList(list1, list2 interface{}) (interface{}, error) {
	appendList := reflect.ValueOf(list2).Interface().([]interface{})
	outputList := reflect.ValueOf(list1).Interface().([]interface{})
	outputList = append(outputList, appendList...)
	return outputList, nil
}

// mergeMap casts two interfaces into maps and merges them. It throws an error
// in case of duplicated keys, to prevent hiding an unexpected behaviour on
// conflicts
func mergeMap(map1, map2 interface{}) (interface{}, error) {
	mergeMap := reflect.ValueOf(map2).Interface().(map[interface{}]interface{})
	outputMap := reflect.ValueOf(map1).Interface().(map[interface{}]interface{})
	// iterate over the second list
	for nested_k, nested_v := range mergeMap {
		// verify that the key is absent from the first list
		if _, ok := outputMap[nested_k]; ok {
			// Throw an error on conflict
			return nil, fmt.Errorf("Duplicated key %s\n", nested_k)
		} else {
			outputMap[nested_k] = nested_v
		}
	}
	return outputMap, nil
}

// mergeCloudInitMaps merges the two cloud init structures, performing sanity
// checks such as type matching
func mergeCloudInitMaps(yaml1Struct, yaml2Struct map[string]interface{}) (map[string]interface{}, error) {
	var err error
	yamlStructOutput := make(map[string]interface{})

	for keyMap1, valueMap1 := range yaml1Struct {
		yamlStructOutput[keyMap1] = valueMap1

		// If the value is also in map2, merge
		if valueMap2, ok := yaml2Struct[keyMap1]; ok {

			//Check the kinds match
			if reflect.TypeOf(valueMap1).Kind() != reflect.TypeOf(valueMap2).Kind() {
				return nil, fmt.Errorf("Types not matching for %s\n", keyMap1)
			}

			//perform the merge depending on the kind
			switch reflect.TypeOf(valueMap1).Kind() {
			case reflect.Slice:
				yamlStructOutput[keyMap1], err = mergeList(valueMap1, valueMap2)
			case reflect.Map:
				yamlStructOutput[keyMap1], err = mergeMap(valueMap1, valueMap2)
			default:
				return nil, fmt.Errorf("Unexpected type for %s\n", keyMap1)
			}

			if err != nil {
				return nil, fmt.Errorf("Error merging for %s: %s\n", keyMap1,
					err.Error())
			}
		}
	}

	// Add the keys that do not exist in the first structure from the second
	for keyMap2, valueMap2 := range yaml2Struct {
		if _, ok := yaml1Struct[keyMap2]; !ok {
			yamlStructOutput[keyMap2] = valueMap2
		}
	}
	return yamlStructOutput, nil
}

// unmarshal parses the yaml input into a map
func unmarshal(yamlDoc []byte) (map[string]interface{}, error) {
	var yamlStruct map[string]interface{}
	err := yaml.Unmarshal(yamlDoc, &yamlStruct)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling YAML: %s\n", err.Error())
	}
	return yamlStruct, nil
}

// fetchUserDataFromSecret parses the content of the userdata secret into a map
func (mgr *MachineManager) fetchUserDataFromSecret(ctx context.Context,
	inputUserDataSecret *corev1.SecretReference) (map[string]interface{}, error) {

	// If the namespace is not set, use the same as the machine
	if inputUserDataSecret.Namespace == "" {
		inputUserDataSecret.Namespace = mgr.BareMetalMachine.Namespace
	}

	// Fetch the secret
	tmpInputSecret := corev1.Secret{}
	key := client.ObjectKey{
		Name:      inputUserDataSecret.Name,
		Namespace: inputUserDataSecret.Namespace,
	}
	err := mgr.client.Get(ctx, key, &tmpInputSecret)
	if err != nil {
		return nil, err
	}

	// Fail if no userData in the secret
	if _, ok := tmpInputSecret.Data["userData"]; !ok {
		return nil, fmt.Errorf("No userData field in userData Secret %s",
			inputUserDataSecret.Name)
	}

	// Parse the content of the secret
	return unmarshal(tmpInputSecret.Data["userData"])
}

// mergeCloudInitWrapper merges the content of a secret into an existing map
func (mgr *MachineManager) mergeCloudInitWrapper(ctx context.Context,
	inputUserDataSecret *corev1.SecretReference,
	userDataStruct map[string]interface{},
	prepend bool) (map[string]interface{}, error) {

	// Get the content of the secret parsed
	if inputUserDataSecret != nil {
		addStruct, err := mgr.fetchUserDataFromSecret(ctx, inputUserDataSecret)
		if err != nil {
			return nil, err
		}
		//TODO: Check for conflicts in writeFiles
		if prepend {
			return mergeCloudInitMaps(addStruct, userDataStruct)
		}
		return mergeCloudInitMaps(userDataStruct, addStruct)
	}
	return userDataStruct, nil
}

// mergeCloudInit merges the content of UserDataInput in an existing cloud-init
func (mgr *MachineManager) mergeCloudInit(ctx context.Context,
	bootstrapProviderUserData []byte, inputUserData *capbm.UserDataInput) ([]byte, error) {

	// Parse the existing cloud-init into a map
	bootstrapProviderStruct, err := unmarshal(bootstrapProviderUserData)
	if err != nil {
		return nil, err
	}

	// merge the UserData to append
	bootstrapProviderStruct, err = mgr.mergeCloudInitWrapper(ctx,
		inputUserData.UserDataAppend,
		bootstrapProviderStruct, false)
	if err != nil {
		return nil, err
	}

	// merge the UserData to prepend
	bootstrapProviderStruct, err = mgr.mergeCloudInitWrapper(ctx,
		inputUserData.UserDataPrepend,
		bootstrapProviderStruct, true)
	if err != nil {
		return nil, err
	}

	// Marshal the merged cloud-init into a []byte
	bootstrapProviderUserData, err = yaml.Marshal(bootstrapProviderStruct)
	if err != nil {
		return nil, err
	}

	// Append the required first line comment
	bootstrapProviderUserData = append([]byte("#cloud-config\n"),
		bootstrapProviderUserData...)

	return bootstrapProviderUserData, nil
}

// Merge the UserData from the machine and the user
func (mgr *MachineManager) mergeUserData(ctx context.Context, host *bmh.BareMetalHost) error {
	if mgr.Machine.Spec.Bootstrap.Data != nil {

		mgr.Log.Info("Bootstrap data vailable, creating the baremetalhost secret")

		// Get the bootstrap data decoded
		bootstrapProviderUserData, err := base64.StdEncoding.DecodeString(
			*mgr.Machine.Spec.Bootstrap.Data)
		if err != nil {
			return err
		}

		// If UserData is specified in BaremetalMachine
		if mgr.BareMetalMachine.Spec.UserData != nil {
			// Process it depending on type
			switch mgr.BareMetalMachine.Spec.UserData.Type {
			case "cloud-init":
				bootstrapProviderUserData, err = mgr.mergeCloudInit(ctx,
					bootstrapProviderUserData, mgr.BareMetalMachine.Spec.UserData)
			default:
				err = fmt.Errorf("Unknown user data type %v",
					mgr.BareMetalMachine.Spec.UserData.Type)
			}
			if err != nil {
				return err
			}
		}

		// If UserData is specified in BaremetalHost
		// Process this after BaremetalMachine input to ensure that the additions
		// from BMH will either be inserted at the beginning of each list or
		// appended at the end.
		if host.Spec.UserDataInput != nil {
			// Process it depending on type
			switch host.Spec.UserDataInput.Type {
			case "cloud-init":
				bootstrapProviderUserData, err = mgr.mergeCloudInit(ctx,
					bootstrapProviderUserData, host.Spec.UserDataInput)
			default:
				err = fmt.Errorf("Unknown user data type %v",
					host.Spec.UserDataInput.Type)
			}
			if err != nil {
				return err
			}
		}

		// Create the output secret structure
		bootstrapSecret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      mgr.Machine.Name + "-user-data",
				Namespace: mgr.Machine.Namespace,
			},
			Data: map[string][]byte{
				"userData": bootstrapProviderUserData,
			},
			Type: "Opaque",
		}

		// Try to get the existing secret to replace it or create a new one
		tmpBootstrapSecret := corev1.Secret{}
		key := client.ObjectKey{
			Name:      mgr.Machine.Name + "-user-data",
			Namespace: mgr.Machine.Namespace,
		}
		err = mgr.client.Get(ctx, key, &tmpBootstrapSecret)
		if apiErrors.IsNotFound(err) {
			// Create the secret with use data
			err = mgr.client.Create(ctx, bootstrapSecret)
		} else if err != nil {
			return err
		} else {
			// Update the secret with use data
			err = mgr.client.Update(ctx, bootstrapSecret)
		}

		if err != nil {
			mgr.Log.Info("Unable to create secret for bootstrap")
			return err
		}

		// Update the BaremetalMachine
		// The BaremetalHost update will be updated based on this
		mgr.BareMetalMachine.Status.UserData = &corev1.SecretReference{
			Name:      mgr.Machine.Name + "-user-data",
			Namespace: mgr.Machine.Namespace,
		}

	}
	return nil
}
