package charts

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/charts"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IstioTestSuite struct {
	suite.Suite
	client              *rancher.Client
	session             *session.Session
	project             *management.Project
	chartInstallOptions *chartInstallOptions
	chartFeatureOptions *chartFeatureOptions
}

func (i *IstioTestSuite) TearDownSuite() {
	i.session.Cleanup()
}

func (i *IstioTestSuite) SetupSuite() {
	testSession := session.NewSession()
	i.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(i.T(), err)

	i.client = client

	// Get clusterName from config yaml
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(i.T(), clusterName, "Cluster name to install is not set")

	// Get clusterID with clusterName
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(i.T(), err)

	// Change kiali and jaeger paths if it's not local cluster
	if clusterID != clusterName {
		kialiPath = fmt.Sprintf("k8s/clusters/%s/%s", clusterID, kialiPath)
		tracingPath = fmt.Sprintf("k8s/clusters/%s/%s", clusterID, tracingPath)
	}

	// Get latest versions of monitoring & istio charts
	latestIstioVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherIstioName)
	require.NoError(i.T(), err)
	latestMonitoringVersion, err := client.Catalog.GetLatestChartVersion(charts.RancherMonitoringName)
	require.NoError(i.T(), err)

	// Create project
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      exampleAppProjectName,
	}
	createdProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(i.T(), err)
	require.Equal(i.T(), createdProject.Name, exampleAppProjectName)
	i.project = createdProject

	i.chartInstallOptions = &chartInstallOptions{
		monitoring: &charts.InstallOptions{
			ClusterName: clusterName,
			ClusterID:   clusterID,
			Version:     latestMonitoringVersion,
			ProjectID:   createdProject.ID,
		},
		istio: &charts.InstallOptions{
			ClusterName: clusterName,
			ClusterID:   clusterID,
			Version:     latestIstioVersion,
			ProjectID:   createdProject.ID,
		},
	}

	i.chartFeatureOptions = &chartFeatureOptions{
		monitoring: &charts.RancherMonitoringOpts{
			IngressNginx:         true,
			RKEControllerManager: true,
			RKEEtcd:              true,
			RKEProxy:             true,
			RKEScheduler:         true,
		},
		istio: &charts.RancherIstioOpts{
			IngressGateways: true,
			EgressGateways:  false,
			Pilot:           true,
			Telemetry:       true,
			Kiali:           true,
			Tracing:         true,
			CNI:             false,
		},
	}
}

