package rancher

import (
	"github.com/mcuadros/go-version"
	"github.com/rancher/norman/condition"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	controllerapiextv1 "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/summary"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	v1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	cattleNamespace                            = "cattle-system"
	forceUpgradeLogoutConfig                   = "forceupgradelogout"
	forceLocalSystemAndDefaultProjectCreation  = "forcelocalprojectcreation"
	forceSystemNamespacesAssignment            = "forcesystemnamespaceassignment"
	migrateFromMachineToPlanSecret             = "migratefrommachinetoplanesecret"
	migrateEncryptionKeyRotationLeaderToStatus = "migrateencryptionkeyrotationleadertostatus"
	rancherVersionKey                          = "rancherVersion"
	projectsCreatedKey                         = "projectsCreated"
	namespacesAssignedKey                      = "namespacesAssigned"
	capiMigratedKey                            = "capiMigrated"
	encryptionKeyRotationStatusMigratedKey     = "encryptionKeyRotationStatusMigrated"
)

func runMigrations(wranglerContext *wrangler.Context) error {
	if err := forceUpgradeLogout(wranglerContext.Core.ConfigMap(), wranglerContext.Mgmt.Token(), "v2.6.0"); err != nil {
		return err
	}

	if err := forceSystemAndDefaultProjectCreation(wranglerContext.Core.ConfigMap(), wranglerContext.Mgmt.Cluster()); err != nil {
		return err
	}

	if features.MCM.Enabled() {
		if err := forceSystemNamespaceAssignment(wranglerContext.Core.ConfigMap(), wranglerContext.Mgmt.Project()); err != nil {
			return err
		}
	}

	if features.EmbeddedClusterAPI.Enabled() {
		if err := addWebhookConfigToCAPICRDs(wranglerContext.CRD.CustomResourceDefinition()); err != nil {
			return err
		}
	}

	if features.RKE2.Enabled() {
		if err := migrateCAPIMachineLabelsAndAnnotationsToPlanSecret(wranglerContext); err != nil {
			return err
		}
		if err := migrateEncryptionKeyRotationLeader(wranglerContext); err != nil {
			return err
		}
	}

	return nil
}

func getConfigMap(configMapController controllerv1.ConfigMapController, configMapName string) (*v1.ConfigMap, error) {
	cm, err := configMapController.Cache().Get(cattleNamespace, configMapName)
	if err != nil && !k8serror.IsNotFound(err) {
		return nil, err
	}

	// if this is the first ever migration initialize the configmap
	if cm == nil {
		cm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: cattleNamespace,
			},
			Data: make(map[string]string, 1),
		}
	}

	// we do not migrate in development environments
	if rancherversion.Version == "dev" {
		return nil, nil
	}

	return cm, nil
}

func createOrUpdateConfigMap(configMapClient controllerv1.ConfigMapClient, cm *v1.ConfigMap) error {
	var err error
	if cm.ObjectMeta.GetResourceVersion() != "" {
		_, err = configMapClient.Update(cm)
	} else {
		_, err = configMapClient.Create(cm)
	}

	return err
}

