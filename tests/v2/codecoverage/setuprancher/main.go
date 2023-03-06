package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/clients/dynamic"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/workloads/deployments"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	aws "github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates/aws"
	"github.com/rancher/rancher/tests/framework/extensions/token"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	passwordgenerator "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	appv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sUnstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	adminPassword = os.Getenv("ADMIN_PASSWORD")
)

const (
	corralName       = "ranchertestcoverage"
	rancherTestImage = "ranchertest/rancher:v2.7-head"
	namespace        = "cattle-system"
	deploymentName   = "rancher"
	clusterName      = "local"
	// The json/yaml config key for the corral package to be build ..
	userClusterConfigsConfigurationFileKey = "userClusterConfig"
)

type UserClusterConfig struct {
	Token         string   `json:"token" yaml:"token"`
	Username      string   `json:"username" yaml:"username"`
	Clusters      []string `json:"clusters" yaml:"clusters"`
	AdminPassword string   `json:"AdminPassword" yaml:"AdminPassword"`
}

// setup for code coverage testing and reporting
func main() {
	rancherConfig := new(rancher.Config)
	config.LoadConfig(rancher.ConfigurationFileKey, rancherConfig)

	kubeconfig, err := getRancherKubeconfig()
	if err != nil {
		logrus.Fatalf("error with getting kube config using corral: %v", err)
	}

	password, err := corral.GetCorralEnvVar(corralName, "bootstrap_password")
	if err != nil {
		logrus.Fatalf("error getting password %v", err)
	}

	// update deployment
	err = updateRancherDeployment(kubeconfig)
	if err != nil {
		logrus.Fatalf("error updating rancher deployment: %v", err)
	}

	token, err := createAdminToken(password, rancherConfig)
	if err != nil {
		logrus.Fatalf("error with generating admin token: %v", err)
	}

	rancherConfig.AdminToken = token
	config.UpdateConfig(rancher.ConfigurationFileKey, rancherConfig)
	//provision clusters for test
	session := session.NewSession()
	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)
	kubernetesVersions := clustersConfig.RKE1KubernetesVersions
	cnis := clustersConfig.CNIs
	nodesAndRoles := clustersConfig.NodesAndRolesRKE1

	client, err := rancher.NewClient("", session)
	if err != nil {
		logrus.Fatalf("error creating admin client: %v", err)
	}

	err = postRancherInstall(client)
	if err != nil {
		logrus.Fatalf("error with admin user service account secret: %v", err)
	}

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = passwordgenerator.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	if err != nil {
		logrus.Fatalf("error creating admin client: %v", err)
	}

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	if err != nil {
		logrus.Fatalf("error creating standard user client: %v", err)
	}

	// create admin cluster
	adminClusterNames, err := createTestCluster(client, client, 1, "admintestcluster", cnis[0], kubernetesVersions[0], nodesAndRoles)
	if err != nil {
		logrus.Fatalf("error creating admin user cluster: %v", err)
	}

	// create standard user clusters
	standardClusterNames, err := createTestCluster(standardUserClient, client, 2, "standardtestcluster", cnis[0], kubernetesVersions[0], nodesAndRoles)
	if err != nil {
		logrus.Fatalf("error creating standard user clusters: %v", err)
	}

	//update userconfig
	userClusterConfig := UserClusterConfig{}
	userClusterConfig.Token = standardUserClient.Steve.Opts.TokenKey
	userClusterConfig.Username = newUser.Name
	userClusterConfig.AdminPassword = adminPassword
	userClusterConfig.Clusters = standardClusterNames

	rancherConfig.ClusterName = adminClusterNames[0]
	config.UpdateConfig(rancher.ConfigurationFileKey, rancherConfig)

	err = writeToConfigFile(userClusterConfig)
	if err != nil {
		logrus.Fatalf("error writing config file: %v", err)
	}
}

func getRancherKubeconfig() ([]byte, error) {
	kubeconfig, err := corral.GetKubeConfig(corralName)
	if err != nil {
		return nil, err
	}

	return kubeconfig, nil
}