func (i *IstioTestSuite) TestIstioChart() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	client, err := i.client.WithSession(subSession)
	require.NoError(i.T(), err)

	i.T().Log("Checking if the monitoring chart is installed")
	monitoringChart, err := charts.GetChartStatus(client, i.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(i.T(), err)

	if !monitoringChart.IsAlreadyInstalled {
		i.T().Log("Installing monitoring chart")
		err = charts.InstallRancherMonitoringChart(client, i.chartInstallOptions.monitoring, i.chartFeatureOptions.monitoring)
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart StatefulSets to have expected number of ready replicas")
		err = charts.WatchAndWaitStatefulSets(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)
	}

	i.T().Log("Checking if the istio chart is installed")
	istioChart, err := charts.GetChartStatus(client, i.project.ClusterID, charts.RancherIstioNamespace, charts.RancherIstioName)
	require.NoError(i.T(), err)

	if !istioChart.IsAlreadyInstalled {
		i.T().Log("Installing istio chart with the latest version")
		err = charts.InstallRancherIstioChart(client, i.chartInstallOptions.istio, i.chartFeatureOptions.istio)
		require.NoError(i.T(), err)

		i.T().Log("Waiting istio chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, i.project.ClusterID, charts.RancherIstioNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)

		i.T().Log("Waiting istio chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, i.project.ClusterID, charts.RancherIstioNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)
	}

	i.T().Log("Creating namespace with istio injection enabled option for the example app")
	createdNamespace, err := namespaces.CreateNamespace(client, exampleAppNamespaceName, "{}", map[string]string{"istio-injection": "enabled"}, map[string]string{}, i.project)
	require.NoError(i.T(), err)
	require.Equal(i.T(), exampleAppNamespaceName, createdNamespace.Name)

	i.T().Log("Importing example app objects to the namespace")
	readYamlFile, err := os.ReadFile("./resources/istio-demobookapp.yaml")
	require.NoError(i.T(), err)
	yamlInput := &management.ImportClusterYamlInput{
		DefaultNamespace: exampleAppNamespaceName,
		YAML:             string(readYamlFile),
	}
	cluster, err := client.Management.Cluster.ByID(i.project.ClusterID)
	require.NoError(i.T(), err)
	_, err = client.Management.Cluster.ActionImportYaml(cluster, yamlInput)
	require.NoError(i.T(), err)

	i.T().Log("Waiting example app deployments to have expected number of available replicas")
	err = charts.WatchAndWaitDeployments(client, i.project.ClusterID, exampleAppNamespaceName, metav1.ListOptions{})
	require.NoError(i.T(), err)

	i.T().Log("Validating kiali and jaeger endpoints are accessible")
	kialiResult, err := charts.GetChartCaseEndpoint(client, client.RancherConfig.Host, kialiPath, true)
	require.NoError(i.T(), err)
	assert.True(i.T(), kialiResult.Ok)

	tracingResult, err := charts.GetChartCaseEndpoint(client, client.RancherConfig.Host, tracingPath, true)
	require.NoError(i.T(), err)
	assert.True(i.T(), tracingResult.Ok)

	// Get a random worker node' public external IP of a specific cluster
	nodeCollection, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": i.project.ClusterID,
	}})
	require.NoError(i.T(), err)
	workerNodePublicIPs := []string{}
	for _, node := range nodeCollection.Data {
		workerNodePublicIPs = append(workerNodePublicIPs, node.Annotations["rke.cattle.io/external-ip"])
	}
	randWorkerNodePublicIP := workerNodePublicIPs[rand.Intn(len(workerNodePublicIPs))]
	istioGatewayHost := randWorkerNodePublicIP + ":" + exampleAppPort

	i.T().Log("Validating example app is accessible")
	exampleAppResult, err := charts.GetChartCaseEndpoint(client, istioGatewayHost, exampleAppProductPagePath, false)
	require.NoError(i.T(), err)
	assert.True(i.T(), exampleAppResult.Ok)

	i.T().Log("Validating example app has three different reviews bodies")
	doesContainFirstPart, err := getChartCaseEndpointUntilBodyHas(client, istioGatewayHost, exampleAppProductPagePath, firstReviewBodyPart)
	require.NoError(i.T(), err)
	assert.True(i.T(), doesContainFirstPart)

	doesContainSecondPart, err := getChartCaseEndpointUntilBodyHas(client, istioGatewayHost, exampleAppProductPagePath, secondReviewBodyPart)
	require.NoError(i.T(), err)
	assert.True(i.T(), doesContainSecondPart)

	doesContainThirdPart, err := getChartCaseEndpointUntilBodyHas(client, istioGatewayHost, exampleAppProductPagePath, thirdReviewBodyPart)
	require.NoError(i.T(), err)
	assert.True(i.T(), doesContainThirdPart)
}

