package dynamic

import (
	"context"

	"k8s.io/client-go/rest"

	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Client is a struct that embedds the dynamic.Interface(dynamic client) and has Session as an attribute
// The session.Session attributes is passed all way down to the ResourceClient to keep track of the resources created by the dynamic client
type Client struct {
	dynamic.Interface
	ts *session.Session
}

// NewForConfig creates a new dynamic client or returns an error.
func NewForConfig(ts *session.Session, inConfig *rest.Config) (dynamic.Interface, error) {
	logrus.Infof("Dynamic Client Host:%s", inConfig.Host)

	dynamicClient, err := dynamic.NewForConfig(inConfig)
	if err != nil {
		return nil, err
	}

	return &Client{
		Interface: dynamicClient,
		ts:        ts,
	}, nil
}

// Resource takes a schema.GroupVersionResource parameter to set the appropriate resource interface e.g.
//
//	 schema.GroupVersionResource {
//		  Group:    "management.cattle.io",
//		  Version:  "v3",
//		  Resource: "users",
//	 }
func (d *Client) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &NamespaceableResourceClient{
		NamespaceableResourceInterface: d.Interface.Resource(resource),
		ts:                             d.ts,
	}
}

// NamespaceableResourceClient is a struct that has dynamic.NamespaceableResourceInterface embedded, and has session.Session as an attribute.
// This is inorder to overwrite dynamic.NamespaceableResourceInterface's Namespace function.
type NamespaceableResourceClient struct {
	dynamic.NamespaceableResourceInterface
	ts *session.Session
}

// Namespace returns a dynamic.ResourceInterface that is embedded in ResourceClient, so ultimately its Create is overwritten.
func (d *NamespaceableResourceClient) Namespace(s string) dynamic.ResourceInterface {
	return &ResourceClient{
		ResourceInterface: d.NamespaceableResourceInterface.Namespace(s),
		ts:                d.ts,
	}
}

// ResourceClient has dynamic.ResourceInterface embedded so dynamic.ResourceInterface's Create can be overwritten.
type ResourceClient struct {
	dynamic.ResourceInterface
	ts *session.Session
}

// Create is dynamic.ResourceInterface's Create function, that is being overwritten to register its delete function to the session.Session
// that is being reference.
func (c *ResourceClient) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	unstructuredObj, err := c.ResourceInterface.Create(ctx, obj, opts, subresources...)
	if err != nil {
		return nil, err
	}

	c.ts.RegisterCleanupFunc(func() error {
		err := c.Delete(context.TODO(), unstructuredObj.GetName(), metav1.DeleteOptions{}, subresources...)
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	})

	return unstructuredObj, err
}
