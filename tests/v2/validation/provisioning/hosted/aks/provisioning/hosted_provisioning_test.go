package provisioning

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials/azure"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/aks"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HostedAKSClusterProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
}

func (h *HostedAKSClusterProvisioningTestSuite) TearDownSuite() {
	h.session.Cleanup()
}

func (h *HostedAKSClusterProvisioningTestSuite) SetupSuite() {
	testSession := session.NewSession()
	h.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(h.T(), err)

	h.client = client

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
	require.NoError(h.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(h.T(), err)

	h.standardUserClient = standardUserClient
}

func (h *HostedAKSClusterProvisioningTestSuite) TestProvisioningHostedAKS() {
	tests := []struct {
		name            string
		client          *rancher.Client
		clusterName     string
		cloudCredential *cloudcredentials.CloudCredential
	}{
		{"Admin User", h.client, "", nil},
		{"Standard User", h.standardUserClient, "", nil},
	}

	for _, tt := range tests {
		h.Run(tt.name, func() {
			subSession := h.session.NewSession()
			defer subSession.Cleanup()

			client, err := tt.client.WithSession(subSession)
			require.NoError(h.T(), err)

			h.testProvisioningHostedAKSCluster(client, tt.clusterName, tt.cloudCredential)
		})
	}
}

func (h *HostedAKSClusterProvisioningTestSuite) TestScalingAKSNodePools() {
	tests := []struct {
		name        string
		client      *rancher.Client
		clusterName string
	}{
		{"Admin User", h.client, ""},
		{"Standard User", h.standardUserClient, ""},
	}

	for _, tt := range tests {
		h.Run(tt.name, func() {
			subSession := h.session.NewSession()
			defer subSession.Cleanup()

			client, err := tt.client.WithSession(subSession)
			require.NoError(h.T(), err)

			h.testScalingAKSNodePools(client, tt.clusterName)
		})
	}
}

func (h *HostedAKSClusterProvisioningTestSuite) testProvisioningHostedAKSCluster(rancherClient *rancher.Client, clusterName string, cloudcredential *cloudcredentials.CloudCredential) (*management.Cluster, error) {
	cloudCredential, err := azure.CreateAzureCloudCredentials(rancherClient)
	require.NoError(h.T(), err)

	clusterName = namegen.AppendRandomString("akshostcluster")
	clusterResp, err := aks.CreateAKSHostedCluster(rancherClient, clusterName, cloudCredential.ID, false, false, false, false, map[string]string{})
	require.NoError(h.T(), err)

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}
	watchInterface, err := h.client.GetManagementWatchInterface(management.ClusterType, opts)
	require.NoError(h.T(), err)

	checkFunc := clusters.IsHostedProvisioningClusterReady

	err = wait.WatchWait(watchInterface, checkFunc)
	require.NoError(h.T(), err)
	assert.Equal(h.T(), clusterName, clusterResp.Name)

	clusterToken, err := clusters.CheckServiceAccountTokenSecret(rancherClient, clusterName)
	require.NoError(h.T(), err)
	assert.NotEmpty(h.T(), clusterToken)

	return clusterResp, nil
}

func (h *HostedAKSClusterProvisioningTestSuite) testScalingAKSNodePools(rancherClient *rancher.Client, clusterName string) {
	cloudCredential, err := azure.CreateAzureCloudCredentials(rancherClient)
	require.NoError(h.T(), err)

	cluster, err := h.testProvisioningHostedAKSCluster(rancherClient, clusterName, cloudCredential)
	require.NoError(h.T(), err)

	updatedCluster, err := ScalingAKSNodePools(rancherClient, cluster, cluster.Name, cloudCredential)
	require.NoError(h.T(), err)
	assert.Equal(h.T(), cluster.Name, updatedCluster.Name)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHostedAKSClusterProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(HostedAKSClusterProvisioningTestSuite))
}
