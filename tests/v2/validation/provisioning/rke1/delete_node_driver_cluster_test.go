package rke1

import (
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type RKE1NodeDriverDeletingTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	cnis               []string
	providers          []string
	nodeProviders      []string
}

func (r *RKE1NodeDriverDeletingTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE1NodeDriverDeletingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.RKE1KubernetesVersions
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

func (r *RKE1NodeDriverDeletingTestSuite) CreateAndDeleteRKE1Cluster(t *testing.T, client *rancher.Client, provider Provider, nodesAndRoles []nodepools.NodeRoles, kubeVersion, cni string, nodeTemplate *nodetemplates.NodeTemplate, externalNodeProvider *provisioning.ExternalNodeProvider) (*management.Cluster, error) {
	nodeNames := []string{}
	clusterName := namegen.AppendRandomString(provider.Name)
	cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, client)
	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
	require.NoError(t, err)

	nodePool, err := nodepools.NodePoolSetup(client, nodesAndRoles, clusterResp.ID, nodeTemplate.ID)
	require.NoError(t, err)

	nodePoolName := nodePool.Name

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)
	watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, opts)
	require.NoError(t, err)

	checkFunc := clusters.IsHostedProvisioningClusterReady

	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(t, err)
	assert.Equal(t, clusterName, clusterResp.Name)
	assert.Equal(t, nodePoolName, nodePool.Name)
	assert.Equal(t, kubeVersion, clusterResp.RancherKubernetesEngineConfig.Version)

	clusterIDName, err := clusters.GetClusterIDByName(r.client, clusterName)
	assert.NoError(r.T(), err)

	fmt.Print("--------Checking if the nodes are ready-----------", clusterIDName)
	err = nodestat.IsNodeReady(client, clusterResp.ID)
	require.NoError(t, err)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterName)
	require.NoError(t, err)
	assert.NotEmpty(t, clusterToken)
	podResults, podErrors := pods.StatusPods(client, clusterResp.ID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)

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
	fmt.Println("--------deleting cluster now-----------", clusterIDName)
	err = r.deleteCluster(clusterResp)
	assert.NoError(r.T(), err)

	fmt.Println("--------Checking if the nodes are deleted from AWS-----------")
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

	return clusterResp, nil
}

func (r *RKE1NodeDriverDeletingTestSuite) deleteCluster(cluster *management.Cluster) error {
	adminClient, err := rancher.NewClient(r.client.RancherConfig.AdminToken, r.client.Session)
	if err != nil {
		return err
	}

	clusterResp, err := r.client.Management.Cluster.ByID(cluster.ID)
	if err != nil {
		return err
	}

	watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	r.client, err = r.client.ReLogin()
	if err != nil {
		return err
	}

	err = r.client.Management.Cluster.Delete(clusterResp)
	if err != nil {
		return err
	}

	return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("there was an error deleting cluster")
		} else if event.Type == watch.Deleted {
			return true, nil
		}
		return false, nil
	})
}

func (r *RKE1NodeDriverDeletingTestSuite) TestCreateAndDeleteRKE1Cluster() {
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	nodesAndRoles := clustersConfig.NodesAndRolesRKE1

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
		providerName := " Node Provider: " + provider.Name
		externaNodeProvider := provisioning.ExternalNodeProviderSetup(r.nodeProviders[index])
		for _, kubeVersion := range r.kubernetesVersions {
			name = name + providerName + " Kubernetes version: " + kubeVersion
			for _, cni := range r.cnis {
				nodeTemplate, err := provider.NodeTemplateFunc(r.client)
				require.NoError(r.T(), err)
				name += " cni: " + cni
				r.Run(name, func() {
					r.CreateAndDeleteRKE1Cluster(r.T(), client, provider, nodesAndRoles, kubeVersion, cni, nodeTemplate, &externaNodeProvider)
				})
			}
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestRKE1DeletingTestSuite(t *testing.T) {
	suite.Run(t, new(RKE1NodeDriverDeletingTestSuite))
}
