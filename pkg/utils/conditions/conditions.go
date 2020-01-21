package conditions

import (
	corev1 "k8s.io/api/core/v1"
)

// GetNodeCondition returns node condition by type
func GetNodeCondition(node *corev1.Node, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for _, cond := range node.Status.Conditions {
		if cond.Type == conditionType {
			return &cond
		}
	}
	return nil
}

// NodeHasCondition returns true when the node has condition of the specific type and status
func NodeHasCondition(node *corev1.Node, conditionType corev1.NodeConditionType, contidionStatus corev1.ConditionStatus) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == conditionType && cond.Status == contidionStatus {
			return true
		}
	}
	return false
}
