package corral

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

// The json/yaml config key for the corral package to be build ..
const (
	corralPackageConfigConfigurationFileKey = "corralPackages"
	corralConfigConfigurationFileKey        = "corralConfigs"
)

// CorralConfigs is a struct that for necessary corral config environment variables to build a corral
type CorralConfigs struct {
	CorralConfigVars map[string]string `json:"corralConfigVars" yaml:"corralConfigVars"`
	CorralConfigUser string            `json:"corralConfigUser" yaml:"corralConfigUser" default:"jenkauto"`
	CorralSSHPath    string            `json:"corralSSHPath" yaml:"corralSSHPath" default:"/root/.ssh/public.pub"`
}

// CorralPackages is a struct that has the path to the packages
type CorralPackages struct {
	CorralPackageImages map[string]string `json:"corralPackageImages" yaml:"corralPackageImages"`
	Cleanup             bool              `json:"cleanup" yaml:"cleanup" default:"true"`
	Debug               bool              `json:"debug" yaml:"debug" default:"false"`
	CustomRepo          string            `json:"customRepo" yaml:"customRepo" default:"https://github.com/rancherlabs/corral-packages.git"`
}

// CorralPackagesConfig is a function that reads in the corral package object from the config file
func CorralPackagesConfig() *CorralPackages {
	var corralPackages CorralPackages
	config.LoadConfig(corralPackageConfigConfigurationFileKey, &corralPackages)
	return &corralPackages
}

// CorralConfigurations is a function that reads in the corral config vars from the config file
func CorralConfigurations() *CorralConfigs {
	var corralConfigs CorralConfigs
	config.LoadConfig(corralConfigConfigurationFileKey, &corralConfigs)
	return &corralConfigs
}
