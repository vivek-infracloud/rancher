package rke2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type RKE2NodeDriverDeletingTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	cnis               []string
	providers          []string
	nodeProviders      []string
}

func (r *RKE2NodeDriverDeletingTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2NodeDriverDeletingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.nodeProviders = clustersConfig.NodeProviders
	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(r.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(r.T(), err)

	r.standardUserClient = standardUserClient
}

func (r *RKE2NodeDriverDeletingTestSuite) CreateAndDeleteRKE2Cluster(client *rancher.Client, provider Provider, nodesAndRoles []machinepools.NodeRoles, kubeVersion, cni string, externalNodeProvider *provisioning.ExternalNodeProvider) {
	cloudCredential, err := provider.CloudCredFunc(client)
	nodeNames := []string{}

	clusterName := namegen.AppendRandomString(provider.Name)
	generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
	machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

	machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
	require.NoError(r.T(), err)

	machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, machinePools)
	//fmt.Print("-------------------cluster------------", cluster)

	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	require.NoError(r.T(), err)

	kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
	require.NoError(r.T(), err)

	result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(r.T(), err)

	checkFunc := clusters.IsProvisioningClusterReady

	err = wait.WatchWait(result, checkFunc)
	assert.NoError(r.T(), err)
	assert.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)
	assert.Equal(r.T(), kubeVersion, cluster.Spec.KubernetesVersion)

	clusterIDName, err := clusters.GetClusterIDByName(r.client, clusterName)
	assert.NoError(r.T(), err)

	fmt.Print("--------Checking if the nodes are ready-----------", clusterIDName)
	err = nodestat.IsNodeReady(client, clusterIDName)
	require.NoError(r.T(), err)

	machineResp, err := r.client.Steve.SteveType("cluster.x-k8s.io.machine").List(nil)
	require.NoError(r.T(), err)
	for _, machine := range machineResp.Data {
		machineobj := &capi.Machine{}
		err = v1.ConvertToK8sType(machine.Spec, &machineobj.Spec)
		require.NoError(r.T(), err)
		if machineobj.Spec.ClusterName == clusterName {
			nodeNames = append(nodeNames, machineobj.Spec.InfrastructureRef.Name)
		}
	}

	fmt.Print("---waiting for 1 minute before deleting the cluster---")
	time.Sleep(1 * time.Minute)

	fmt.Print("--------deleting cluster now-----------", clusterIDName)
	err = r.deleteCluster(clusterResp)
	assert.NoError(r.T(), err)

	fmt.Print("--------Checking if the nodes are deleted from AWS-----------", cluster)
	allNodesDeleted := true
	for _, nodeName := range nodeNames {
		isDeleted, err := externalNodeProvider.IsNodeDeleted(r.client, nodeName)
		assert.NoError(r.T(), err)
		if !isDeleted {
			allNodesDeleted = false
			fmt.Println("Error: Node", nodeName, "is not deleted from AWS")
		}
	}
	if allNodesDeleted {
		fmt.Println("Node is deleted sucessfully from the AWS")
	}
}

func (r *RKE2NodeDriverDeletingTestSuite) deleteCluster(cluster *v1.SteveAPIObject) error {
	err := r.client.Steve.SteveType(ProvisioningSteveResouceType).Delete(cluster)
	if err != nil {
		return err
	}
	adminClient, err := rancher.NewClient(r.client.RancherConfig.AdminToken, r.client.Session)
	if err != nil {
		return err
	}

	provKubeClient, err := adminClient.GetKubeAPIProvisioningClient()
	if err != nil {
		return err
	}
	watchInterface, err := provKubeClient.Clusters(cluster.ObjectMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})

	if err != nil {
		return err
	}

	return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
		cluster := event.Object.(*apisV1.Cluster)
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error deleting cluster")
		} else if event.Type == watch.Deleted {
			return true, nil
		} else if cluster == nil {
			return true, nil
		}
		return false, nil
	})

}
func (r *RKE2NodeDriverDeletingTestSuite) TestCreateAndDeleteRKE2Cluster() {
	nodes, err := r.client.Management.Node.List(&types.ListOpts{})
	require.NoError(r.T(), err)
	logrus.Info("----------------", nodes.Data[0].Labels["kubernetes.io/hostname"])

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRoles

	if len(nodesAndRoles) == 0 {
		r.T().Skip()
	}

	var name string
	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	client, err := r.client.WithSession(subSession)
	require.NoError(r.T(), err)

	for index, providerName := range r.providers {
		provider := CreateProvider(providerName)
		externaNodeProvider := provisioning.ExternalNodeProviderSetup(r.nodeProviders[index])
		for _, kubeVersion := range r.kubernetesVersions {
			for _, cni := range r.cnis {
				name += " cni: " + cni
				r.Run(name, func() {
					r.CreateAndDeleteRKE2Cluster(client, provider, nodesAndRoles, kubeVersion, cni, &externaNodeProvider)
				})
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE2DeletingTestSuite(t *testing.T) {
	suite.Run(t, new(RKE2NodeDriverDeletingTestSuite))
}
