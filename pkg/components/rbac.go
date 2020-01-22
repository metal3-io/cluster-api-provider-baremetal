package components

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mrv1 "github.com/metal3-io/cluster-api-provider-baremetal/pkg/apis/machineremediation/v1alpha1"
)

var (
	// Rules contains rules for all machine remediation components
	Rules = map[string][]rbacv1.PolicyRule{
		ComponentMachineRemediation: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"machine.openshift.io",
				},
				Resources: []string{
					"machines",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"machineremediation.metal3.io",
				},
				Resources: []string{
					"machineremediations",
					"machineremediations/status",
				},
				Verbs: []string{
					"create",
					"delete",
					"get",
					"list",
					"update",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"nodes",
				},
				Verbs: []string{
					"delete",
					"get",
					"list",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"metal3.io",
				},
				Resources: []string{
					"baremetalhosts",
				},
				Verbs: []string{
					"get",
					"list",
					"update",
					"watch",
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"configmaps",
				},
				Verbs: []string{
					rbacv1.VerbAll,
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"events",
				},
				Verbs: []string{
					"create",
					"list",
					"watch",
					"patch",
				},
			},
		},
	}
)

// NewServiceAccount returns new ServiceAccount object
func NewServiceAccount(name string, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels: map[string]string{
				mrv1.SchemeGroupVersion.Group: "",
			},
		},
	}
}

// NewClusterRole returns new ClusterRole object
func NewClusterRole(name string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				mrv1.SchemeGroupVersion.Group: "",
			},
		},
		Rules: rules,
	}
}

// NewClusterRoleBinding returns new ClusterRoleBinding object
func NewClusterRoleBinding(name string, namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				mrv1.SchemeGroupVersion.Group: "",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: namespace,
				Name:      name,
			},
		},
	}
}
