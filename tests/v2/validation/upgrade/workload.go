package upgrade

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/ingresses"
	kubeingress "github.com/rancher/rancher/tests/framework/extensions/kubeapi/ingresses"
	"github.com/rancher/rancher/tests/framework/extensions/services"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubewait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

// resourceNames struct contains the names of the resources
type resourceNames struct {
	core           map[string]string
	coreWithSuffix map[string]string
	random         map[string]string
}

const (
	ingressHostName = "sslip.io"

	secretAsVolumeName = "secret-as-volume"

	containerName  = "test1"
	containerImage = "ranchertest/mytestcontainer"

	servicePortNumber = 80
	servicePortName   = "port"

	volumeMountPath = "/root/usr/"
)

func getSteveID(namespaceName, resourceName string) string {
	return fmt.Sprintf(namespaceName + "/" + resourceName)
}

// newIngressTemplate is a private constructor that returns ingress spec for specific services
func newIngressTemplate(ingressName, namespaceName, serviceNameForBackend string) networkingv1.Ingress {
	pathTypePrefix := networkingv1.PathTypeImplementationSpecific
	paths := []networkingv1.HTTPIngressPath{
		{
			Path:     "/",
			PathType: &pathTypePrefix,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: serviceNameForBackend,
					Port: networkingv1.ServiceBackendPort{
						Number: servicePortNumber,
					},
				},
			},
		},
	}

	return ingresses.NewIngressTemplate(ingressName, namespaceName, ingressHostName, paths)
}

// newServiceTemplate is a private constructor that returns service spec for specific workloads
func newServiceTemplate(serviceName, namespaceName string, selector map[string]string) corev1.Service {
	serviceType := corev1.ServiceTypeNodePort
	ports := []corev1.ServicePort{
		{
			Name: servicePortName,
			Port: servicePortNumber,
		},
	}

	return services.NewServiceTemplate(serviceName, namespaceName, serviceType, ports, selector)
}

// newTestContainerMinimal is a private constructor that returns container for minimal workload creations
func newTestContainerMinimal() corev1.Container {
	pullPolicy := corev1.PullAlways
	return workloads.NewContainer(containerName, containerImage, pullPolicy, nil, nil)
}

// newPodTemplateWithTestContainer is a private constructor that returns pod template spec for workload creations
func newPodTemplateWithTestContainer() corev1.PodTemplateSpec {
	testContainer := newTestContainerMinimal()
	containers := []corev1.Container{testContainer}
	return workloads.NewPodTemplate(containers, nil, nil, nil)
}

// newPodTemplateWithSecretVolume is a private constructor that returns pod template spec with volume option for workload creations
func newPodTemplateWithSecretVolume(secretName string) corev1.PodTemplateSpec {
	testContainer := newTestContainerMinimal()
	testContainer.VolumeMounts = []corev1.VolumeMount{{Name: secretAsVolumeName, MountPath: volumeMountPath}}
	containers := []corev1.Container{testContainer}
	volumes := []corev1.Volume{
		{
			Name: secretAsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		},
	}

	return workloads.NewPodTemplate(containers, volumes, nil, nil)
}

// newPodTemplateWithSecretEnvironmentVariable is a private constructor that returns pod template spec with envFrom option for workload creations
func newPodTemplateWithSecretEnvironmentVariable(secretName string) corev1.PodTemplateSpec {
	pullPolicy := corev1.PullAlways
	envFrom := []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			},
		},
	}
	container := workloads.NewContainer(containerName, containerImage, pullPolicy, nil, envFrom)
	containers := []corev1.Container{container}

	return workloads.NewPodTemplate(containers, nil, nil, nil)
}

// waitUntilIngressIsAccessible waits until the ingress is accessible
func waitUntilIngressIsAccessible(client *rancher.Client, hostname string) (bool, error) {
	err := kubewait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		isIngressAccessible, err := ingresses.AccessIngressExternally(client, hostname, false)
		if err != nil {
			return false, err
		}

		return isIngressAccessible, nil
	})

	if err != nil && strings.Contains(err.Error(), kubewait.ErrWaitTimeout.Error()) {
		return false, nil
	}

	return true, nil
}

