package machines

import (
	"reflect"
	"testing"

	"github.com/metal3-io/cluster-api-provider-baremetal/pkg/consts"
	mrtesting "github.com/metal3-io/cluster-api-provider-baremetal/pkg/utils/testing"

	mapiv1 "sigs.k8s.io/cluster-api/pkg/apis/machine/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	// Add types to scheme
	mapiv1.AddToScheme(scheme.Scheme)
}

type expectedMachines struct {
	machinesNames []string
	error         bool
}

func TestGetMachinesByLabelSelector(t *testing.T) {
	emptyLabelSelector := &metav1.LabelSelector{}
	badLabelSelector := &metav1.LabelSelector{
		MatchLabels:      map[string]string{},
		MatchExpressions: []metav1.LabelSelectorRequirement{{Operator: "fake"}},
	}
	labelSelector := mrtesting.NewSelectorFooBar()

	machine1 := mrtesting.NewMachine("machine1", "node", "bareMetalHost1")
	machine2 := mrtesting.NewMachine("machine2", "node", "bareMetalHost2")
	machineWithWrongLabels := mrtesting.NewMachine("machineWithWrongLabels", "node", "bareMetalHost3")
	machineWithWrongLabels.Labels = map[string]string{"wrong": "wrong"}

	fakeClient := fake.NewFakeClient(machine1, machine2, machineWithWrongLabels)

	testsCases := []struct {
		name          string
		labelSelector *metav1.LabelSelector
		expected      expectedMachines
	}{
		{
			name:          "empty LabelSelector",
			labelSelector: emptyLabelSelector,
			expected: expectedMachines{
				machinesNames: nil,
				error:         false,
			},
		},
		{
			name:          "bad LabelSelector",
			labelSelector: badLabelSelector,
			expected: expectedMachines{
				machinesNames: nil,
				error:         true,
			},
		},
		{
			name:          "correct LabelSelector",
			labelSelector: labelSelector,
			expected: expectedMachines{
				machinesNames: []string{machine1.Name, machine2.Name},
				error:         false,
			},
		},
	}

	for _, tc := range testsCases {
		machines, err := GetMachinesByLabelSelector(fakeClient, tc.labelSelector, consts.NamespaceOpenshiftMachineAPI)
		if tc.expected.error != (err != nil) {
			var errorExpectation string
			if !tc.expected.error {
				errorExpectation = "no"
			}
			t.Errorf("Test case: %s. Expected: %s error, got: %v", tc.name, errorExpectation, err)
		}

		if (tc.expected.machinesNames == nil) != (machines == nil) {
			t.Errorf("Test case: %s. Expected Machines: %v, got: %v", tc.name, tc.expected.machinesNames, machines)
		}

		if machines != nil {
			machineNames := []string{}
			for _, m := range machines.Items {
				machineNames = append(machineNames, m.Name)
			}

			if !reflect.DeepEqual(machineNames, tc.expected.machinesNames) {
				t.Errorf("Test case: %s. Expected Machines: %v, got: %v", tc.name, tc.expected.machinesNames, machineNames)
			}
		}
	}
}

type expectedNode struct {
	node  *corev1.Node
	error bool
}

func TestGetNodeByMachine(t *testing.T) {
	machineName := "machineWithNode"
	node := mrtesting.NewNode("node", true, machineName)
	machineWithNode := mrtesting.NewMachine(machineName, node.Name, "bareMetalHost1")

	machineWithoutNodeRef := mrtesting.NewMachine("machine", "node", "bareMetalHost2")
	machineWithoutNodeRef.Status.NodeRef = nil

	machineWithoutNode := mrtesting.NewMachine("machine", "noNode", "bareMetalHost3")

	fakeClient := fake.NewFakeClient(node)

	testsCases := []struct {
		machine  *mapiv1.Machine
		expected expectedNode
	}{
		{
			machine: machineWithNode,
			expected: expectedNode{
				node:  node,
				error: false,
			},
		},
		{
			machine: machineWithoutNodeRef,
			expected: expectedNode{
				node:  nil,
				error: true,
			},
		},
		{
			machine: machineWithoutNode,
			expected: expectedNode{
				node:  nil,
				error: true,
			},
		},
	}

	for _, tc := range testsCases {
		node, err := GetNodeByMachine(fakeClient, tc.machine)
		if tc.expected.error != (err != nil) {
			var errorExpectation string
			if !tc.expected.error {
				errorExpectation = "no"
			}
			t.Errorf("Test case: %s. Expected: %s error, got: %v", tc.machine.Name, errorExpectation, err)
		}

		if tc.expected.node != nil && node.Name != tc.expected.node.Name {
			t.Errorf("Test case: %s. Expected node: %v, got: %v", tc.machine.Name, tc.expected.node, node)
		}
	}
}
