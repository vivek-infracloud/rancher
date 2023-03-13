package rbac

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RoleGroupVersionResource is the required Group Version Resource for accessing roles in a cluster,
// using the dynamic client.
var RoleGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "roles",
}

// RoleBindingGroupVersionResource is the required Group Version Resource for accessing rolebindings in a cluster,
// using the dynamic client.
var RoleBindingGroupVersionResource = schema.GroupVersionResource{
	Group:    rbacv1.SchemeGroupVersion.Group,
	Version:  rbacv1.SchemeGroupVersion.Version,
	Resource: "rolebindings",
}

// CreateRole is a helper function that uses the dynamic client to create a role on a namespace for a specific cluster.
func CreateRole(client *rancher.Client, clusterName string, role *rbacv1.Role) (*rbacv1.Role, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	roleResource := dynamicClient.Resource(RoleGroupVersionResource).Namespace(role.Namespace)

	unstructuredResp, err := roleResource.Create(context.Background(), unstructured.MustToUnstructured(role), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newRole := &rbacv1.Role{}
	err = scheme.Scheme.Convert(unstructuredResp, newRole, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newRole, nil
}

// CreateRoleBinding is a helper function that uses the dynamic client to create a rolebinding on a namespace for a specific cluster.
func CreateRoleBinding(client *rancher.Client, clusterName, roleBindingName, namespace, roleName string, subject rbacv1.Subject) (*rbacv1.RoleBinding, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "Role",
			Name:     roleName,
		},
	}

	roleBindingResource := dynamicClient.Resource(RoleBindingGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := roleBindingResource.Create(context.Background(), unstructured.MustToUnstructured(roleBinding), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newRoleBinding := &rbacv1.RoleBinding{}
	err = scheme.Scheme.Convert(unstructuredResp, newRoleBinding, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newRoleBinding, nil
}
