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

package machineset

import (
	"context"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	actuator "github.com/metal3-io/cluster-api-provider-baremetal/pkg/cloud/baremetal/actuators/machine"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	machinev1alpha1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("machineset-controller")

// Add creates a new MachineSet Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMachineSet{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("machineset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MachineSet
	err = c.Watch(&source.Kind{Type: &machinev1alpha1.MachineSet{}},
		&handler.EnqueueRequestForObject{}, predicate.ResourceVersionChangedPredicate{})
	if err != nil {
		return err
	}

	mapper := msmapper{client: mgr.GetClient()}
	err = c.Watch(&source.Kind{Type: &bmh.BareMetalHost{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: &mapper}, predicate.ResourceVersionChangedPredicate{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMachineSet{}

// ReconcileMachineSet reconciles a MachineSet object
type ReconcileMachineSet struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a MachineSet object and makes changes based on the state read
// and what is in the MachineSet.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=cluster.k8s.io,resources=machinesets,verbs=get;list;watch;update;patch
func (r *ReconcileMachineSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := log.WithValues("MachineSet", request.NamespacedName.String())
	ctx := context.TODO()
	// Fetch the MachineSet instance
	instance := &machinev1alpha1.MachineSet{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Don't take action if the annotation is not present
	annotations := instance.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return reconcile.Result{}, nil
	}
	_, present := annotations[AutoScaleAnnotation]
	if !present {
		return reconcile.Result{}, nil
	}

	selector, err := actuator.SelectorFromProviderSpec(&instance.Spec.Template.Spec.ProviderSpec)
	if err != nil {
		return reconcile.Result{}, err
	}

	// The HostSelector limits which BareMetalHosts can be matched with a
	// Machine. If you have a MachineSet in your cluster that matches *anything*
	// because it lacks a HostSelector, then if you try to add a second
	// MachineSet later that does have a HostSelector, the first MachineSet will
	// compete with it for the same BareMetalHosts.
	if selector.Empty() {
		log.Info("MachineSet lacks a HostSelector; adding a future MachineSet may be difficult.")
	}

	hosts := &bmh.BareMetalHostList{}
	opts := &client.ListOptions{
		Namespace: instance.Namespace,
	}

	err = r.List(ctx, opts, hosts)
	if err != nil {
		return reconcile.Result{}, err
	}

	var count int32
	for _, host := range hosts.Items {
		if selector.Matches(labels.Set(host.ObjectMeta.Labels)) {
			count++
		}
	}

	if instance.Spec.Replicas == nil || count != *instance.Spec.Replicas {
		log.Info("Scaling MachineSet", "new_replicas", count, "old_replicas", instance.Spec.Replicas)
		new := instance.DeepCopy()
		new.Spec.Replicas = &count
		err = r.Update(ctx, new)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