// forceUpgradeLogout will delete all dashboard tokens forcing a logout.  This is useful when there is a major frontend
// upgrade and we want all users to be sent to a central point.  This function will check for the `forceUpgradeLogoutConfig`
// configuration map and only run if the last migrated version is lower than the given `migrationVersion`.
func forceUpgradeLogout(configMapController controllerv1.ConfigMapController, tokenController v3.TokenController, migrationVersion string) error {
	cm, err := getConfigMap(configMapController, forceUpgradeLogoutConfig)
	if err != nil || cm == nil {
		return err
	}

	// if no last migration is found we always run force logout
	if lastMigration, ok := cm.Data[rancherVersionKey]; ok {

		// if a valid sem ver is found we only migrate if the version is less than the target version
		if semver.IsValid(lastMigration) && semver.IsValid(rancherversion.Version) && version.Compare(migrationVersion, lastMigration, "<=") {
			return nil
		}

		// if an unknown format is given we migrate any time the current version does not equal the last migration
		if lastMigration == rancherversion.Version {
			return nil
		}
	}

	logrus.Infof("Detected %s upgrade, forcing logout for all web users", migrationVersion)

	// list all tokens that were created for the dashboard
	allTokens, err := tokenController.Cache().List(labels.SelectorFromSet(labels.Set{tokens.TokenKindLabel: "session"}))
	if err != nil {
		logrus.Error("Failed to list tokens for upgrade forced logout")
		return err
	}

	// log out all the dashboard users forcing them to be redirected to the login page
	for _, token := range allTokens {
		err = tokenController.Delete(token.ObjectMeta.Name, &metav1.DeleteOptions{})
		if err != nil && !k8serror.IsNotFound(err) {
			logrus.Errorf("Failed to delete token [%s] for upgrade forced logout", token.Name)
		}
	}

	cm.Data[rancherVersionKey] = rancherversion.Version
	return createOrUpdateConfigMap(configMapController, cm)
}

// forceSystemAndDefaultProjectCreation will set the corresponding conditions on the local cluster object,
// if it exists, to Unknown. This will force the corresponding controller to check that the projects exist
// and create them, if necessary.
func forceSystemAndDefaultProjectCreation(configMapController controllerv1.ConfigMapController, clusterClient v3.ClusterClient) error {
	cm, err := getConfigMap(configMapController, forceLocalSystemAndDefaultProjectCreation)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[projectsCreatedKey] == "true" {
		return nil
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		localCluster, err := clusterClient.Get("local", metav1.GetOptions{})
		if err != nil {
			if k8serror.IsNotFound(err) {
				return nil
			}
			return err
		}

		v32.ClusterConditionDefaultProjectCreated.Unknown(localCluster)
		v32.ClusterConditionSystemProjectCreated.Unknown(localCluster)

		_, err = clusterClient.Update(localCluster)
		return err
	}); err != nil {
		return err
	}

	cm.Data[projectsCreatedKey] = "true"
	return createOrUpdateConfigMap(configMapController, cm)
}

func forceSystemNamespaceAssignment(configMapController controllerv1.ConfigMapController, projectClient v3.ProjectClient) error {
	cm, err := getConfigMap(configMapController, forceSystemNamespacesAssignment)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[namespacesAssignedKey] == rancherversion.Version {
		return nil
	}

	err = applyProjectConditionForNamespaceAssignment("authz.management.cattle.io/system-project=true", v32.ProjectConditionSystemNamespacesAssigned, projectClient)
	if err != nil {
		return err
	}
	err = applyProjectConditionForNamespaceAssignment("authz.management.cattle.io/default-project=true", v32.ProjectConditionDefaultNamespacesAssigned, projectClient)
	if err != nil {
		return err
	}

	cm.Data[namespacesAssignedKey] = rancherversion.Version
	return createOrUpdateConfigMap(configMapController, cm)
}

