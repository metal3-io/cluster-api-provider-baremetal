package nodereboot

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	mrv1 "github.com/metal3-io/cluster-api-provider-baremetal/pkg/apis/machineremediation/v1alpha1"
	"github.com/metal3-io/cluster-api-provider-baremetal/pkg/consts"
	mrtesting "github.com/metal3-io/cluster-api-provider-baremetal/pkg/utils/testing"

	mapiv1 "sigs.k8s.io/cluster-api/pkg/apis/machine/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	// Add types to scheme
	mrv1.AddToScheme(scheme.Scheme)
	mapiv1.AddToScheme(scheme.Scheme)
}

// newFakeReconciler returns a new reconcile.Reconciler with a fake client
func newFakeReconciler(initObjects ...runtime.Object) *ReconcileNodeReboot {
	fakeClient := fake.NewFakeClient(initObjects...)
	return &ReconcileNodeReboot{
		client:    fakeClient,
		namespace: consts.NamespaceOpenshiftMachineAPI,
	}
}

func TestReconcile(t *testing.T) {
	nodeWithRebootAnnotation := mrtesting.NewNode("nodeWithRebootAnnotation", true, "machineWithRebootAnnotation")
	nodeWithRebootAnnotation.Annotations[consts.AnnotationNodeMachineReboot] = ""
	machineWithRebootAnnotation := mrtesting.NewMachine("machineWithRebootAnnotation", nodeWithRebootAnnotation.Name, "")

	nodeWithoutRebootAnnotation := mrtesting.NewNode("nodeWithoutRebootAnnotation", true, "machineWithoutRebootAnnotation")
	machineWithoutRebootAnnotation := mrtesting.NewMachine("machineWithoutRebootAnnotation", nodeWithoutRebootAnnotation.Name, "")

	machineRemediationEnded := mrtesting.NewMachineRemediation("machineRemediationEnded", machineWithRebootAnnotation.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStateFailed)
	machineRemediationEnded.Status.EndTime = &metav1.Time{Time: time.Now()}
	machineRemediationNotEnded := mrtesting.NewMachineRemediation("machineRemediationNotEnded", machineWithRebootAnnotation.Name, mrv1.RemediationTypeReboot, mrv1.RemediationStateStarted)

	testsCases := []struct {
		node                           *corev1.Node
		machineRemdiation              *mrv1.MachineRemediation
		expectedNumMachineRemediations int
	}{
		{
			node:                           nodeWithRebootAnnotation,
			machineRemdiation:              nil,
			expectedNumMachineRemediations: 1,
		},
		{
			node:                           nodeWithoutRebootAnnotation,
			machineRemdiation:              nil,
			expectedNumMachineRemediations: 0,
		},
		{
			node:                           nodeWithRebootAnnotation,
			machineRemdiation:              machineRemediationEnded,
			expectedNumMachineRemediations: 2,
		},
		{
			node:                           nodeWithRebootAnnotation,
			machineRemdiation:              machineRemediationNotEnded,
			expectedNumMachineRemediations: 1,
		},
	}

	for _, tc := range testsCases {
		objects := []runtime.Object{
			machineWithRebootAnnotation,
			machineWithoutRebootAnnotation,
			tc.node,
		}
		if tc.machineRemdiation != nil {
			objects = append(objects, tc.machineRemdiation)
		}
		r := newFakeReconciler(objects...)
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: metav1.NamespaceNone,
				Name:      tc.node.Name,
			},
		}
		result, err := r.Reconcile(request)
		assert.NoError(t, err)
		assert.Equal(t, result, reconcile.Result{})

		mrList := &mrv1.MachineRemediationList{}
		assert.NoError(t, r.client.List(context.TODO(), mrList))

		assert.Equal(t, len(mrList.Items), tc.expectedNumMachineRemediations)
	}
}
