package remediator

import (
	"context"
	"fmt"
	"time"

	mrv1 "github.com/metal3-io/cluster-api-provider-baremetal/pkg/apis/machineremediation/v1alpha1"
	"github.com/metal3-io/cluster-api-provider-baremetal/pkg/consts"
	"github.com/metal3-io/cluster-api-provider-baremetal/pkg/utils/conditions"

	bmov1 "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	glog "k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	mapiv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	rebootDefaultTimeoutMinutes = 5
)

// BareMetalRemediator implements Remediator interface for bare metal machines
type BareMetalRemediator struct {
	client   client.Client
	recorder record.EventRecorder
}

// NewBareMetalRemediator returns new BareMetalRemediator object
func NewBareMetalRemediator(mgr manager.Manager) *BareMetalRemediator {
	return &BareMetalRemediator{
		client:   mgr.GetClient(),
		recorder: mgr.GetRecorder("baremetal-remediator"),
	}
}

// Recreate recreates the bare metal machine under the cluster
func (bmr *BareMetalRemediator) Recreate(ctx context.Context, machineRemediation *mrv1.MachineRemediation) error {
	return fmt.Errorf("Not implemented yet")
}

// Reboot reboots the bare metal machine
func (bmr *BareMetalRemediator) Reboot(ctx context.Context, machineRemediation *mrv1.MachineRemediation) error {
	glog.V(4).Infof("MachineRemediation %q has state %q", machineRemediation.Name, machineRemediation.Status.State)

	// Get the machine from the MachineRemediation
	key := types.NamespacedName{
		Namespace: machineRemediation.Namespace,
		Name:      machineRemediation.Spec.MachineName,
	}
	machine := &mapiv1.Machine{}
	if err := bmr.client.Get(context.TODO(), key, machine); err != nil {
		return err
	}

	// Get the bare metal host object
	bmh, err := getBareMetalHostByMachine(bmr.client, machine)
	if err != nil {
		return err
	}

	// Copy the BareMetalHost object to prevent modification of the original one
	bmhCopy := bmh.DeepCopy()

	// Copy the MachineRemediation object to prevent modification of the original one
	mrCopy := machineRemediation.DeepCopy()

	now := time.Now()
	switch machineRemediation.Status.State {
	// initiating the reboot action
	case mrv1.RemediationStateStarted:
		rebootInProgress := isRebootInProgress(bmh)
		// skip the reboot in case when the machine has power off state before the reboot action
		// it can mean that an user power off the machine by purpose
		if !bmh.Spec.Online && !rebootInProgress {
			glog.V(4).Infof("Skip the remediation, machine %q has power off state before the remediation action", machine.Name)
			bmr.recorder.Eventf(
				machine,
				corev1.EventTypeNormal,
				"MachineRemediationSkippedOffline",
				"Remediation of machine %q skipped because it was in power off state already",
				machine.Name,
			)
			mrCopy.Status.State = mrv1.RemediationStateSucceeded
			mrCopy.Status.Reason = "Skip the reboot, the machine was powered off already"
			mrCopy.Status.EndTime = &metav1.Time{Time: now}
		} else {
			if !rebootInProgress {
				// set rebootInProgress annotation on the bare metal host
				if bmhCopy.Annotations == nil {
					bmhCopy.Annotations = map[string]string{}
				}
				bmhCopy.Annotations[consts.AnnotationRebootInProgress] = "true"
			}

			// power off the machine
			glog.V(4).Infof("Power off machine %q", machine.Name)
			bmhCopy.Spec.Online = false

			if err := bmr.client.Update(context.TODO(), bmhCopy); err != nil {
				return err
			}

			bmr.recorder.Eventf(
				machine,
				corev1.EventTypeNormal,
				"MachineRemediationRebootStarted",
				"Reboot of machine %q has started",
				machine.Name,
			)

			mrCopy.Status.State = mrv1.RemediationStatePowerOff
			mrCopy.Status.Reason = "Starts the reboot process"
		}
		return bmr.client.Status().Update(context.TODO(), mrCopy)

	case mrv1.RemediationStatePowerOff:
		// failed the remediation on timeout
		if machineRemediation.Status.StartTime.Time.Add(rebootDefaultTimeoutMinutes * time.Minute).Before(now) {
			glog.Errorf("Remediation of machine %q failed on timeout", machine.Name)
			bmr.recorder.Eventf(
				machine,
				corev1.EventTypeWarning,
				"MachineRemediationRebootTimedOut",
				"Remediation of machine %q timed out",
				machine.Name,
			)
			mrCopy.Status.State = mrv1.RemediationStateFailed
			mrCopy.Status.Reason = "Reboot failed on timeout"
			mrCopy.Status.EndTime = &metav1.Time{Time: now}
			return bmr.client.Status().Update(context.TODO(), mrCopy)
		}

		// host still has state on, we need to reconcile
		if bmh.Status.PoweredOn {
			glog.Warningf("machine %q still has power on state", machine.Name)
			return nil
		}

		// delete the node to release workloads, once we are sure that host has state power off
		if err := deleteMachineNode(bmr.client, machine); err != nil {
			return err
		}

		// power on the machine
		glog.V(4).Infof("Power on machine %q", machine.Name)
		bmhCopy.Spec.Online = true

		// remove the reboot in progress annotation
		if bmhCopy.Annotations != nil {
			_, ok := bmhCopy.Annotations[consts.AnnotationRebootInProgress]
			if ok {
				delete(bmhCopy.Annotations, consts.AnnotationRebootInProgress)
			}
		}

		if err := bmr.client.Update(context.TODO(), bmhCopy); err != nil {
			return err
		}
		bmr.recorder.Eventf(
			machine,
			corev1.EventTypeNormal,
			"MachineRemediationRebootPoweringOn",
			"Powering on machine %q",
			machine.Name,
		)

		mrCopy.Status.State = mrv1.RemediationStatePowerOn
		mrCopy.Status.Reason = "Reboot in progress"
		return bmr.client.Status().Update(context.TODO(), mrCopy)

	case mrv1.RemediationStatePowerOn:
		// failed the remediation on timeout
		if machineRemediation.Status.StartTime.Time.Add(rebootDefaultTimeoutMinutes * time.Minute).Before(now) {
			glog.Errorf("Remediation of machine %q failed on timeout", machine.Name)
			bmr.recorder.Eventf(
				machine,
				corev1.EventTypeWarning,
				"MachineRemediationRebootTimedOut",
				"Remediation of machine %q timed out",
				machine.Name,
			)
			mrCopy.Status.State = mrv1.RemediationStateFailed
			mrCopy.Status.Reason = "Reboot failed on timeout"
			mrCopy.Status.EndTime = &metav1.Time{Time: now}
			return bmr.client.Status().Update(context.TODO(), mrCopy)
		}

		node, err := getNodeByMachine(bmr.client, machine)
		if err != nil {
			// we want to reconcile with delay of 10 seconds when the machine does not have node reference
			// or node does not exist
			if errors.IsNotFound(err) {
				glog.Warningf("The machine %q node does not exist", machine.Name)
				return nil
			}
			return err
		}

		// Node back to Ready under the cluster
		if conditions.NodeHasCondition(node, corev1.NodeReady, corev1.ConditionTrue) {
			glog.V(4).Infof("Remediation of machine %q succeeded", machine.Name)
			bmr.recorder.Eventf(
				machine,
				corev1.EventTypeNormal,
				"MachineRemediationRebootSucceeded",
				"Remediation of machine %q succeeded",
				machine.Name,
			)
			mrCopy.Status.State = mrv1.RemediationStateSucceeded
			mrCopy.Status.Reason = "Reboot succeeded"
			mrCopy.Status.EndTime = &metav1.Time{Time: now}
			return bmr.client.Status().Update(context.TODO(), mrCopy)
		}
		return nil

	// assumption that the reboot annotation removed because of node removal
	case mrv1.RemediationStateSucceeded:
		// remove machine remediation object
		return bmr.client.Delete(context.TODO(), machineRemediation)

	case mrv1.RemediationStateFailed:
		node, err := getNodeByMachine(bmr.client, machine)
		if errors.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return err
		}

		// remove the reboot annotation from the node, to initiate the reboot again
		return removeNodeRebootAnnotation(bmr.client, node)
	}
	return nil
}