func (i *IstioTestSuite) TestUpgradeIstioChart() {
	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	client, err := i.client.WithSession(subSession)
	require.NoError(i.T(), err)

	steveclient, err := client.Steve.ProxyDownstream(i.project.ClusterID)
	require.NoError(i.T(), err)

	i.T().Log("Checking if the monitoring chart is installed")
	monitoringChart, err := charts.GetChartStatus(client, i.project.ClusterID, charts.RancherMonitoringNamespace, charts.RancherMonitoringName)
	require.NoError(i.T(), err)

	if !monitoringChart.IsAlreadyInstalled {
		i.T().Log("Installing monitoring chart with the latest version")
		err = charts.InstallRancherMonitoringChart(client, i.chartInstallOptions.monitoring, i.chartFeatureOptions.monitoring)
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)

		i.T().Log("Waiting monitoring chart StatefulSets to have expected number of ready replicas")
		err = charts.WatchAndWaitStatefulSets(client, i.project.ClusterID, charts.RancherMonitoringNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)
	}

	// Change istio install option version to previous version of the latest version
	versionsList, err := client.Catalog.GetListChartVersions(charts.RancherIstioName)
	require.NoError(i.T(), err)
	require.Greaterf(i.T(), len(versionsList), 1, "There should be at least 2 versions of the istio chart")
	versionLatest := versionsList[0]
	versionBeforeLatest := versionsList[1]
	i.chartInstallOptions.istio.Version = versionBeforeLatest

	i.T().Log("Checking if the istio chart is installed with one of the previous versions")
	initialIstioChart, err := charts.GetChartStatus(client, i.project.ClusterID, charts.RancherIstioNamespace, charts.RancherIstioName)
	require.NoError(i.T(), err)

	if initialIstioChart.IsAlreadyInstalled && initialIstioChart.ChartDetails.Spec.Chart.Metadata.Version == versionLatest {
		i.T().Skip("Skipping the upgrade case, istio chart is already installed with the latest version")
	}

	if !initialIstioChart.IsAlreadyInstalled {
		i.T().Log("Installing istio chart with the last but one version")
		err = charts.InstallRancherIstioChart(client, i.chartInstallOptions.istio, i.chartFeatureOptions.istio)
		require.NoError(i.T(), err)

		i.T().Log("Waiting istio chart deployments to have expected number of available replicas")
		err = charts.WatchAndWaitDeployments(client, i.project.ClusterID, charts.RancherIstioNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)

		i.T().Log("Waiting istio chart DaemonSets to have expected number of available nodes")
		err = charts.WatchAndWaitDaemonSets(client, i.project.ClusterID, charts.RancherIstioNamespace, metav1.ListOptions{})
		require.NoError(i.T(), err)
	}

	istioChartPreUpgrade, err := charts.GetChartStatus(client, i.project.ClusterID, charts.RancherIstioNamespace, charts.RancherIstioName)
	require.NoError(i.T(), err)

	// Validate current version of rancheristio is one of the versions before latest
	chartVersionPreUpgrade := istioChartPreUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	require.Contains(i.T(), versionsList[1:], chartVersionPreUpgrade)

	// List deployments that have the istio app version as label
	istioVersionPreUpgrade := istioChartPreUpgrade.ChartDetails.Spec.Chart.Metadata.AppVersion
	deploymentListPreUpgrade, err := listIstioDeployments(steveclient)
	require.NoError(i.T(), err)
	require.Equalf(i.T(), 2, len(deploymentListPreUpgrade), "Pilot & Ingressgateways deployments don't have the correct istio version labels")

	for _, deploymentSpec := range deploymentListPreUpgrade {
		imageVersion := strings.Split(deploymentSpec.Template.Spec.Containers[0].Image, ":")[1]
		i.T().Logf("Comparing image and app versions: \n container image version: %v \n istio version: %v and actual: %v\n", deploymentSpec.Template.Spec.Containers[0].Image, istioVersionPreUpgrade, imageVersion)
		require.Containsf(i.T(), imageVersion, istioVersionPreUpgrade, "Pilot & Ingressgateways images don't use the correct istio image version")
	}

	i.chartInstallOptions.istio.Version, err = client.Catalog.GetLatestChartVersion(charts.RancherIstioName)
	require.NoError(i.T(), err)

	i.T().Log("Upgrading istio chart with the latest version")
	err = charts.UpgradeRancherIstioChart(client, i.chartInstallOptions.istio, i.chartFeatureOptions.istio)
	require.NoError(i.T(), err)

	i.T().Log("Waiting istio chart deployments to have expected number of available replicas after upgrade")
	err = charts.WatchAndWaitDeployments(client, i.project.ClusterID, charts.RancherIstioNamespace, metav1.ListOptions{})
	require.NoError(i.T(), err)

	i.T().Log("Waiting istio chart DaemonSets to have expected number of available nodes after upgrade")
	err = charts.WatchAndWaitDaemonSets(client, i.project.ClusterID, charts.RancherIstioNamespace, metav1.ListOptions{})
	require.NoError(i.T(), err)

	istioChartPostUpgrade, err := charts.GetChartStatus(client, i.project.ClusterID, charts.RancherIstioNamespace, charts.RancherIstioName)
	require.NoError(i.T(), err)

	// Compare rancheristio versions
	chartVersionPostUpgrade := istioChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.Version
	assert.Equal(i.T(), i.chartInstallOptions.istio.Version, chartVersionPostUpgrade)

	// List deployments that have the istio app version as label
	istioVersionPostUpgrade := istioChartPostUpgrade.ChartDetails.Spec.Chart.Metadata.AppVersion
	deploymentListPostUpgrade, err := listIstioDeployments(steveclient)
	require.NoError(i.T(), err)
	require.Equalf(i.T(), 2, len(deploymentListPostUpgrade), "Pilot & Ingressgateways deployments don't have the correct istio version labels")

	for _, deploymentSpec := range deploymentListPostUpgrade {
		imageVersion := strings.Split(deploymentSpec.Template.Spec.Containers[0].Image, ":")[1]
		i.T().Logf("Comparing image and app versions: \n container image: %v \n istio version: %v and actual: %v\n", deploymentSpec.Template.Spec.Containers[0].Image, istioVersionPostUpgrade, imageVersion)
		require.Containsf(i.T(), imageVersion, istioVersionPostUpgrade, "Pilot & Ingressgateways images don't use the correct istio image version")
	}
}

func TestIstioTestSuite(t *testing.T) {
	suite.Run(t, new(IstioTestSuite))
}