// waitUntilIngressHostnameUpdates is a private function to wait until the ingress hostname updates
func waitUntilIngressHostnameUpdates(client *rancher.Client, clusterID, namespace, ingressName string) error {
	timeout := int64(60 * 5)
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	adminDynamicClient, err := adminClient.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}
	adminIngressResource := adminDynamicClient.Resource(kubeingress.IngressesGroupVersionResource).Namespace(namespace)

	watchAppInterface, err := adminIngressResource.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + ingressName,
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		return err
	}

	return wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		ingressUnstructured := event.Object.(*unstructured.Unstructured)
		ingress := &networkingv1.Ingress{}

		err = scheme.Scheme.Convert(ingressUnstructured, ingress, ingressUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}

		if ingress.Spec.Rules[0].Host != ingressHostName {
			return true, nil
		}
		return false, nil
	})
}

// containsItemWithPrefix returns true if the given slice contains an item with the given prefix
func containsItemWithPrefix(slice []string, expected string) bool {
	for _, s := range slice {
		if checkPrefix(s, expected) {
			return true
		}
	}
	return false
}

// getItemWithPrefix returns the item with the given prefix
func getItemWithPrefix(slice []string, expected string) string {
	for _, s := range slice {
		if checkPrefix(s, expected) {
			return s
		}
	}
	return ""
}

// checkPrefix checks if the given string starts with the given prefix
func checkPrefix(name string, prefix string) bool {
	return strings.HasPrefix(name, prefix)
}

// validateDaemonset checks daemonset available number is equal to the number of workers in the cluster
func validateDaemonset(t *testing.T, client *rancher.Client, clusterID, namespaceName, daemonsetName string) {
	t.Helper()

	workerNodesCollection, err := client.Management.Node.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
			"worker":    true,
		},
	})
	require.NoError(t, err)

	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	daemonSetID := getSteveID(namespaceName, daemonsetName)
	daemonsetResp, err := steveClient.SteveType(workloads.DaemonsetSteveType).ByID(daemonSetID)
	require.NoError(t, err)

	daemonsetStatus := &appv1.DaemonSetStatus{}
	err = v1.ConvertToK8sType(daemonsetResp.Status, daemonsetStatus)
	require.NoError(t, err)

	assert.Equalf(t, int(daemonsetStatus.NumberAvailable), len(workerNodesCollection.Data), "Daemonset %v doesn't have the required ready", daemonsetName)
}

// newNames returns a new resourceNames struct
// it creates a random names with random suffix for each resource by using core and coreWithSuffix names
func newNames() *resourceNames {
	const (
		projectName             = "upgrade-wl-project"
		namespaceName           = "namespace"
		deploymentName          = "deployment"
		daemonsetName           = "daemonset"
		secretName              = "secret"
		serviceName             = "service"
		ingressName             = "ingress"
		defaultRandStringLength = 3
	)

	names := &resourceNames{
		core: map[string]string{
			"projectName":    projectName,
			"namespaceName":  namespaceName,
			"deploymentName": deploymentName,
			"daemonsetName":  daemonsetName,
			"secretName":     secretName,
			"serviceName":    serviceName,
			"ingressName":    ingressName,
		},
		coreWithSuffix: map[string]string{
			"deploymentNameForVolumeSecret":              deploymentName + "-volume-secret",
			"deploymentNameForEnvironmentVariableSecret": deploymentName + "-envar-secret",
			"deploymentNameForIngress":                   deploymentName + "-ingress",
			"daemonsetNameForIngress":                    daemonsetName + "-ingress",
			"daemonsetNameForVolumeSecret":               daemonsetName + "-volume-secret",
			"daemonsetNameForEnvironmentVariableSecret":  daemonsetName + "-envar-secret",
			"serviceNameForDeployment":                   serviceName + "-deployment",
			"serviceNameForDaemonset":                    serviceName + "-daemonset",
			"ingressNameForDeployment":                   ingressName + "-deployment",
			"ingressNameForDaemonset":                    ingressName + "-daemonset",
		},
	}

	names.random = map[string]string{}
	for k, v := range names.coreWithSuffix {
		names.random[k] = v + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	}
	for k, v := range names.core {
		names.random[k] = v + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	}

	return names
}
