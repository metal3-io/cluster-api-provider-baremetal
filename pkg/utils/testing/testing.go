package testing

import (
	"fmt"
	"strings"
	"testing"
	"time"

	bmov1 "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	osconfigv1 "github.com/openshift/api/config/v1"
	maov1 "github.com/openshift/machine-api-operator/pkg/apis/healthchecking/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mrv1 "github.com/metal3-io/cluster-api-provider-baremetal/pkg/apis/machineremediation/v1alpha1"
	"github.com/metal3-io/cluster-api-provider-baremetal/pkg/consts"

	mapiv1 "sigs.k8s.io/cluster-api/pkg/apis/machine/v1beta1"
)

var (
	// KnownDate contains date that can be used under tests
	KnownDate = metav1.Time{Time: time.Date(1985, 06, 03, 0, 0, 0, 0, time.Local)}
)

// FooBar returns foo:bar map that can be used as default label
func FooBar() map[string]string {
	return map[string]string{"foo": "bar"}
}

// NewSelector returns new LabelSelector
func NewSelector(labels map[string]string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: labels}
}

// NewSelectorFooBar returns new foo:bar label selector
func NewSelectorFooBar() *metav1.LabelSelector {
	return NewSelector(FooBar())
}

// NewMachineHealthCheck returns new MachineHealthCheck object that can be used for testing
func NewMachineHealthCheck(name string) *maov1.MachineHealthCheck {
	return &maov1.MachineHealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: consts.NamespaceOpenshiftMachineAPI,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "MachineHealthCheck",
		},
		Spec: maov1.MachineHealthCheckSpec{
			Selector: *NewSelectorFooBar(),
		},
		Status: maov1.MachineHealthCheckStatus{},
	}
}

// NewUnhealthyConditionsConfigMap returns new config map object with unhealthy conditions
func NewUnhealthyConditionsConfigMap(name string, data string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: consts.NamespaceOpenshiftMachineAPI,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		Data: map[string]string{
			"conditions": data,
		},
	}
}

// NewBareMetalHost returns new bare metal host object that can be used for testing
func NewBareMetalHost(name string, online bool, powerOn bool) *bmov1.BareMetalHost {
	return &bmov1.BareMetalHost{
		TypeMeta: metav1.TypeMeta{Kind: "BareMetalHost"},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   consts.NamespaceOpenshiftMachineAPI,
		},
		Spec: bmov1.BareMetalHostSpec{
			Online: online,
		},
		Status: bmov1.BareMetalHostStatus{
			PoweredOn: powerOn,
		},
	}
}

// NewMachine returns new machine object that can be used for testing
func NewMachine(name string, nodeName string, bareMetalHostName string) *mapiv1.Machine {
	m := &mapiv1.Machine{
		TypeMeta: metav1.TypeMeta{Kind: "Machine"},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				consts.AnnotationBareMetalHost: fmt.Sprintf("%s/%s", consts.NamespaceOpenshiftMachineAPI, bareMetalHostName),
			},
			Name:            name,
			Namespace:       consts.NamespaceOpenshiftMachineAPI,
			OwnerReferences: []metav1.OwnerReference{{Kind: "MachineSet"}},
			Labels:          FooBar(),
		},
		Spec: mapiv1.MachineSpec{},
	}
	if nodeName != "" {
		m.Status = mapiv1.MachineStatus{
			NodeRef: &corev1.ObjectReference{
				Name:      nodeName,
				Namespace: metav1.NamespaceNone,
			},
		}
	}
	return m
}

// NewMachineRemediation returns new machine remediation object that can be used for testing
func NewMachineRemediation(name string, machineName string, remediationType mrv1.RemediationType, remediationState mrv1.RemediationState) *mrv1.MachineRemediation {
	return &mrv1.MachineRemediation{
		TypeMeta: metav1.TypeMeta{Kind: "MachineRemediation"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: consts.NamespaceOpenshiftMachineAPI,
		},
		Spec: mrv1.MachineRemediationSpec{
			MachineName: machineName,
			Type:        remediationType,
		},
		Status: mrv1.MachineRemediationStatus{
			StartTime: &metav1.Time{Time: time.Now()},
			State:     remediationState,
		},
	}
}

// NewNode returns new node object that can be used for testing
func NewNode(name string, ready bool, machineName string) *corev1.Node {
	nodeReadyStatus := corev1.ConditionTrue
	if !ready {
		nodeReadyStatus = corev1.ConditionUnknown
	}

	return &corev1.Node{
		TypeMeta: metav1.TypeMeta{Kind: "Node"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceNone,
			Annotations: map[string]string{
				consts.AnnotationMachine: fmt.Sprintf("%s/%s", consts.NamespaceOpenshiftMachineAPI, machineName),
			},
			Labels: map[string]string{},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             nodeReadyStatus,
					LastTransitionTime: KnownDate,
				},
			},
		},
	}
}

// NewInfrastructure returns a new Infrastructure object that can be used for testing
func NewInfrastructure(name string, platform osconfigv1.PlatformType) *osconfigv1.Infrastructure {
	return &osconfigv1.Infrastructure{
		TypeMeta: metav1.TypeMeta{Kind: "Infrastructure"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceNone,
		},
		Status: osconfigv1.InfrastructureStatus{
			Platform: platform,
		},
	}
}

// AssertEvents verifies when the test run iniates expected events
func AssertEvents(t *testing.T, testCase string, expectedEvents []string, realEvents chan string) {
	if len(expectedEvents) != len(realEvents) {
		t.Errorf(
			"Test case: %s. Number of expected events (%v) differs from number of real events (%v)",
			testCase,
			len(expectedEvents),
			len(realEvents),
		)
	} else {
		for _, eventType := range expectedEvents {
			select {
			case event := <-realEvents:
				if !strings.Contains(event, eventType) {
					t.Errorf("Test case: %s. Expected %s event, got: %v", testCase, eventType, event)
				}
			default:
				t.Errorf("Test case: %s. Expected %s event, but no event occured", testCase, eventType)
			}
		}
	}
}
