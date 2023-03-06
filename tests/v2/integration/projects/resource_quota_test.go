package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/resourcequotas"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	steveResourceQuotas "github.com/rancher/rancher/tests/framework/extensions/resourcequotas"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	resourceQuotaNamespaceName  = "ns1"
	resourceQuotaNamespaceName2 = "ns2"
	localClusterID              = "local"
)

type ResourceQuotaSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *ResourceQuotaSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *ResourceQuotaSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *ResourceQuotaSuite) TestCreateNamespaceWithQuotaInProject() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	projectLimit := &management.ResourceQuotaLimit{
		LimitsCPU: "500m",
	}
	namespaceDefaultLimit := &management.ResourceQuotaLimit{
		LimitsCPU: "200m",
	}

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: projectLimit,
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: namespaceDefaultLimit,
		},
	}
	testProject, err := client.Management.Project.Create(projectConfig)
	s.Require().NoError(err)

	namespace, err := namespaces.CreateNamespace(client, resourceQuotaNamespaceName, "", map[string]string{}, map[string]string{}, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	quotas, err := resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in a new namespace, but got %d", len(quotas.Items))

	resourceList := quotas.Items[0].Spec.Hard
	want := v1.ResourceList{
		v1.ResourceLimitsCPU: resource.MustParse("200m"),
	}
	s.Require().Equal(want, resourceList)
	s.Require().NoError(err)
}

func (s *ResourceQuotaSuite) TestCreateNamespaceWithOverriddenQuotaInProject() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{
				LimitsCPU: "500m",
			},
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{
				LimitsCPU: "200m",
			},
		},
	}
	testProject, err := client.Management.Project.Create(projectConfig)
	s.Require().NoError(err)

	annotations1 := map[string]string{
		"field.cattle.io/resourceQuota": "{\"limit\":{\"limitsCpu\":\"190m\"}}",
	}
	namespace, err := namespaces.CreateNamespace(client, resourceQuotaNamespaceName, "", map[string]string{}, annotations1, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	annotations2 := map[string]string{
		"field.cattle.io/resourceQuota": "{\"limit\":{\"limitsCpu\":\"400m\", \"configMaps\":\"50\"}}",
	}
	namespace, err = namespaces.CreateNamespace(client, resourceQuotaNamespaceName2, "", map[string]string{}, annotations2, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	quotas, err := resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in %s, but got %d", resourceQuotaNamespaceName, len(quotas.Items))

	resourceList := quotas.Items[0].Spec.Hard
	want := v1.ResourceList{
		v1.ResourceLimitsCPU: resource.MustParse("190m"),
	}
	s.Require().Equal(want, resourceList)
	s.Require().NoError(err)

	quotas, err = resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName2, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in %s, but got %d", resourceQuotaNamespaceName2, len(quotas.Items))

	resourceList = quotas.Items[0].Spec.Hard
	want = v1.ResourceList{
		v1.ResourceLimitsCPU: resource.MustParse("0"),
	}
	s.Require().Equal(want, resourceList)
	s.Require().NoError(err)
}

func (s *ResourceQuotaSuite) TestRemoveQuotaFromProjectWithNamespacePropagation() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	dynamicClient, err := client.GetRancherDynamicClient()
	s.Require().NoError(err)

	projectLimit := &management.ResourceQuotaLimit{
		LimitsCPU:  "500m",
		ConfigMaps: "10",
	}
	namespaceDefaultLimit := &management.ResourceQuotaLimit{
		LimitsCPU:  "200m",
		ConfigMaps: "5",
	}

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: projectLimit,
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: namespaceDefaultLimit,
		},
	}
	testProject, err := client.Management.Project.Create(projectConfig)
	s.Require().NoError(err)

	namespace, err := namespaces.CreateNamespace(client, resourceQuotaNamespaceName, "", map[string]string{}, map[string]string{}, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	testProject.ResourceQuota.Limit.LimitsCPU = ""
	testProject.NamespaceDefaultResourceQuota.Limit.LimitsCPU = ""

	_, err = client.Management.Project.Replace(testProject)
	s.Require().NoError(err)

	quotas, err := resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)

	// Allow the controller to update the resource quotas after the project has been updated.
	quotaName := quotas.Items[0].Name
	quotaID := fmt.Sprintf("%s/%s", resourceQuotaNamespaceName, quotaName)
	err = steveResourceQuotas.CheckResourceActiveState(client, quotaID)
	s.Require().NoError(err)

	quotas, err = resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in the namespace, but got %d", len(quotas.Items))

	resourceList := quotas.Items[0].Spec.Hard
	want := v1.ResourceList{
		v1.ResourceConfigMaps: resource.MustParse("5"),
	}
	s.Require().Equal(want, resourceList, "Expected the CPU limits to be removed, but config maps limit to remain")
	s.Require().NoError(err)

	// Now remove the last resource limit from the project.
	testProject.ResourceQuota.Limit.ConfigMaps = ""
	testProject.NamespaceDefaultResourceQuota.Limit.ConfigMaps = ""

	watchInterface, err := dynamicClient.Resource(resourcequotas.ResourceQuotaGroupVersionResource).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + quotaName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	s.Require().NoError(err)

	_, err = client.Management.Project.Replace(testProject)
	s.Require().NoError(err)

	err = wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error deleting cluster")
		} else if event.Type == watch.Deleted {
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)

	quotas, err = resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 0, "Expected no quotas in the namespace, but got %d", len(quotas.Items))
}

func (s *ResourceQuotaSuite) TestAddQuotaFromProjectWithNamespacePropagation() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	projectLimit := &management.ResourceQuotaLimit{
		LimitsCPU: "500m",
	}
	namespaceDefaultLimit := &management.ResourceQuotaLimit{
		LimitsCPU: "200m",
	}

	projectConfig := &management.Project{
		ClusterID: "local",
		Name:      "TestProject",
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: projectLimit,
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: namespaceDefaultLimit,
		},
	}
	testProject, err := client.Management.Project.Create(projectConfig)
	s.Require().NoError(err)

	namespace, err := namespaces.CreateNamespace(client, resourceQuotaNamespaceName, "", map[string]string{}, map[string]string{}, testProject)
	s.Require().NoError(err)
	s.Require().NotNil(namespace)

	testProject.ResourceQuota.Limit.Secrets = "20"
	testProject.NamespaceDefaultResourceQuota.Limit.Secrets = "10"

	_, err = client.Management.Project.Replace(testProject)
	s.Require().NoError(err)

	quotas, err := resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)

	// Allow the controller to update the resource quotas after the project has been updated.
	quotaName := quotas.Items[0].Name
	quotaID := fmt.Sprintf("%s/%s", resourceQuotaNamespaceName, quotaName)
	err = steveResourceQuotas.CheckResourceActiveState(client, quotaID)
	s.Require().NoError(err)

	quotas, err = resourcequotas.ListResourceQuotas(client, localClusterID, resourceQuotaNamespaceName, metav1.ListOptions{})
	s.Require().NoError(err)
	s.Require().NotNil(quotas)
	s.Require().Lenf(quotas.Items, 1, "Expected 1 quota in the namespace, but got %d", len(quotas.Items))

	resourceList := quotas.Items[0].Spec.Hard
	want := v1.ResourceList{
		v1.ResourceLimitsCPU: resource.MustParse("200m"),
		v1.ResourceSecrets:   resource.MustParse("10"),
	}
	s.Require().Equal(want, resourceList)
	s.Require().NoError(err)
}

func TestResourceQuotaTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceQuotaSuite))
}