func applyProjectConditionForNamespaceAssignment(label string, condition condition.Cond, projectClient v3.ProjectClient) error {
	projects, err := projectClient.List("", metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return err
	}

	for i := range projects.Items {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			p := &projects.Items[i]
			p, err = projectClient.Get(p.Namespace, p.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			condition.Unknown(p)
			_, err = projectClient.Update(p)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func addWebhookConfigToCAPICRDs(crdClient controllerapiextv1.CustomResourceDefinitionClient) error {
	crds, err := crdClient.List(metav1.ListOptions{
		LabelSelector: "auth.cattle.io/cluster-indexed=true",
	})
	if err != nil {
		return err
	}
	for _, crd := range crds.Items {
		if crd.Spec.Group != "cluster.x-k8s.io" || (crd.Spec.Conversion != nil && crd.Spec.Conversion.Strategy != apiextv1.NoneConverter) {
			continue
		}
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latestCrd, err := crdClient.Get(crd.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			latestCrd.Spec.Conversion = &apiextv1.CustomResourceConversion{
				Strategy: apiextv1.WebhookConverter,
				Webhook: &apiextv1.WebhookConversion{
					ClientConfig: &apiextv1.WebhookClientConfig{
						Service: &apiextv1.ServiceReference{
							Namespace: "cattle-system",
							Name:      "webhook-service",
							Path:      &[]string{"/convert"}[0],
							Port:      &[]int32{443}[0],
						},
					},
					ConversionReviewVersions: []string{"v1", "v1beta1"},
				},
			}
			_, err = crdClient.Update(&crd)
			return err
		}); err != nil {
			return err
		}
	}

	return nil
}

func migrateCAPIMachineLabelsAndAnnotationsToPlanSecret(w *wrangler.Context) error {
	cm, err := getConfigMap(w.Core.ConfigMap(), migrateFromMachineToPlanSecret)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[capiMigratedKey] == "true" {
		return nil
	}

	mgmtClusters, err := w.Mgmt.Cluster().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	bootstrapLabelExcludes := map[string]struct{}{
		rke2.InitNodeMachineIDLabel: {},
		rke2.InitNodeLabel:          {},
	}

	boostrapAnnotationExcludes := map[string]struct{}{
		rke2.DrainAnnotation:     {},
		rke2.DrainDoneAnnotation: {},
		rke2.JoinURLAnnotation:   {},
		rke2.PostDrainAnnotation: {},
		rke2.PreDrainAnnotation:  {},
		rke2.UnCordonAnnotation:  {},
	}

	for _, mgmtCluster := range mgmtClusters.Items {
		provClusters, err := w.Provisioning.Cluster().List(mgmtCluster.Spec.FleetWorkspaceName, metav1.ListOptions{})
		if k8serror.IsNotFound(err) || len(provClusters.Items) == 0 {
			continue
		} else if err != nil {
			return err
		}

		for _, provCluster := range provClusters.Items {
			machines, err := w.CAPI.Machine().List(provCluster.Namespace, metav1.ListOptions{LabelSelector: labels.Set{capi.ClusterLabelName: provCluster.Name}.String()})
			if err != nil {
				return err
			}

			otherMachines, err := w.CAPI.Machine().List(provCluster.Namespace, metav1.ListOptions{LabelSelector: labels.Set{rke2.ClusterNameLabel: provCluster.Name}.String()})
			if err != nil {
				return err
			}

			allMachines := append(machines.Items, otherMachines.Items...)

			for _, machine := range allMachines {
				if machine.Spec.Bootstrap.ConfigRef == nil || machine.Spec.Bootstrap.ConfigRef.APIVersion != rke2.RKEAPIVersion {
					continue
				}

				planSecrets, err := w.Core.Secret().List(machine.Namespace, metav1.ListOptions{LabelSelector: labels.Set{rke2.MachineNameLabel: machine.Name}.String()})
				if err != nil {
					return err
				}
				if len(planSecrets.Items) == 0 {
					continue
				}

				for _, secret := range planSecrets.Items {
					if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						secret, err := w.Core.Secret().Get(secret.Namespace, secret.Name, metav1.GetOptions{})
						if err != nil {
							return err
						}

						secret = secret.DeepCopy()
						rke2.CopyMap(secret.Labels, machine.Labels)
						rke2.CopyMap(secret.Annotations, machine.Annotations)
						_, err = w.Core.Secret().Update(secret)
						return err
					}); err != nil {
						return err
					}
				}

				if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
					bootstrap, err := w.RKE.RKEBootstrap().Get(machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					bootstrap = bootstrap.DeepCopy()
					rke2.CopyMapWithExcludes(bootstrap.Labels, machine.Labels, bootstrapLabelExcludes)
					rke2.CopyMapWithExcludes(bootstrap.Annotations, machine.Annotations, boostrapAnnotationExcludes)
					if bootstrap.Spec.ClusterName == "" {
						// If the bootstrap spec cluster name is blank, we need to update the bootstrap spec to the correct value
						// This is to handle old rkebootstrap objects for unmanaged clusters that did not have the spec properly set
						if v, ok := bootstrap.Labels[capi.ClusterLabelName]; ok && v != "" {
							bootstrap.Spec.ClusterName = v
						}
					}
					_, err = w.RKE.RKEBootstrap().Update(bootstrap)
					return err
				}); err != nil {
					return err
				}

				if machine.Spec.InfrastructureRef.APIVersion == rke2.RKEAPIVersion || machine.Spec.InfrastructureRef.APIVersion == rke2.RKEMachineAPIVersion {
					gv, err := schema.ParseGroupVersion(machine.Spec.InfrastructureRef.APIVersion)
					if err != nil {
						// This error should not occur because RKEAPIVersion and RKEMachineAPIVersion are valid
						continue
					}

					gvk := schema.GroupVersionKind{
						Group:   gv.Group,
						Version: gv.Version,
						Kind:    machine.Spec.InfrastructureRef.Kind,
					}
					if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
						infraMachine, err := w.Dynamic.Get(gvk, machine.Spec.InfrastructureRef.Namespace, machine.Spec.InfrastructureRef.Name)
						if err != nil {
							return err
						}

						d, err := data.Convert(infraMachine.DeepCopyObject())
						if err != nil {
							return err
						}

						if changed, err := insertOrUpdateCondition(d, summary.NewCondition("Ready", "True", "", "")); err != nil {
							return err
						} else if changed {
							_, err = w.Dynamic.UpdateStatus(&unstructured.Unstructured{Object: d})
							return err
						}
						return err
					}); err != nil {
						return err
					}
				}
			}
		}
	}

	cm.Data[capiMigratedKey] = "true"
	return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
}