// getBareMetalHostByMachine returns the bare metal host that linked to the machine
func getBareMetalHostByMachine(c client.Client, machine *mapiv1.Machine) (*bmov1.BareMetalHost, error) {
	bmhKey, ok := machine.Annotations[consts.AnnotationBareMetalHost]
	if !ok {
		return nil, fmt.Errorf("machine does not have bare metal host annotation")
	}

	bmhNamespace, bmhName, err := cache.SplitMetaNamespaceKey(bmhKey)
	bmh := &bmov1.BareMetalHost{}
	key := client.ObjectKey{
		Name:      bmhName,
		Namespace: bmhNamespace,
	}

	err = c.Get(context.TODO(), key, bmh)
	if err != nil {
		return nil, err
	}
	return bmh, nil
}

// getNodeByMachine returns the node object referenced by machine
func getNodeByMachine(c client.Client, machine *mapiv1.Machine) (*corev1.Node, error) {
	if machine.Status.NodeRef == nil {
		return nil, errors.NewNotFound(corev1.Resource("ObjectReference"), machine.Name)
	}

	node := &corev1.Node{}
	key := client.ObjectKey{
		Name:      machine.Status.NodeRef.Name,
		Namespace: machine.Status.NodeRef.Namespace,
	}

	if err := c.Get(context.TODO(), key, node); err != nil {
		return nil, err
	}
	return node, nil
}

// deleteMachineNode deletes the node that mapped to specified machine
func deleteMachineNode(c client.Client, machine *mapiv1.Machine) error {
	node, err := getNodeByMachine(c, machine)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Warningf("The machine %q node does not exist", machine.Name)
			return nil
		}
		return err
	}

	return c.Delete(context.TODO(), node)
}

// isRebootInProgress returns true when the BareMetalHost currently is rebooting
func isRebootInProgress(bmh *bmov1.BareMetalHost) bool {
	rebootInProgress, ok := bmh.Annotations[consts.AnnotationRebootInProgress]
	if !ok || rebootInProgress != "true" {
		return false
	}
	return true
}

// removeNodeRebootAnnotation removes the reboot annotation from the node
func removeNodeRebootAnnotation(c client.Client, node *corev1.Node) error {
	nodeCopy := node.DeepCopy()

	if nodeCopy.Annotations == nil {
		return nil
	}

	if _, ok := nodeCopy.Annotations[consts.AnnotationNodeMachineReboot]; !ok {
		return nil
	}

	delete(nodeCopy.Annotations, consts.AnnotationNodeMachineReboot)
	return c.Update(context.TODO(), nodeCopy)
}
