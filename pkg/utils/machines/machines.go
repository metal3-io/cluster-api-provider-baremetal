package machines

import (
	"context"
	"fmt"

	glog "k8s.io/klog"

	"github.com/metal3-io/cluster-api-provider-baremetal/pkg/consts"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	mapiv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetMachinesByLabelSelector returns machines that suit to the label selector
func GetMachinesByLabelSelector(c client.Client, selector *metav1.LabelSelector, namespace string) (*mapiv1.MachineList, error) {
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	if sel.Empty() {
		return nil, nil
	}

	machines := &mapiv1.MachineList{}
	listOptions := &client.ListOptions{
               Namespace:     namespace,
               LabelSelector: sel,
        }

        if err = c.List(context.TODO(), listOptions, machines); err != nil {
		return nil, err
	}
	return machines, nil
}

// GetNodeByMachine get the node object by machine object
func GetNodeByMachine(c client.Client, machine *mapiv1.Machine) (*v1.Node, error) {
	if machine.Status.NodeRef == nil {
		glog.Errorf("machine %s does not have NodeRef", machine.Name)
		return nil, fmt.Errorf("machine %s does not have NodeRef", machine.Name)
	}
	node := &v1.Node{}
	nodeKey := types.NamespacedName{
		Namespace: machine.Status.NodeRef.Namespace,
		Name:      machine.Status.NodeRef.Name,
	}
	if err := c.Get(context.TODO(), nodeKey, node); err != nil {
		return nil, err
	}
	return node, nil
}

// GetMachineByNode get the machine object by node object
func GetMachineByNode(c client.Client, node *v1.Node) (*mapiv1.Machine, error) {
	machineKey, ok := node.Annotations[consts.AnnotationMachine]
	if !ok {
		return nil, fmt.Errorf("No machine annotation for node %s", node.Name)
	}
	glog.Infof("Node %s is annotated with machine %s", node.Name, machineKey)

	machine := &mapiv1.Machine{}
	namespace, machineName, err := cache.SplitMetaNamespaceKey(machineKey)
	if err != nil {
		return nil, err
	}
	key := &types.NamespacedName{
		Namespace: namespace,
		Name:      machineName,
	}
	if err := c.Get(context.TODO(), *key, machine); err != nil {
		return nil, err
	}
	return machine, nil
}