func migrateEncryptionKeyRotationLeader(w *wrangler.Context) error {
	cm, err := getConfigMap(w.Core.ConfigMap(), migrateEncryptionKeyRotationLeaderToStatus)
	if err != nil || cm == nil {
		return err
	}

	if cm.Data[encryptionKeyRotationStatusMigratedKey] == "true" {
		return nil
	}

	mgmtClusters, err := w.Mgmt.Cluster().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mgmtCluster := range mgmtClusters.Items {
		if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			cp, err := w.RKE.RKEControlPlane().Get(mgmtCluster.Spec.FleetWorkspaceName, mgmtCluster.Name, metav1.GetOptions{})
			if k8serror.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			leader := cp.Annotations["rke.cattle.io/encrypt-key-rotation-leader"]
			if leader == "" {
				return nil
			}
			cp = cp.DeepCopy()
			cp.Status.RotateEncryptionKeysLeader = leader
			cp, err = w.RKE.RKEControlPlane().UpdateStatus(cp)
			if err != nil {
				return err
			}
			cp = cp.DeepCopy()
			delete(cp.Annotations, "rke.cattle.io/encrypt-key-rotation-leader")
			cp, err = w.RKE.RKEControlPlane().Update(cp)
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}

	cm.Data[encryptionKeyRotationStatusMigratedKey] = "true"
	return createOrUpdateConfigMap(w.Core.ConfigMap(), cm)
}

func insertOrUpdateCondition(d data.Object, desiredCondition summary.Condition) (bool, error) {
	for _, cond := range summary.GetUnstructuredConditions(d) {
		if desiredCondition.Equals(cond) {
			return false, nil
		}
	}

	// The conditions must be converted to a map so that DeepCopyJSONValue will
	// recognize it as a map instead of a data.Object.
	newCond, err := convert.EncodeToMap(desiredCondition.Object)
	if err != nil {
		return false, err
	}

	dConditions := d.Slice("status", "conditions")
	conditions := make([]interface{}, len(dConditions))
	found := false
	for i, cond := range dConditions {
		if cond.String("type") == desiredCondition.Type() {
			conditions[i] = newCond
			found = true
		} else {
			conditions[i], err = convert.EncodeToMap(cond)
			if err != nil {
				return false, err
			}
		}
	}

	if !found {
		conditions = append(conditions, newCond)
	}
	d.SetNested(conditions, "status", "conditions")

	return true, nil
}
