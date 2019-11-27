package machineset

import (
	"context"
	"fmt"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	actuator "github.com/metal3-io/cluster-api-provider-baremetal/pkg/cloud/baremetal/actuators/machine"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	machinev1alpha1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AutoScaleAnnotation is an annotation key that, when added to a MachineSet
// with any value, indicates that this controller should scale that MachineSet
// to equal the number of matching BareMetalHosts in the same namespace.
const AutoScaleAnnotation = "metal3.io/autoscale-to-hosts"

type msmapper struct {
	client client.Client
}

// Map will return reconcile requests for a MachineSet if the event is for a
// BareMetalHost and that BareMetalHost matches the MachineSet's HostSelector.
func (m *msmapper) Map(obj handler.MapObject) []reconcile.Request {
	requests := []reconcile.Request{}
	if host, ok := obj.Object.(*bmh.BareMetalHost); ok {
		msets := machinev1alpha1.MachineSetList{}
		err := m.client.List(context.TODO(), &client.ListOptions{Namespace: host.Namespace}, &msets)
		if err != nil {
			log.Error(err, "failed to list MachineSets")
			return []reconcile.Request{}
		}
		for _, ms := range msets.Items {
			annotations := ms.ObjectMeta.GetAnnotations()
			if annotations == nil {
				continue
			}
			_, present := annotations[AutoScaleAnnotation]
			if !present {
				continue
			}

			matches, err := m.hostMatchesMachineSet(host, &ms)
			if err != nil {
				nn := fmt.Sprintf("%s/%s", ms.Namespace, ms.Name)
				log.Error(err, "failed to determine if host matches MachineSet", "MachineSet", nn)
				continue
			}
			if matches {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ms.Name,
						Namespace: ms.Namespace,
					},
				})
			}
		}
	}
	return requests
}

func (m *msmapper) hostMatchesMachineSet(host *bmh.BareMetalHost, ms *machinev1alpha1.MachineSet) (bool, error) {
	selector, err := actuator.SelectorFromProviderSpec(&ms.Spec.Template.Spec.ProviderSpec)
	if err != nil {
		return false, err
	}
	return selector.Matches(labels.Set(host.ObjectMeta.Labels)), nil
}
