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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	clusterControllerName = "BareMetalCluster-controller"
)

// BareMetalClusterReconciler reconciles a BareMetalCluster object
type BareMetalClusterReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a BareMetalCluster object and makes changes based on the state read
// and what is in the BareMetalCluster.Spec
func (r *BareMetalClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	log := log.Log.WithName(clusterControllerName).WithValues("baremetal-cluster", req.NamespacedName)

	// Fetch the BareMetalCluster instance
	baremetalCluster := &infrav1.BareMetalCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, baremetalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, baremetalCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on BareMetalCluster")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Create a helper for managing a baremetal container hosting the loadbalancer.
	externalLoadBalancer, err := baremetal.NewLoadBalancer(cluster.Name, log)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the externalLoadBalancer")
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(baremetalCluster, r)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the BareMetalCluster object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, baremetalCluster); err != nil {
			log.Error(err, "failed to patch BareMetalCluster")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted clusters
	if !baremetalCluster.DeletionTimestamp.IsZero() {
		return reconcileDelete(baremetalCluster, externalLoadBalancer)
	}

	// Handle non-deleted clusters
	return reconcileNormal(baremetalCluster, externalLoadBalancer)
}

func reconcileNormal(baremetalCluster *infrav1.BareMetalCluster, externalLoadBalancer *baremetal.LoadBalancer) (ctrl.Result, error) {
	// If the BareMetalCluster doesn't have finalizer, add it.
	if !util.Contains(baremetalCluster.Finalizers, infrav1.ClusterFinalizer) {
		baremetalCluster.Finalizers = append(baremetalCluster.Finalizers, infrav1.ClusterFinalizer)
	}

	//Create the baremetal container hosting the load balancer
	if err := externalLoadBalancer.Create(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create load balancer")
	}

	// Set APIEndpoints with the load balancer IP so the Cluster API Cluster Controller can pull it
	lbip4, err := externalLoadBalancer.IP()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get ip for the load balancer")
	}

	baremetalCluster.Status.APIEndpoints = []infrav1.APIEndpoint{
		{
			Host: lbip4,
			// Port: loadbalancer.ControlPlanePort,
		},
	}

	// Mark the baremetalCluster ready
	baremetalCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func reconcileDelete(baremetalCluster *infrav1.BareMetalCluster, externalLoadBalancer *baremetal.LoadBalancer) (ctrl.Result, error) {
	// Delete the baremetal container hosting the load balancer
	if err := externalLoadBalancer.Delete(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete load balancer")
	}

	// Cluster is deleted so remove the finalizer.
	baremetalCluster.Finalizers = util.Filter(baremetalCluster.Finalizers, infrav1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *BareMetalClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.BareMetalCluster{}).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("BareMetalCluster")),
			},
		).
		Complete(r)
}
