package remediator

import (
	"context"
	"testing"
	"time"

	bmov1 "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	mrv1 "github.com/metal3-io/cluster-api-provider-baremetal/pkg/apis/machineremediation/v1alpha1"
	"github.com/metal3-io/cluster-api-provider-baremetal/pkg/consts"
	mrtesting "github.com/metal3-io/cluster-api-provider-baremetal/pkg/utils/testing"

	mapiv1 "sigs.k8s.io/cluster-api/pkg/apis/machine/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	// Add types to scheme
	bmov1.SchemeBuilder.AddToScheme(scheme.Scheme)
	mrv1.AddToScheme(scheme.Scheme)
	mapiv1.AddToScheme(scheme.Scheme)
}

func newFakeBareMetalRemediator(recorder record.EventRecorder, objects ...runtime.Object) *BareMetalRemediator {
	fakeClient := fake.NewFakeClient(objects...)
	return &BareMetalRemediator{
		client:   fakeClient,
		recorder: recorder,
	}
}

type expectedRemediationResult struct {
	state                           mrv1.RemediationState
	hasEndTime                      bool
	bareMetalHostOnline             bool
	nodeDeleted                     bool
	machineRemediationDeleted       bool
	rebootInProgressAnnotationExist bool
	nodeRebootAnnotationExist       bool
}

