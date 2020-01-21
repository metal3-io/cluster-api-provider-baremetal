package nodereboot

import (
	"context"

	glog "k8s.io/klog"

	mrv1 "github.com/metal3-io/cluster-api-provider-baremetal/pkg/apis/machineremediation/v1alpha1"
	"github.com/metal3-io/cluster-api-provider-baremetal/pkg/consts"
	machineutils "github.com/metal3-io/cluster-api-provider-baremetal/pkg/utils/machines"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ reconcile.Reconciler = &ReconcileNodeReboot{}

// ReconcileNodeReboot reconciles a node object
type ReconcileNodeReboot struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	namespace string
}

// Add creates a new NodeReboot Controller and adds it to the Manager.
// The Manager will set fields on the Controller and start it when the Manager is started.
func Add(mgr manager.Manager, opts manager.Options) error {
	r, err := newReconciler(mgr, opts)
	if err != nil {
		return err
	}
	return add(mgr, r)
}

func newReconciler(mgr manager.Manager, opts manager.Options) (reconcile.Reconciler, error) {
	return &ReconcileNodeReboot{
		client:    mgr.GetClient(),
		namespace: opts.Namespace,
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("nodereboot-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{})
}

// Reconcile monitors Nodes and creates the MachineRemediation object when the node has reboot annotation.
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNodeReboot) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	glog.Infof("Reconciling node %s/%s\n", request.Namespace, request.Name)

	// Get node from request
	node := &corev1.Node{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if node.Annotations == nil {
		return reconcile.Result{}, nil
	}

	if _, ok := node.Annotations[consts.AnnotationNodeMachineReboot]; !ok {
		return reconcile.Result{}, nil
	}

	// Verify that we do not have machine remediation in progress
	machine, err := machineutils.GetMachineByNode(r.client, node)
	if err != nil {
		return reconcile.Result{}, err
	}

	rebootInProgress, err := isRebootInProgress(r.client, machine.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	if rebootInProgress {
		return reconcile.Result{}, nil
	}

	// Creates new machine remediation object
	mr := &mrv1.MachineRemediation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "remediation-",
			Namespace:    machine.Namespace,
		},
		Spec: mrv1.MachineRemediationSpec{
			MachineName: machine.Name,
			Type:        mrv1.RemediationTypeReboot,
		},
	}

	if err = r.client.Create(context.TODO(), mr); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func isRebootInProgress(c client.Client, machineName string) (bool, error) {
	machineRemediations := &mrv1.MachineRemediationList{}
	listOptions := &client.ListOptions{}
	if err := c.List(context.TODO(), listOptions, machineRemediations); err != nil {
		return false, err
	}

	rebootInProgress := false

	for _, mr := range machineRemediations.Items {
		if mr.Spec.MachineName == machineName {
			if mr.Status.EndTime == nil {
				rebootInProgress = true
				break
			}
		}
	}
	return rebootInProgress, nil
}
