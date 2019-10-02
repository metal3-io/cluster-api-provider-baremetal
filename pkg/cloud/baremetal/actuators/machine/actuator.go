/*
Copyright 2019 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package machine

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/cache"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	ProviderName = "baremetal"
	// HostAnnotation is the key for an annotation that should go on a Machine to
	// reference what BareMetalHost it corresponds to.
	HostAnnotation = "metal3.io/BareMetalHost"
	requeueAfter   = time.Second * 30
)

// Add RBAC rules to access cluster-api resources
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=machines;machines/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=machineClasses,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes;events,verbs=get;list;watch;create;update;patch;delete

// RBAC to access BareMetalHost resources from metal3.io
//+kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;update;patch

// Actuator is responsible for performing machine reconciliation
type Actuator struct {
	client client.Client
}

// ActuatorParams holds parameter information for Actuator
type ActuatorParams struct {
	Client client.Client
}

// NewActuator creates a new Actuator
func NewActuator(params ActuatorParams) (*Actuator, error) {
	return &Actuator{
		client: params.Client,
	}, nil
}

// Create creates a machine and is invoked by the Machine Controller
func (a *Actuator) Create(ctx context.Context, cluster *capi.Cluster, machine *capi.Machine) error {
	log.Printf("Creating machine %v .", machine.Name)

	// load and validate the config
	if machine.Spec.ProviderSpec.Value == nil {
		return a.setError(ctx, machine, "ProviderSpec is missing")
	}
	config, err := configFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		log.Printf("Error reading ProviderSpec for machine %s: %s", machine.Name, err.Error())
		return err
	}
	err = config.IsValid()
	if err != nil {
		return a.setError(ctx, machine, err.Error())
	}

	// clear an error if one was previously set
	err = a.clearError(ctx, machine)
	if err != nil {
		return err
	}

	// look for associated BMH
	host, err := a.getHost(ctx, machine)
	if err != nil {
		return err
	}

	// none found, so try to choose one
	if host == nil {
		host, err = a.chooseHost(ctx, machine, config)
		if err != nil {
			return err
		}
		if host == nil {
			log.Printf("No available host found. Requeuing.")
			return &RequeueAfterError{RequeueAfter: requeueAfter}
		}
		log.Printf("Associating machine %s with host %s", machine.Name, host.Name)
	} else {
		log.Printf("Machine %s already associated with host %s", machine.Name, host.Name)
	}

	err = a.setHostSpec(ctx, host, machine, config)
	if err != nil {
		return err
	}

	err = a.ensureAnnotation(ctx, machine, host)
	if err != nil {
		return err
	}

	if err := a.updateMachineStatus(ctx, machine, host); err != nil {
		return err
	}

	log.Printf("Finished creating machine %v .", machine.Name)
	return nil
}

// Delete deletes a machine and is invoked by the Machine Controller
func (a *Actuator) Delete(ctx context.Context, cluster *capi.Cluster, machine *capi.Machine) error {
	log.Printf("Deleting machine %v .", machine.Name)
	host, err := a.getHost(ctx, machine)
	if err != nil {
		return err
	}
	if host != nil && host.Spec.ConsumerRef != nil {
		// don't remove the ConsumerRef if it references some other machine
		if !consumerRefMatches(host.Spec.ConsumerRef, machine) {
			log.Printf("host associated with %v, not machine %v.",
				host.Spec.ConsumerRef.Name, machine.Name)
			return nil
		}
		if host.Spec.Image != nil || host.Spec.Online || host.Spec.UserData != nil {
			host.Spec.Image = nil
			host.Spec.Online = false
			host.Spec.UserData = nil
			err = a.client.Update(ctx, host)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			return &RequeueAfterError{}
		}

		waiting := true
		switch host.Status.Provisioning.State {
		case bmh.StateRegistrationError, bmh.StateRegistering,
			bmh.StateMatchProfile, bmh.StateInspecting,
			bmh.StateReady, bmh.StateValidationError:
			// Host is not provisioned
			waiting = false
		case bmh.StateExternallyProvisioned:
			// We have no control over provisioning, so just wait until the
			// host is powered off
			waiting = host.Status.PoweredOn
		}
		if waiting {
			return &RequeueAfterError{RequeueAfter: requeueAfter}
		} else {
			host.Spec.ConsumerRef = nil
			err = a.client.Update(ctx, host)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}
	log.Printf("finished deleting machine %v.", machine.Name)
	return nil
}

// Update updates a machine and is invoked by the Machine Controller
func (a *Actuator) Update(ctx context.Context, cluster *capi.Cluster, machine *capi.Machine) error {
	log.Printf("Updating machine %v .", machine.Name)

	// clear any error message that was previously set. This method doesn't set
	// error messages yet, so we know that it's incorrect to have one here.
	err := a.clearError(ctx, machine)
	if err != nil {
		return err
	}

	host, err := a.getHost(ctx, machine)
	if err != nil {
		return err
	}
	if host == nil {
		return fmt.Errorf("host not found for machine %s", machine.Name)
	}

	err = a.ensureAnnotation(ctx, machine, host)
	if err != nil {
		return err
	}

	if err := a.updateMachineStatus(ctx, machine, host); err != nil {
		return err
	}

	log.Printf("Finished updating machine %v .", machine.Name)
	return nil
}

// Exists tests for the existence of a machine and is invoked by the Machine Controller
func (a *Actuator) Exists(ctx context.Context, cluster *capi.Cluster, machine *capi.Machine) (bool, error) {
	log.Printf("Checking if machine %v exists.", machine.Name)
	host, err := a.getHost(ctx, machine)
	if err != nil {
		return false, err
	}
	if host == nil {
		log.Printf("Machine %v does not exist.", machine.Name)
		return false, nil
	}
	log.Printf("Machine %v exists.", machine.Name)
	return true, nil
}

// The Machine Actuator interface must implement GetIP and GetKubeConfig functions as a workaround for issues
// cluster-api#158 (https://github.com/kubernetes-sigs/cluster-api/issues/158) and cluster-api#160
// (https://github.com/kubernetes-sigs/cluster-api/issues/160).

// GetIP returns IP address of the machine in the cluster.
func (a *Actuator) GetIP(cluster *capi.Cluster, machine *capi.Machine) (string, error) {
	log.Printf("Getting IP of machine %v .", machine.Name)
	return "", fmt.Errorf("TODO: Not yet implemented")
}

// GetKubeConfig gets a kubeconfig from the running control plane.
func (a *Actuator) GetKubeConfig(cluster *capi.Cluster, controlPlaneMachine *capi.Machine) (string, error) {
	log.Printf("Getting IP of machine %v .", controlPlaneMachine.Name)
	return "", fmt.Errorf("TODO: Not yet implemented")
}

// getHost gets the associated host by looking for an annotation on the machine
// that contains a reference to the host. Returns nil if not found. Assumes the
// host is in the same namespace as the machine.
func (a *Actuator) getHost(ctx context.Context, machine *capi.Machine) (*bmh.BareMetalHost, error) {
	annotations := machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return nil, nil
	}
	hostKey, ok := annotations[HostAnnotation]
	if !ok {
		return nil, nil
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		log.Printf("Error parsing annotation value \"%s\": %v", hostKey, err)
		return nil, err
	}

	host := bmh.BareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = a.client.Get(ctx, key, &host)
	if errors.IsNotFound(err) {
		log.Printf("Annotated host %s not found", hostKey)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &host, nil
}

// chooseHost iterates through known hosts and returns one that can be
// associated with the machine. It searches all hosts in case one already has an
// association with this machine.
func (a *Actuator) chooseHost(ctx context.Context, machine *capi.Machine,
	config *capbm.BareMetalMachineProviderSpec) (*bmh.BareMetalHost, error) {

	// get list of BMH
	hosts := bmh.BareMetalHostList{}
	opts := &client.ListOptions{
		Namespace: machine.Namespace,
	}

	err := a.client.List(ctx, opts, &hosts)
	if err != nil {
		return nil, err
	}

	// Using the label selector on ListOptions above doesn't seem to work.
	// I think it's because we have a local cache of all BareMetalHosts.
	labelSelector := labels.NewSelector()
	var reqs labels.Requirements
	for labelKey, labelVal := range config.HostSelector.MatchLabels {
		log.Printf("Adding requirement to match label: '%s' == '%s'", labelKey, labelVal)
		r, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal})
		if err != nil {
			log.Printf("Failed to create MatchLabel requirement, not choosing host: %v", err)
			return nil, err
		}
		reqs = append(reqs, *r)
	}
	for _, req := range config.HostSelector.MatchExpressions {
		log.Printf("Adding requirement to match label: '%s' %s '%s'", req.Key, req.Operator, req.Values)
		lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
		r, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values)
		if err != nil {
			log.Printf("Failed to create MatchExpression requirement, not choosing host: %v", err)
			return nil, err
		}
		reqs = append(reqs, *r)
	}
	labelSelector = labelSelector.Add(reqs...)

	availableHosts := []*bmh.BareMetalHost{}

	for i, host := range hosts.Items {
		if host.Available() {
			if labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
				log.Printf("Host '%s' matched hostSelector for Machine '%s'", host.Name, machine.Name)
				availableHosts = append(availableHosts, &hosts.Items[i])
			} else {
				log.Printf("Host '%s' did not match hostSelector for Machine '%s'", host.Name, machine.Name)
			}
		} else if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, machine) {
			log.Printf("found host %s with existing ConsumerRef", host.Name)
			return &hosts.Items[i], nil
		}
	}
	log.Printf("%d hosts available while choosing host for machine '%s'", len(availableHosts), machine.Name)
	if len(availableHosts) == 0 {
		return nil, nil
	}

	// choose a host at random from available hosts
	rand.Seed(time.Now().Unix())
	chosenHost := availableHosts[rand.Intn(len(availableHosts))]

	return chosenHost, nil
}

// consumerRefMatches returns a boolean based on whether the consumer
// reference and machine metadata match
func consumerRefMatches(consumer *corev1.ObjectReference, machine *capi.Machine) bool {
	if consumer.Name != machine.Name {
		return false
	}
	if consumer.Namespace != machine.Namespace {
		return false
	}
	if consumer.Kind != machine.Kind {
		return false
	}
	if consumer.APIVersion != machine.APIVersion {
		return false
	}
	return true
}

// setHostSpec will ensure the host's Spec is set according to the machine's
// details. It will then update the host via the kube API. If UserData does not
// include a Namespace, it will default to the Machine's namespace.
func (a *Actuator) setHostSpec(ctx context.Context, host *bmh.BareMetalHost, machine *capi.Machine,
	config *capbm.BareMetalMachineProviderSpec) error {

	// We only want to update the image setting if the host does not
	// already have an image.
	//
	// A host with an existing image is already provisioned and
	// upgrades are not supported at this time. To re-provision a
	// host, we must fully deprovision it and then provision it again.
	if host.Spec.Image == nil {
		host.Spec.Image = &bmh.Image{
			URL:      config.Image.URL,
			Checksum: config.Image.Checksum,
		}
		host.Spec.UserData = config.UserData
		if host.Spec.UserData != nil && host.Spec.UserData.Namespace == "" {
			host.Spec.UserData.Namespace = machine.Namespace
		}
	}

	host.Spec.ConsumerRef = &corev1.ObjectReference{
		Kind:       "Machine",
		Name:       machine.Name,
		Namespace:  machine.Namespace,
		APIVersion: machine.APIVersion,
	}

	host.Spec.Online = true
	return a.client.Update(ctx, host)
}

// ensureAnnotation makes sure the machine has an annotation that references the
// host and uses the API to update the machine if necessary.
func (a *Actuator) ensureAnnotation(ctx context.Context, machine *capi.Machine, host *bmh.BareMetalHost) error {
	annotations := machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	hostKey, err := cache.MetaNamespaceKeyFunc(host)
	if err != nil {
		log.Printf("Error parsing annotation value \"%s\": %v", hostKey, err)
		return err
	}
	existing, ok := annotations[HostAnnotation]
	if ok {
		if existing == hostKey {
			return nil
		}
		log.Printf("Warning: found stray annotation for host %s on machine %s. Overwriting.", existing, machine.Name)
	}
	annotations[HostAnnotation] = hostKey
	machine.ObjectMeta.SetAnnotations(annotations)
	return a.client.Update(ctx, machine)
}

// setError sets the ErrorMessage and ErrorReason fields on the machine and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant MachineStatusError choice.
func (a *Actuator) setError(ctx context.Context, machine *capi.Machine, message string) error {
	machine.Status.ErrorMessage = &message
	reason := capi.InvalidConfigurationMachineError
	machine.Status.ErrorReason = &reason
	log.Printf("Machine %s: %s", machine.Name, message)
	return a.client.Status().Update(ctx, machine)
}

// clearError removes the ErrorMessage from the machine's Status if set. Returns
// nil if ErrorMessage was already nil. Returns a RequeueAfterError if the
// machine was updated.
func (a *Actuator) clearError(ctx context.Context, machine *capi.Machine) error {
	if machine.Status.ErrorMessage != nil || machine.Status.ErrorReason != nil {
		machine.Status.ErrorMessage = nil
		machine.Status.ErrorReason = nil
		err := a.client.Status().Update(ctx, machine)
		if err != nil {
			return err
		}
		log.Printf("Cleared error message from machine %s", machine.Name)
		return &RequeueAfterError{}
	}
	return nil
}

// configFromProviderSpec returns a BareMetalMachineProviderSpec by
// deserializing the contents of a ProviderSpec
func configFromProviderSpec(providerSpec capi.ProviderSpec) (*capbm.BareMetalMachineProviderSpec, error) {
	if providerSpec.Value == nil {
		return nil, fmt.Errorf("ProviderSpec missing")
	}

	var config capbm.BareMetalMachineProviderSpec
	err := yaml.UnmarshalStrict(providerSpec.Value.Raw, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// updateMachineStatus updates a machine object's status.
func (a *Actuator) updateMachineStatus(ctx context.Context, machine *capi.Machine, host *bmh.BareMetalHost) error {
	addrs, err := a.nodeAddresses(host)
	if err != nil {
		return err
	}

	if err := a.applyMachineStatus(ctx, machine, addrs); err != nil {
		return err
	}

	return nil
}

func (a *Actuator) applyMachineStatus(ctx context.Context, machine *capi.Machine, addrs []corev1.NodeAddress) error {
	machineCopy := machine.DeepCopy()
	machineCopy.Status.Addresses = addrs

	if equality.Semantic.DeepEqual(machine.Status, machineCopy.Status) {
		// Status did not change
		return nil
	}

	now := metav1.Now()
	machineCopy.Status.LastUpdated = &now

	err := a.client.Status().Update(ctx, machineCopy)
	return err
}

// NodeAddresses returns a slice of corev1.NodeAddress objects for a
// given Baremetal machine.
func (a *Actuator) nodeAddresses(host *bmh.BareMetalHost) ([]corev1.NodeAddress, error) {
	addrs := []corev1.NodeAddress{}

	// If the host is nil or we have no hw details, return an empty address array.
	if host == nil || host.Status.HardwareDetails == nil {
		return addrs, nil
	}

	for _, nic := range host.Status.HardwareDetails.NIC {
		address := corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: nic.IP,
		}
		addrs = append(addrs, address)
	}

	if host.Status.HardwareDetails.Hostname != "" {
		addrs = append(addrs, corev1.NodeAddress{
			Type:    corev1.NodeHostName,
			Address: host.Status.HardwareDetails.Hostname,
		})
		addrs = append(addrs, corev1.NodeAddress{
			Type:    corev1.NodeInternalDNS,
			Address: host.Status.HardwareDetails.Hostname,
		})
	}

	return addrs, nil
}
