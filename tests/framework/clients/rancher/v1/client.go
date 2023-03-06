package v1

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	"github.com/rancher/apiserver/pkg/types"
	normantypes "github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/pkg/clientbase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	hostRegex = "https://(.+)/v1"
)

// State is the Steve specific field in the rancher Steve API
type State struct {
	Error         bool   `json:"error,omitempty" yaml:"error,omitempty"`
	Message       string `json:"message,omitempty" yaml:"message,omitempty"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	Transitioning bool   `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
}

// ObjectMeta is the native k8s object meta field that kubernetes objects used, with the added
// Steve API State field.
type ObjectMeta struct {
	metav1.ObjectMeta
	State *State `json:"state,omitempty" yaml:"state,omitempty"`
}

// SteveAPIObject is the generic object used in the v1/steve API call responses
type SteveAPIObject struct {
	normantypes.Resource
	JSONResp        map[string]any
	metav1.TypeMeta `json:",inline"`
	ObjectMeta      `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec            any `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status          any `json:"status,omitempty" yaml:"status,omitempty"`
}

// SteveCollection is the collection type of the SteveAPIObjects
type SteveCollection struct {
	types.Collection
	Data   []SteveAPIObject `json:"data,omitempty"`
	client *SteveClient
}

// SteveClient is the client used to access Steve API endpoints
type SteveClient struct {
	apiClient *Client
	steveType string
}

// NamespacedSteveClient is the client used to access namespaced Steve API endpoints
type NamespacedSteveClient struct {
	SteveClient
	namespace string
}

type SteveOperations interface {
	List(params url.Values) (*SteveCollection, error)
	ListAll(params url.Values) (*SteveCollection, error)
	Create(opts any) (*SteveAPIObject, error)
	Update(existing *SteveAPIObject, updates any) (*SteveAPIObject, error)
	Replace(existing *SteveAPIObject) (*SteveAPIObject, error)
	ByID(id string) (*SteveAPIObject, error)
	Delete(container *SteveAPIObject) error
}

type Client struct {
	clientbase.APIBaseClient
}

func NewClient(opts *clientbase.ClientOpts) (*Client, error) {
	baseClient, err := clientbase.NewAPIClient(opts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		APIBaseClient: baseClient,
	}

	return client, nil
}

// SteveType is a function that sets the resource type for the SteveClient
// e.g. accessing the Steve namespace resource
//
//	nameSpaceClient := client.V1.SteveType("namespace")
func (c *Client) SteveType(steveType string) *SteveClient {
	return &SteveClient{
		apiClient: c,
		steveType: steveType,
	}
}

func (c *SteveClient) NamespacedSteveClient(namespace string) *NamespacedSteveClient {
	return &NamespacedSteveClient{*c, namespace}
}

// ProxyDownstream is a function that sets the URL to a proxy URL
// to be able to make Steve API calls to a downstream cluster
func (c *Client) ProxyDownstream(clusterID string) (*Client, error) {
	hostRegexp := regexp.MustCompile(hostRegex)

	matches := hostRegexp.FindStringSubmatch(c.Opts.URL)
	host := matches[1]

	updatedOpts := *c.Opts
	proxyHost := fmt.Sprintf("https://%s/k8s/clusters/%s/v1", host, clusterID)
	updatedOpts.URL = proxyHost

	baseClient, err := clientbase.NewAPIClient(&updatedOpts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		APIBaseClient: baseClient,
	}
	client.Ops.Session = c.Ops.Session

	return client, nil
}

func (c *SteveClient) Create(container any) (*SteveAPIObject, error) {
	resp := &SteveAPIObject{}
	var jsonResp map[string]any
	err := c.apiClient.Ops.DoCreate(c.steveType, container, &jsonResp)
	if err != nil {
		return nil, err
	}
	err = ConvertToK8sType(jsonResp, resp)
	resp.JSONResp = jsonResp
	return resp, err
}

func (c *SteveClient) Update(existing *SteveAPIObject, updates any) (*SteveAPIObject, error) {
	resp := &SteveAPIObject{}
	var jsonResp map[string]any
	err := c.apiClient.Ops.DoUpdate(c.steveType, &existing.Resource, updates, &jsonResp)
	if err != nil {
		return nil, err
	}
	err = ConvertToK8sType(jsonResp, resp)
	resp.JSONResp = jsonResp
	return resp, err
}

func (c *SteveClient) Replace(obj *SteveAPIObject) (*SteveAPIObject, error) {
	resp := &SteveAPIObject{}
	var jsonResp map[string]any
	err := c.apiClient.Ops.DoReplace(c.steveType, &obj.Resource, obj, &jsonResp)
	if err != nil {
		return nil, err
	}
	err = ConvertToK8sType(jsonResp, resp)
	resp.JSONResp = jsonResp
	return resp, err
}