func createAdminToken(password string, rancherConfig *rancher.Config) (string, error) {
	adminUser := &management.User{
		Username: "admin",
		Password: password,
	}

	hostURL := rancherConfig.Host
	var userToken *management.Token
	err := kwait.Poll(500*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		userToken, err = token.GenerateUserToken(adminUser, hostURL)
		if err != nil {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return "", err
	}

	return userToken.Token, nil
}

func updateRancherDeployment(kubeconfig []byte) error {
	session := session.NewSession()
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return err
	}

	restConfig, err := (clientConfig).ClientConfig()
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(session, restConfig)
	if err != nil {
		return err
	}

	deploymentResource := dynamicClient.Resource(deployments.DeploymentGroupVersionResource)

	cattleSystemDeploymentResource := deploymentResource.Namespace(namespace)
	unstructuredDeployment, err := cattleSystemDeploymentResource.Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	updatedDeployment := &appv1.Deployment{}
	err = scheme.Scheme.Convert(unstructuredDeployment, updatedDeployment, unstructuredDeployment.GroupVersionKind())
	if err != nil {
		return err
	}

	updatedDeployment.Spec.Template.Spec.Containers[0].Args = []string{}
	updatedDeployment.Spec.Template.Spec.Containers[0].Image = rancherTestImage
	updatedDeployment.Spec.Strategy.RollingUpdate = nil
	updatedDeployment.Spec.Strategy.Type = appv1.RecreateDeploymentStrategyType

	unstructuredResp, err := cattleSystemDeploymentResource.Update(context.TODO(), unstructured.MustToUnstructured(updatedDeployment), metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		_, err = cattleSystemDeploymentResource.Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	newDeployment := &appv1.Deployment{}
	err = scheme.Scheme.Convert(unstructuredResp, newDeployment, unstructuredResp.GroupVersionKind())
	if err != nil {
		return err
	}

	// wait until pods are ready with new images

	result, err := cattleSystemDeploymentResource.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + deploymentName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(result, func(event watch.Event) (ready bool, err error) {
		deploymentUnstructured := event.Object.(*k8sUnstructured.Unstructured)
		newDeployment := &appv1.Deployment{}
		err = scheme.Scheme.Convert(deploymentUnstructured, newDeployment, deploymentUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}
		if newDeployment.Status.ReadyReplicas == int32(3) {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return err
	}

	err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		_, err = cattleSystemDeploymentResource.Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if k8sErrors.IsInternalError(err) || k8sErrors.IsServiceUnavailable(err) {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	err = kwait.Poll(500*time.Millisecond, 10*time.Minute, func() (done bool, err error) {
		webhookDeployment, err := cattleSystemDeploymentResource.Get(context.TODO(), "rancher-webhook", metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}

		newDeployment := &appv1.Deployment{}
		err = scheme.Scheme.Convert(webhookDeployment, newDeployment, webhookDeployment.GroupVersionKind())
		if err != nil {
			return false, err
		}
		if newDeployment.Status.ReadyReplicas == int32(1) {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return err
	}

	cattleFleetLocalSystemDeploymentResource := deploymentResource.Namespace("cattle-fleet-local-system")
	err = kwait.Poll(500*time.Millisecond, 10*time.Minute, func() (done bool, err error) {
		fleetAgentDeployment, err := cattleFleetLocalSystemDeploymentResource.Get(context.TODO(), "fleet-agent", metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		newDeployment := &appv1.Deployment{}
		err = scheme.Scheme.Convert(fleetAgentDeployment, newDeployment, fleetAgentDeployment.GroupVersionKind())
		if err != nil {
			return false, err
		}
		if newDeployment.Status.ReadyReplicas == int32(1) {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return err
	}

	cattleFleetSystemDeploymentResource := deploymentResource.Namespace("cattle-fleet-system")
	err = kwait.Poll(500*time.Millisecond, 10*time.Minute, func() (done bool, err error) {
		fleetControllerDeployment, err := cattleFleetSystemDeploymentResource.Get(context.TODO(), "fleet-controller", metav1.GetOptions{})
		if k8sErrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}

		newDeployment := &appv1.Deployment{}
		err = scheme.Scheme.Convert(fleetControllerDeployment, newDeployment, fleetControllerDeployment.GroupVersionKind())
		if err != nil {
			return false, err
		}
		if newDeployment.Status.ReadyReplicas == int32(1) {
			return true, nil
		}
		return false, nil
	})

	return err
}

func createTestCluster(client, adminClient *rancher.Client, numClusters int, clusterNameBase, cni, kubeVersion string, nodesAndRoles []nodepools.NodeRoles) ([]string, error) {
	clusterNames := []string{}
	for i := 0; i < numClusters; i++ {
		clusterName := namegen.AppendRandomString(clusterNameBase)
		clusterNames = append(clusterNames, clusterName)
		cluster := clusters.NewRKE1ClusterConfig(clusterName, cni, kubeVersion, client)

		clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
		if err != nil {
			return nil, err
		}

		err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
			cluster, err := client.Management.Cluster.ByID(clusterResp.ID)
			if err != nil {
				return false, nil
			} else if cluster != nil && cluster.ID == clusterResp.ID {
				return true, nil
			}
			return false, nil
		})

		if err != nil {
			return nil, err
		}

		nodeTemplateResp, err := aws.CreateAWSNodeTemplate(client)
		if err != nil {
			return nil, err
		}

		err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
			nodeTemplate, err := client.Management.NodeTemplate.ByID(nodeTemplateResp.ID)
			if err != nil {
				return false, nil
			} else if nodeTemplate != nil && nodeTemplate.ID == nodeTemplateResp.ID {
				return true, nil
			}
			return false, nil
		})

		if err != nil {
			return nil, err
		}

		err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
			_, err = nodepools.NodePoolSetup(client, nodesAndRoles, clusterResp.ID, nodeTemplateResp.ID)
			if err != nil {
				return false, nil
			}

			return true, nil
		})

		if err != nil {
			return nil, err
		}
		fmt.Println("before node pool poll")

		opts := metav1.ListOptions{
			FieldSelector:  "metadata.name=" + clusterResp.ID,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		}
		watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, opts)
		if err != nil {
			return nil, err
		}

		checkFunc := clusters.IsHostedProvisioningClusterReady

		err = wait.WatchWait(watchInterface, checkFunc)
		if err != nil {
			return nil, err
		}
	}
	return clusterNames, nil
}

func writeToConfigFile(config UserClusterConfig) error {
	result := map[string]UserClusterConfig{}
	result[userClusterConfigsConfigurationFileKey] = config

	yamlConfig, err := yaml.Marshal(result)

	if err != nil {
		return err
	}

	return os.WriteFile("userclusterconfig.yaml", yamlConfig, 0644)
}

func postRancherInstall(adminClient *rancher.Client) error {
	clusterID, err := clusters.GetClusterIDByName(adminClient, clusterName)
	if err != nil {
		return err
	}

	steveClient, err := adminClient.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	timeStamp := time.Now().Format(time.RFC3339)
	settingEULA := v3.Setting{
		ObjectMeta: metav1.ObjectMeta{
			Name: "eula-agreed",
		},
		Default: timeStamp,
		Value:   timeStamp,
	}

	urlSetting := &v3.Setting{}

	_, err = steveClient.SteveType("management.cattle.io.setting").Create(settingEULA)
	if err != nil {
		return err
	}

	urlSettingResp, err := steveClient.SteveType("management.cattle.io.setting").ByID("server-url")
	if err != nil {
		return err
	}

	err = v1.ConvertToK8sType(urlSettingResp.JSONResp, urlSetting)
	if err != nil {
		return err
	}

	urlSetting.Value = fmt.Sprintf("https://%s", adminClient.RancherConfig.Host)

	_, err = steveClient.SteveType("management.cattle.io.setting").Update(urlSettingResp, urlSetting)
	if err != nil {
		return err
	}

	userList, err := adminClient.Management.User.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"username": "admin",
		},
	})
	if err != nil {
		return err
	} else if len(userList.Data) == 0 {
		return fmt.Errorf("admin user not found")
	}

	adminUser := &userList.Data[0]
	setPasswordInput := management.SetPasswordInput{
		NewPassword: adminPassword,
	}
	_, err = adminClient.Management.User.ActionSetpassword(adminUser, &setPasswordInput)

	return err
}
