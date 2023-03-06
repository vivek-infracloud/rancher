// Package chart is used for defining helm chart information.
package chart

import (
	"fmt"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
)

const (
	// PriorityClassKey is the name of the helm value used for setting the name of the priority class on pods deployed by rancher and feature charts.
	PriorityClassKey = "priorityClassName"
)

// Manager is an interface used by the handler to install an uninstall charts
type Manager interface {
	// Ensure ensures that the chart is installed into the given namespace with the given minVersion version and values.
	Ensure(namespace, name, minVersion string, values map[string]interface{}, forceAdopt bool) error

	// Uninstall uninstalls the given chart in the given namespace.
	Uninstall(namespace, name string) error

	// Remove removes the chart from the desired state
	Remove(namespace, name, minVersion string)
}

// Definition defines a helm chart.
type Definition struct {
	ReleaseNamespace  string
	ChartName         string
	MinVersionSetting settings.Setting
	Values            func() map[string]interface{}
	Enabled           func() bool
	Uninstall         bool
	RemoveNamespace   bool
}

// RancherConfigGetter is used to get Rancher chart configuration information from the rancher config map
type RancherConfigGetter struct {
	ConfigCache corev1.ConfigMapCache
}

// GetPriorityClassName attempts to retrieve the priority class for rancher pods and feature charts as set via helm values.
func (r *RancherConfigGetter) GetPriorityClassName() (string, error) {
	configMap, err := r.ConfigCache.Get(namespace.System, settings.ConfigMapName.Get())
	if err != nil {
		return "", fmt.Errorf("failed to get rancher config: %w", err)
	}
	priorityClassName, ok := configMap.Data[PriorityClassKey]
	if !ok {
		return "", fmt.Errorf("%q not set in %q configMap", PriorityClassKey, configMap.Name)
	}
	return priorityClassName, nil
}
