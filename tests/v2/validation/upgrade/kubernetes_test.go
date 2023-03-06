package upgrade

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/bundledclusters"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UpgradeKubernetesTestSuite struct {
	suite.Suite
	session  *session.Session
	client   *rancher.Client
	Clusters []ClustersToUpgrade
}

func (u *UpgradeKubernetesTestSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeKubernetesTestSuite) SetupSuite() {
	testSession := session.NewSession()
	u.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(u.T(), err)

	u.client = client

	clusters, err := loadUpgradeConfig(client)
	require.NoError(u.T(), err)

	require.NotEmptyf(u.T(), clusters, "couldn't generate the config for the upgrade test")
	u.Clusters = clusters
}

func (u *UpgradeKubernetesTestSuite) TestUpgradeKubernetes() {
	for _, cluster := range u.Clusters {
		cluster := cluster
		u.Run(cluster.Name, func() {
			u.testUpgradeSingleCluster(cluster.Name, cluster.VersionToUpgrade, cluster.isLatestVersion)
		})
	}
}

func TestKubernetesUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeKubernetesTestSuite))
}

func (u *UpgradeKubernetesTestSuite) testUpgradeSingleCluster(clusterName, versionToUpgrade string, isLatestVersion bool) {
	subSession := u.session.NewSession()
	defer subSession.Cleanup()

	client, err := u.client.WithSession(subSession)
	require.NoError(u.T(), err)

	clusterMeta, err := clusters.NewClusterMeta(client, clusterName)
	require.NoError(u.T(), err)
	require.NotNilf(u.T(), clusterMeta, "Couldn't get the cluster meta")
	u.T().Logf("[%v]: Provider is: %v, Hosted: %v, Imported: %v ", clusterName, clusterMeta.Provider, clusterMeta.IsHosted, clusterMeta.IsImported)

	initCluster, err := bundledclusters.NewWithClusterMeta(clusterMeta)
	require.NoError(u.T(), err)

	cluster, err := initCluster.Get(client)
	require.NoError(u.T(), err)

	versions, err := cluster.ListAvailableVersions(client)
	require.NoError(u.T(), err)
	u.T().Logf("[%v]: Available versions for the cluster: %v", clusterName, versions)

	version := getVersion(u.T(), clusterName, versions, isLatestVersion, versionToUpgrade)
	require.NotNilf(u.T(), version, "Couldn't get the version")
	u.T().Logf("[%v]: Selected version: %v", clusterName, *version)

	updatedCluster, err := cluster.UpdateKubernetesVersion(client, version)
	require.NoError(u.T(), err)

	u.T().Logf("[%v]: Validating sent update request for kubernetes version of the cluster", clusterName)
	validateKubernetesVersions(u.T(), client, updatedCluster, version, isCheckingCurrentCluster)

	err = clusters.WaitClusterToBeUpgraded(client, clusterMeta.ID)
	require.NoError(u.T(), err)
	u.T().Logf("[%v]: Waiting cluster to be upgraded and ready", clusterName)

	u.T().Logf("[%v]: Validating updated cluster's kubernetes version", clusterName)
	validateKubernetesVersions(u.T(), client, updatedCluster, version, !isCheckingCurrentCluster)

	if clusterMeta.IsHosted {
		updatedCluster.UpdateNodepoolKubernetesVersions(client, version)

		u.T().Logf("[%v]: Validating sent update request for nodepools kubernetes versions of the cluster", clusterName)
		validateNodepoolVersions(u.T(), client, updatedCluster, version, isCheckingCurrentCluster)

		err = clusters.WaitClusterToBeUpgraded(client, clusterMeta.ID)
		require.NoError(u.T(), err)

		u.T().Logf("[%v]: Validating updated cluster's nodepools kubernetes versions", clusterName)
		validateNodepoolVersions(u.T(), client, updatedCluster, version, !isCheckingCurrentCluster)
	}
}
