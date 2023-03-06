module github.com/rancher/rancher/pkg/client

go 1.19

replace k8s.io/client-go => github.com/rancher/client-go v1.25.4-rancher1

require (
	github.com/rancher/norman v0.0.0-20221205184727-32ef2e185b99
	k8s.io/apimachinery v0.25.4
)

require (
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rancher/wrangler v1.0.1-0.20230131212012-76adc44fca0c // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.70.1 // indirect
)