func (c *SteveClient) List(query url.Values) (*SteveCollection, error) {
	resp := &SteveCollection{}
	var jsonResp map[string]any
	url, err := c.apiClient.Ops.GetCollectionURL(c.steveType, "GET")
	if err != nil {
		return nil, err
	}
	url = url + "?" + query.Encode()
	err = c.apiClient.Ops.DoGet(url, nil, &jsonResp)
	if err != nil {
		return nil, err
	}

	err = ConvertToK8sType(jsonResp, resp)
	if err != nil {
		return nil, err
	}

	steveList := jsonResp["data"]
	for index, item := range steveList.([]any) {
		resp.Data[index].JSONResp = item.(map[string]any)
	}
	return resp, err
}

func (c *SteveClient) ListAll(params url.Values) (*SteveCollection, error) {
	resp, err := c.List(params)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (sc *SteveCollection) Next() (*SteveCollection, error) {
	if sc != nil && sc.Pagination != nil && sc.Pagination.Next != "" {
		resp := &SteveCollection{}
		err := sc.client.apiClient.Ops.DoNext(sc.Pagination.Next, resp)
		resp.client = sc.client
		return resp, err
	}
	return nil, nil
}

func (c *SteveClient) ByID(id string) (*SteveAPIObject, error) {
	resp := &SteveAPIObject{}
	var jsonResp map[string]any

	err := c.apiClient.Ops.DoByID(c.steveType, id, &jsonResp)
	if err != nil {
		return nil, err
	}
	err = ConvertToK8sType(jsonResp, resp)
	resp.JSONResp = jsonResp
	return resp, err
}

func (c *SteveClient) Delete(container *SteveAPIObject) error {
	return c.apiClient.Ops.DoResourceDelete(c.steveType, &container.Resource)
}

func (c *NamespacedSteveClient) Create(container any) (*SteveAPIObject, error) {
	resp := &SteveAPIObject{}
	var jsonResp map[string]any
	url, err := c.apiClient.Ops.GetCollectionURL(c.steveType, "POST")
	if err != nil {
		return nil, err
	}
	if c.namespace != "" {
		url += "/" + c.namespace
	}
	err = c.apiClient.Ops.DoModify("POST", url, container, &jsonResp)
	if err != nil {
		return nil, err
	}
	err = ConvertToK8sType(jsonResp, resp)
	resp.JSONResp = jsonResp
	return resp, err
}

func (c *NamespacedSteveClient) Update(existing *SteveAPIObject, updates any) (*SteveAPIObject, error) {
	return c.SteveClient.Update(existing, updates)
}

func (c *NamespacedSteveClient) Replace(obj *SteveAPIObject) (*SteveAPIObject, error) {
	return c.SteveClient.Replace(obj)
}

func (c *NamespacedSteveClient) List(query url.Values) (*SteveCollection, error) {
	resp := &SteveCollection{}
	var jsonResp map[string]any
	url, err := c.apiClient.Ops.GetCollectionURL(c.steveType, "GET")
	if err != nil {
		return nil, err
	}
	if c.namespace != "" {
		url += "/" + c.namespace
	}
	if len(query) > 0 {
		url += "?" + query.Encode()
	}
	err = c.apiClient.Ops.DoGet(url, nil, &jsonResp)
	if err != nil {
		return nil, err
	}

	err = ConvertToK8sType(jsonResp, resp)
	if err != nil {
		return nil, err
	}

	steveList := jsonResp["data"]
	for index, item := range steveList.([]any) {
		resp.Data[index].JSONResp = item.(map[string]any)
	}
	return resp, err
}

func (c *NamespacedSteveClient) ListAll(params url.Values) (*SteveCollection, error) {
	resp, err := c.List(params)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (c *NamespacedSteveClient) ByID(id string) (*SteveAPIObject, error) {
	return c.SteveClient.ByID(id)
}

func (c *NamespacedSteveClient) Delete(container *SteveAPIObject) error {
	return c.SteveClient.Delete(container)
}

// ConvertToK8sType is helper function that coverts the generic Spec, Status, JSONResp fields of a
// SteveAPIObject to its native kubernetes type
// e.g. converting a SteveAPIObject spec to a NamespaceSpec
//
//	namespaceSpec := &coreV1.NamespaceSpec{}
//	err := namespaces.ConvertToK8sType(createdNamespace.Spec, namespaceSpec)
//	require.NoError(p.T(), err)
//
//	namespaceSpec.Finalizers
func ConvertToK8sType(steveResp any, kubernetesObject any) error {
	jsonbody, err := json.Marshal(steveResp)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonbody, kubernetesObject); err != nil {
		return err
	}

	return nil
}
