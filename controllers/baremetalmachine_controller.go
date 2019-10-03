/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/kind/pkg/cluster/constants"
)

const (
	machineControllerName = "BareMetalMachine-controller"
)

// BareMetalMachineReconciler reconciles a BareMetalMachine object
type BareMetalMachineReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;machines,verbs=get;list;watch

// Reconcile handles BareMetalMachine events
func (r *BareMetalMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	log := r.Log.WithName(machineControllerName).WithValues("baremetal-machine", req.NamespacedName)

	// Fetch the BareMetalMachine instance.
	baremetalMachine := &infrav1.BareMetalMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, baremetalMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, baremetalMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Waiting for Machine Controller to set OwnerRef on BareMetalMachine")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("BareMetalMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.MachineClusterLabelName))
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for BareMetalCluster Controller to create cluster infrastructure")
		return ctrl.Result{}, nil
	}

	// Fetch the BareMetal Cluster.
	baremetalCluster := &infrav1.BareMetalCluster{}
	baremetalClusterName := types.NamespacedName{
		Namespace: baremetalMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, baremetalClusterName, baremetalCluster); err != nil {
		log.Info("BareMetalCluster is not available yet")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("baremetal-cluster", baremetalCluster.Name)

	// Create a helper for managing the baremetal container hosting the machine.
	externalMachine, err := baremetal.NewMachine(cluster.Name, machine.Name, &baremetalMachine.Spec.Image, log)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the externalMachine")
	}

	// Create a helper for managing a baremetal container hosting the loadbalancer.
	// NB. the machine controller has to manage the cluster load balancer because the current implementation of the
	// baremetal load balancer does not support auto-discovery of control plane nodes, so CAPD should take care of
	// updating the cluster load balancer configuration when control plane machines are added/removed
	externalLoadBalancer, err := baremetal.NewLoadBalancer(cluster.Name, log)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the externalLoadBalancer")
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(baremetalMachine, r)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the BareMetalMachine object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, baremetalMachine); err != nil {
			log.Error(err, "failed to patch BareMetalMachine")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted machines
	if !baremetalMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machine, baremetalMachine, externalMachine, externalLoadBalancer)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(machine, baremetalMachine, externalMachine, externalLoadBalancer, log)
}

func (r *BareMetalMachineReconciler) reconcileNormal(machine *clusterv1.Machine, baremetalMachine *infrav1.BareMetalMachine, externalMachine *baremetal.Machine, externalLoadBalancer *baremetal.LoadBalancer, log logr.Logger) (ctrl.Result, error) {
	// If the BareMetalMachine doesn't have finalizer, add it.
	if !util.Contains(baremetalMachine.Finalizers, infrav1.MachineFinalizer) {
		baremetalMachine.Finalizers = append(baremetalMachine.Finalizers, infrav1.MachineFinalizer)
	}

	// if the machine is already provisioned, return
	if baremetalMachine.Spec.ProviderID != nil {
		return ctrl.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machine.Spec.Bootstrap.Data == nil {
		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return ctrl.Result{}, nil
	}

	//Create the baremetal container hosting the machine
	role := constants.WorkerNodeRoleValue
	if util.IsControlPlaneMachine(machine) {
		role = constants.ControlPlaneNodeRoleValue
	}

	if err := externalMachine.Create(role, machine.Spec.Version); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create worker BareMetalMachine")
	}

	// if the machine is a control plane added, update the load balancer configuration
	if util.IsControlPlaneMachine(machine) {
		if err := externalLoadBalancer.UpdateConfiguration(); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to update BareMetalCluster.loadbalancer configuration")
		}
	}

	// exec bootstrap
	// NB. this step is necessary to mimic the behaviour of cloud-init that is embedded in the base images
	// for other cloud providers
	if err := externalMachine.ExecBootstrap(*machine.Spec.Bootstrap.Data); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to exec BareMetalMachine bootstrap")
	}

	// Set the provider ID on the Kubernetes node corresponding to the external machine
	// NB. this step is necessary because there is no a cloud controller for baremetal that executes this step
	if err := externalMachine.SetNodeProviderID(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch the Kubernetes node with the machine providerID")
	}

	// Set ProviderID so the Cluster API Machine Controller can pull it
	providerID := externalMachine.ProviderID()
	baremetalMachine.Spec.ProviderID = &providerID

	// Mark the baremetalMachine ready
	baremetalMachine.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *BareMetalMachineReconciler) reconcileDelete(machine *clusterv1.Machine, baremetalMachine *infrav1.BareMetalMachine, externalMachine *baremetal.Machine, externalLoadBalancer *baremetal.LoadBalancer) (ctrl.Result, error) {
	// if the deleted machine is a control-plane node, exec kubeadm reset so the etcd member hosted
	// on the machine gets removed in a controlled way
	if util.IsControlPlaneMachine(machine) {
		if err := externalMachine.KubeadmReset(); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to execute kubeadm reset")
		}
	}

	// delete the machine
	if err := externalMachine.Delete(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete BareMetalMachine")
	}

	// if the deleted machine is a control-plane node, remove it from the load balancer configuration;
	if util.IsControlPlaneMachine(machine) {
		if err := externalLoadBalancer.UpdateConfiguration(); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to update BareMetalCluster.loadbalancer configuration")
		}
	}

	// Machine is deleted so remove the finalizer.
	baremetalMachine.Finalizers = util.Filter(baremetalMachine.Finalizers, infrav1.MachineFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *BareMetalMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.BareMetalMachine{}).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("BareMetalMachine")),
			},
		).
		Watches(
			&source.Kind{Type: &infrav1.BareMetalCluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.BareMetalClusterToBareMetalMachines),
			},
		).
		Complete(r)
}

// BareMetalClusterToBareMetalMachines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of BareMetalMachines.
func (r *BareMetalMachineReconciler) BareMetalClusterToBareMetalMachines(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.Object.(*infrav1.BareMetalCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a BareMetalCluster but got a %T", o.Object), "failed to get BareMetalMachine for BareMetalCluster")
		return nil
	}
	log := r.Log.WithValues("BareMetalCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return result
	}

	labels := map[string]string{clusterv1.MachineClusterLabelName: cluster.Name}
	machineList := &clusterv1.MachineList{}
	if err := r.Client.List(context.TODO(), machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list BareMetalMachines")
		return nil
	}
	for _, m := range machineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}
