
# K3S Provisioning Configs

For your config, you will need everything in the Prerequisites section on the previous readme, [Define your test](#provisioning-input), and at least one [Cloud Credential](#cloud-credentials) and [Node Driver Machine Config](#machine-k3s-config) or [Custom Cluster Template](#custom-cluster), which should match what you have specified in `provisioningInput`. 

Your GO test_package should be set to `provisioning/k3s`.
Your GO suite should be set to `-run ^TestK3SProvisioningTestSuite$`.
Please see below for more details for your config. 

1. [Prerequisites](../README.md)
2. [Define your test](#provisioning-input)
3. [Cloud Credential](#cloud-credentials)
4. [Configure providers to use for Node Driver Clusters](#machine-k3s-config)
5. [Configuring Custom Clusters](#custom-cluster)
6. [Back to general provisioning](../README.md)

## Provisioning Input
provisioningInput is needed to the run the K3S tests, specifically kubernetesVersion and providers. nodesAndRoles is only needed for the TestProvisioningDynamicInput test, node pools are divided by "{nodepool},". 

**nodeProviders is only needed for custom cluster tests; the framework only supports custom clusters through aws/ec2 instances.**

```json
"provisioningInput": {
    "nodesAndRoles": [
      {
        "etcd": true,
        "controlplane": true,
        "worker": true,
        "quantity": 1,
      },
      {
        "worker": true,
        "quantity": 1,
      }
    ],
    "k3sKubernetesVersion": ["v1.24.4+k3s1"],
    "providers": ["linode", "aws", "azure", "harvester"],
    "nodeProviders": ["ec2"],
    "hardened": true
  }
```

## Cloud Credentials
These are the inputs needed for the different node provider cloud credentials, inlcuding linode, aws, harvester, azure, and google.

### Linode
```json
"linodeCredentials": {
   "token": ""
  },
```
### Azure
```json
"azureCredentials": {
   "clientId": "",
   "clientSecret": "",
     "subscriptionId": "",
     "environment": "AzurePublicCloud"
  },
```
### AWS
```json
"awsCredentials": {
   "secretKey": "",
   "accessKey": "",
   "defaultRegion": ""
  },
```
### Harvester
```json
"harvesterCredentials": {
   "clusterId": "",
   "clusterType": "",
   "kubeconfigContent": ""
},
```
### Google
```json
"googleCredentials": {
    "authEncodedJson": ""
}
```

## Machine K3S Config
Machine K3S config is the final piece needed for the config to run K3S provisioning tests.

### AWS K3S Machine Config
```json
"awsMachineConfig": {
    "region": "us-east-2",
    "ami": "",
    "instanceType": "t3a.medium",
    "sshUser": "ubuntu",
    "vpcId": "",
    "volumeType": "gp2",
    "zone": "a",
    "retries": "5",
    "rootSize": "16",
    "securityGroup": ["rancher-nodes"]
},
```
### Linode K3S Machine Config
```json
"linodeMachineConfig": {
  "authorizedUsers": "",
  "createPrivateIp": false,
  "dockerPort": "2376",
  "image": "linode/ubuntu20.04",
  "instanceType": "g6-standard-2",
  "region": "us-west",
  "rootPass": "",
  "sshPort": "22",
  "sshUser": "",
  "stackscript": "",
  "stackscriptData": "",
  "swapSize": "512",
  "tags": "",
  "uaPrefix": ""
},
```
### Azure K3S Machine Config
```json
"azureMachineConfig": {
  "availabilitySet": "docker-machine",
  "diskSize": "30",
  "environment": "AzurePublicCloud",
  "faultDomainCount": "3",
  "image": "canonical:UbuntuServer:18.04-LTS:latest",
  "location": "westus",
  "managedDisks": false,
  "noPublicIp": false,
  "nsg": "",
  "openPort": ["6443/tcp", "2379/tcp", "2380/tcp", "8472/udp", "4789/udp", "9796/tcp", "10256/tcp", "10250/tcp", "10251/tcp", "10252/tcp"],
  "resourceGroup": "docker-machine",
  "size": "Standard_D2_v2",
  "sshUser": "docker-user",
  "staticPublicIp": false,
  "storageType": "Standard_LRS",
  "subnet": "docker-machine",
  "subnetPrefix": "192.168.0.0/16",
  "updateDomainCount": "5",
  "usePrivateIp": false,
  "vnet": "docker-machine-vnet"
},
```
### Harvester K3S Machine Config
```json
"harvesterMachineConfig": {
  "diskSize": "40",
  "cpuCount": "2",
  "memorySize": "8",
  "networkName": "default/ctw-network-1",
  "imageName": "default/image-rpj98",
  "vmNamespace": "default",
  "sshUser": "ubuntu",
  "diskBus": "virtio"
}
```

## Custom Cluster
For custom clusters, the below config is needed, only AWS/EC2 will work.
**Ensure you have nodeProviders in provisioningInput**

```json
 "awsEC2Config": {
    "region": "us-east-2",
    "instanceType": "t3a.medium",
    "awsRegionAZ": "",
    "awsAMI": "",
    "awsSecurityGroups": [""],
    "awsAccessKeyID": "",
    "awsSecretAccessKey": "",
    "awsSSHKeyName": "",
    "awsCICDInstanceTag": "",
    "awsIAMProfile": "",
    "awsUser": "ubuntu",
    "volumeSize": 50
  },
```