func TestRemediationReboot(t *testing.T) {
	nodeOnline := mrtesting.NewNode("nodeOnline", true, "machineOnline")
	nodeOnline.Annotations = map[string]string{
		consts.AnnotationNodeMachineReboot: "",
	}

	bareMetalHostOnline := mrtesting.NewBareMetalHost("bareMetalHostOnline", true, true)
	bareMetalHostOnline.Annotations[consts.AnnotationRebootInProgress] = "true"
	machineOnline := mrtesting.NewMachine("machineOnline", nodeOnline.Name, bareMetalHostOnline.Name)

	bareMetalHostOnlineWithoutRebootAnnotation := mrtesting.NewBareMetalHost("bareMetalHostOnlineWithoutRebootAnnotation", true, true)
	machineOnlineWithoutRebootAnnotation := mrtesting.NewMachine("machineOnlineWithoutRebootAnnotation", nodeOnline.Name, bareMetalHostOnlineWithoutRebootAnnotation.Name)

	nodeOffline := mrtesting.NewNode("nodeOffline", false, "machineOffline")
	nodeOffline.Annotations = map[string]string{
		consts.AnnotationNodeMachineReboot: "",
	}

	bareMetalHostOffline := mrtesting.NewBareMetalHost("bareMetalHostOffline", false, false)
	machineOffline := mrtesting.NewMachine("machineOffline", nodeOffline.Name, bareMetalHostOffline.Name)

	bareMetalHostOfflineWithRebootAnnotation := mrtesting.NewBareMetalHost("bareMetalHostOfflineWithRebootAnnotation", false, false)
	bareMetalHostOfflineWithRebootAnnotation.Annotations[consts.AnnotationRebootInProgress] = "true"
	machineOfflineWithRebootAnnotation := mrtesting.NewMachine("machineOfflineWithRebootAnnotation", nodeOffline.Name, bareMetalHostOfflineWithRebootAnnotation.Name)

	nodeNotReady := mrtesting.NewNode("nodeNotReady", false, "machineNotReady")
	nodeNotReady.Annotations = map[string]string{
		consts.AnnotationNodeMachineReboot: "",
	}

	bareMetalHostNotReady := mrtesting.NewBareMetalHost("bareMetalHostNotReady", true, true)
	machineNotReady := mrtesting.NewMachine("machineNotReady", nodeNotReady.Name, bareMetalHostNotReady.Name)

	machineRemediationStartedOnline := mrtesting.NewMachineRemediation("machineRemediationStartedOnline", machineOnline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStateStarted)
	machineRemediationStartedOffline := mrtesting.NewMachineRemediation("machineRemediationStartedOffline", machineOffline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStateStarted)
	machineRemediationPoweroffOnline := mrtesting.NewMachineRemediation("machineRemediationPoweroffOnline", machineOnline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStatePowerOff)
	machineRemediationPoweroffOffline := mrtesting.NewMachineRemediation("machineRemediationPoweroffOffline", machineOffline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStatePowerOff)
	machineRemediationPoweroffTimeout := mrtesting.NewMachineRemediation("machineRemediationPoweroffTimeout", machineOffline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStatePowerOff)
	machineRemediationPoweroffTimeout.Status.StartTime = &metav1.Time{
		Time: machineRemediationPoweroffTimeout.Status.StartTime.Time.Add(-time.Minute * 6),
	}
	machineRemediationPoweron := mrtesting.NewMachineRemediation("machineRemediationPoweron", machineOnline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStatePowerOn)
	machineRemediationPoweronTimeout := mrtesting.NewMachineRemediation("machineRemediationPoweronTimeout", machineOnline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStatePowerOn)
	machineRemediationPoweronTimeout.Status.StartTime = &metav1.Time{
		Time: machineRemediationPoweroffTimeout.Status.StartTime.Time.Add(-time.Minute * 6),
	}
	machineRemediationPoweronNotReady := mrtesting.NewMachineRemediation("machineRemediationPoweronNotReady", machineNotReady.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStatePowerOn)
	machineRemediationSucceeded := mrtesting.NewMachineRemediation("machineRemediationSucceeded", machineOnline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStateSucceeded)
	machineRemediationFailed := mrtesting.NewMachineRemediation("machineRemediationFailed", machineOnline.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStateFailed)
	machineRemediationStartedOfflineWithRebootInProgressAnnotation := mrtesting.NewMachineRemediation("machineRemediationStartedOfflineWithRebootInProgressAnnotation", machineOfflineWithRebootAnnotation.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStateStarted)
	machineRemediationStartedOnlineWithoutRebootInProgressAnnotation := mrtesting.NewMachineRemediation("machineRemediationStartedOnlineWithoutRebootInProgressAnnotation", machineOnlineWithoutRebootAnnotation.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStateStarted)
	machineRemediationPoweroffOnlineWithRebootInProgressAnnotation := mrtesting.NewMachineRemediation("machineRemediationPoweroffOnlineWithRebootInProgressAnnotation", machineOfflineWithRebootAnnotation.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStatePowerOff)

	testCases := []struct {
		name               string
		machineRemediation *mrv1.MachineRemediation
		bareMetalHost      *bmov1.BareMetalHost
		node               *corev1.Node
		expected           expectedRemediationResult
		expectedEvents     []string
	}{
		{
			name:               "with machine remediation started and host has power off state",
			machineRemediation: machineRemediationStartedOffline,
			bareMetalHost:      bareMetalHostOffline,
			node:               nodeOffline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStateSucceeded,
				hasEndTime:                      true,
				bareMetalHostOnline:             false,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: false,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationSkippedOffline"},
		},
		{
			name:               "with machine remediation started and host with rebootInProgress annotation has power off state",
			machineRemediation: machineRemediationStartedOfflineWithRebootInProgressAnnotation,
			bareMetalHost:      bareMetalHostOfflineWithRebootAnnotation,
			node:               nodeOffline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStatePowerOff,
				hasEndTime:                      false,
				bareMetalHostOnline:             false,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: true,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationRebootStarted"},
		},
		{
			name:               "with machine remediation started and host has power on state",
			machineRemediation: machineRemediationStartedOnline,
			bareMetalHost:      bareMetalHostOnline,
			node:               nodeOnline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStatePowerOff,
				hasEndTime:                      false,
				bareMetalHostOnline:             false,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: true,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationRebootStarted"},
		},
		{
			name:               "with machine remediation in power off state and host has power off state",
			machineRemediation: machineRemediationPoweroffOffline,
			bareMetalHost:      bareMetalHostOffline,
			node:               nodeOffline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStatePowerOn,
				hasEndTime:                      false,
				bareMetalHostOnline:             true,
				nodeDeleted:                     true,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: false,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationRebootPoweringOn"},
		},
		{
			name:               "with machine remediation in power off state and host has power on state",
			machineRemediation: machineRemediationPoweroffOnline,
			bareMetalHost:      bareMetalHostOnline,
			node:               nodeOnline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStatePowerOff,
				hasEndTime:                      false,
				bareMetalHostOnline:             true,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: true,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{},
		},
		{
			name:               "with machine remediation in power off state that timeouted",
			machineRemediation: machineRemediationPoweroffTimeout,
			bareMetalHost:      bareMetalHostOffline,
			node:               nodeOffline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStateFailed,
				hasEndTime:                      true,
				bareMetalHostOnline:             false,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: false,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationRebootTimedOut"},
		},
		{
			name:               "with machine remediation in power on state and ready node",
			machineRemediation: machineRemediationPoweron,
			bareMetalHost:      bareMetalHostOnline,
			node:               nodeOnline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStateSucceeded,
				hasEndTime:                      true,
				bareMetalHostOnline:             true,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: true,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationRebootSucceeded"},
		},
		{
			name:               "with machine remediation in power on state that timeouted",
			machineRemediation: machineRemediationPoweronTimeout,
			bareMetalHost:      bareMetalHostOnline,
			node:               nodeOnline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStateFailed,
				hasEndTime:                      true,
				bareMetalHostOnline:             true,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: true,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationRebootTimedOut"},
		},
		{
			name:               "with machine remediation in power on state and non ready node",
			machineRemediation: machineRemediationPoweronNotReady,
			bareMetalHost:      bareMetalHostNotReady,
			node:               nodeNotReady,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStatePowerOn,
				hasEndTime:                      false,
				bareMetalHostOnline:             true,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: false,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{},
		},
		{
			name:               "with machine remediation in succeeded state",
			machineRemediation: machineRemediationSucceeded,
			bareMetalHost:      bareMetalHostOnline,
			node:               nodeOnline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStateSucceeded,
				hasEndTime:                      false,
				bareMetalHostOnline:             true,
				nodeDeleted:                     false,
				machineRemediationDeleted:       true,
				rebootInProgressAnnotationExist: true,
				nodeRebootAnnotationExist:       true,
			},
		},
		{
			name:               "with machine remediation in failed state",
			machineRemediation: machineRemediationFailed,
			bareMetalHost:      bareMetalHostOnline,
			node:               nodeOnline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStateFailed,
				hasEndTime:                      false,
				bareMetalHostOnline:             true,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: true,
				nodeRebootAnnotationExist:       false,
			},
			expectedEvents: []string{},
		},
		{
			name:               "with machine remediation in started state without reboot annotation",
			machineRemediation: machineRemediationStartedOnlineWithoutRebootInProgressAnnotation,
			bareMetalHost:      bareMetalHostOnlineWithoutRebootAnnotation,
			node:               nodeOnline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStatePowerOff,
				hasEndTime:                      false,
				bareMetalHostOnline:             false,
				nodeDeleted:                     false,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: true,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationRebootStarted"},
		},
		{
			name:               "with machine remediation in poweroff state with reboot annotation",
			machineRemediation: machineRemediationPoweroffOnlineWithRebootInProgressAnnotation,
			bareMetalHost:      bareMetalHostOfflineWithRebootAnnotation,
			node:               nodeOffline,
			expected: expectedRemediationResult{
				state:                           mrv1.RemediationStatePowerOn,
				hasEndTime:                      false,
				bareMetalHostOnline:             true,
				nodeDeleted:                     true,
				machineRemediationDeleted:       false,
				rebootInProgressAnnotationExist: false,
				nodeRebootAnnotationExist:       true,
			},
			expectedEvents: []string{"MachineRemediationRebootPoweringOn"},
		},
	}

	for _, tc := range testCases {
		recorder := record.NewFakeRecorder(10)
		bmr := newFakeBareMetalRemediator(
			recorder,
			nodeOnline,
			nodeOffline,
			nodeNotReady,
			machineOnline,
			machineOffline,
			machineNotReady,
			machineOfflineWithRebootAnnotation,
			machineOnlineWithoutRebootAnnotation,
			tc.bareMetalHost,
			tc.machineRemediation,
		)

		err := bmr.Reboot(context.TODO(), tc.machineRemediation)
		if err != nil {
			t.Errorf("%s failed, expected no error, got: %v", tc.name, err)
		}

		mrtesting.AssertEvents(t, tc.name, tc.expectedEvents, recorder.Events)

		newMachineRemediation := &mrv1.MachineRemediation{}
		key := types.NamespacedName{
			Namespace: tc.machineRemediation.Namespace,
			Name:      tc.machineRemediation.Name,
		}
		err = bmr.client.Get(context.TODO(), key, newMachineRemediation)
		if err != nil {
			if errors.IsNotFound(err) && !tc.expected.machineRemediationDeleted {
				t.Errorf("%s failed, expected machine remediation %s to be deleted", tc.name, tc.machineRemediation.Name)
			}

			if !errors.IsNotFound(err) {
				t.Errorf("%s failed, expected no error, got: %v", tc.name, err)
			}
		}

		if err == nil && tc.expected.machineRemediationDeleted {
			t.Errorf("%s failed, expected machine remediation %s to be not deleted", tc.name, tc.machineRemediation.Name)
		}

		if !tc.expected.machineRemediationDeleted && newMachineRemediation.Status.State != tc.expected.state {
			t.Errorf("%s failed, expected MachineRemediation state: %s, got: %s", tc.name, tc.expected.state, newMachineRemediation.Status.State)
		}

		if tc.expected.hasEndTime != (newMachineRemediation.Status.EndTime != nil) {
			endTimeExpectation := ""
			if !tc.expected.hasEndTime {
				endTimeExpectation = "no"
			}
			t.Errorf("%s failed, expected %s endTime, got: %s", tc.name, endTimeExpectation, newMachineRemediation.Status.EndTime)
		}

		newBareMetalHost := &bmov1.BareMetalHost{}
		key = types.NamespacedName{
			Namespace: tc.bareMetalHost.Namespace,
			Name:      tc.bareMetalHost.Name,
		}
		err = bmr.client.Get(context.TODO(), key, newBareMetalHost)
		if err != nil {
			t.Errorf("%s failed, expected no error, got: %v", tc.name, err)
		}

		if tc.expected.bareMetalHostOnline != newBareMetalHost.Spec.Online {
			t.Errorf("%s failed, expected bare metal online parameter: %t, got: %t", tc.name, tc.expected.bareMetalHostOnline, newBareMetalHost.Spec.Online)
		}

		if newBareMetalHost.Annotations == nil {
			if tc.expected.rebootInProgressAnnotationExist {
				t.Errorf("%s failed, expected bare metal to have %q annotation", tc.name, consts.AnnotationRebootInProgress)
			}
		} else {
			_, ok := newBareMetalHost.Annotations[consts.AnnotationRebootInProgress]
			if tc.expected.rebootInProgressAnnotationExist != ok {
				t.Errorf("%s failed, expected bare metal to have %q annotation parameter: %t, got: %t", tc.name, consts.AnnotationRebootInProgress, tc.expected.rebootInProgressAnnotationExist, ok)
			}
		}

		node := &corev1.Node{}
		key = types.NamespacedName{
			Namespace: tc.node.Namespace,
			Name:      tc.node.Name,
		}
		err = bmr.client.Get(context.TODO(), key, node)
		if err != nil {
			if errors.IsNotFound(err) && !tc.expected.nodeDeleted {
				t.Errorf("%s failed, expected node %s to be deleted", tc.name, tc.node.Name)
			}

			if !errors.IsNotFound(err) {
				t.Errorf("%s failed, expected no error, got: %v", tc.name, err)
			}
		}

		if err == nil && tc.expected.nodeDeleted {
			t.Errorf("%s failed, expected node %s to be not deleted", tc.name, node.Name)
		}

		if node != nil && node.Annotations != nil {
			_, ok := node.Annotations[consts.AnnotationNodeMachineReboot]
			if tc.expected.nodeRebootAnnotationExist != ok {
				t.Errorf("%s failed, expected node reboot annotation %t, got %t", tc.name, tc.expected.nodeRebootAnnotationExist, ok)
			}
		}
	}
}
