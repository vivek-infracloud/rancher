/*
Copyright 2023 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package v3

import (
	"context"
	"time"

	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/generic"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type RkeK8sSystemImageHandler func(string, *v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error)

type RkeK8sSystemImageController interface {
	generic.ControllerMeta
	RkeK8sSystemImageClient

	OnChange(ctx context.Context, name string, sync RkeK8sSystemImageHandler)
	OnRemove(ctx context.Context, name string, sync RkeK8sSystemImageHandler)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, duration time.Duration)

	Cache() RkeK8sSystemImageCache
}

type RkeK8sSystemImageClient interface {
	Create(*v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error)
	Update(*v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error)

	Delete(namespace, name string, options *metav1.DeleteOptions) error
	Get(namespace, name string, options metav1.GetOptions) (*v3.RkeK8sSystemImage, error)
	List(namespace string, opts metav1.ListOptions) (*v3.RkeK8sSystemImageList, error)
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v3.RkeK8sSystemImage, err error)
}

type RkeK8sSystemImageCache interface {
	Get(namespace, name string) (*v3.RkeK8sSystemImage, error)
	List(namespace string, selector labels.Selector) ([]*v3.RkeK8sSystemImage, error)

	AddIndexer(indexName string, indexer RkeK8sSystemImageIndexer)
	GetByIndex(indexName, key string) ([]*v3.RkeK8sSystemImage, error)
}

type RkeK8sSystemImageIndexer func(obj *v3.RkeK8sSystemImage) ([]string, error)

type rkeK8sSystemImageController struct {
	controller    controller.SharedController
	client        *client.Client
	gvk           schema.GroupVersionKind
	groupResource schema.GroupResource
}

func NewRkeK8sSystemImageController(gvk schema.GroupVersionKind, resource string, namespaced bool, controller controller.SharedControllerFactory) RkeK8sSystemImageController {
	c := controller.ForResourceKind(gvk.GroupVersion().WithResource(resource), gvk.Kind, namespaced)
	return &rkeK8sSystemImageController{
		controller: c,
		client:     c.Client(),
		gvk:        gvk,
		groupResource: schema.GroupResource{
			Group:    gvk.Group,
			Resource: resource,
		},
	}
}

func FromRkeK8sSystemImageHandlerToHandler(sync RkeK8sSystemImageHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v3.RkeK8sSystemImage
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v3.RkeK8sSystemImage))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *rkeK8sSystemImageController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v3.RkeK8sSystemImage))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateRkeK8sSystemImageDeepCopyOnChange(client RkeK8sSystemImageClient, obj *v3.RkeK8sSystemImage, handler func(obj *v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error)) (*v3.RkeK8sSystemImage, error) {
	if obj == nil {
		return obj, nil
	}

	copyObj := obj.DeepCopy()
	newObj, err := handler(copyObj)
	if newObj != nil {
		copyObj = newObj
	}
	if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
		return client.Update(copyObj)
	}

	return copyObj, err
}

func (c *rkeK8sSystemImageController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}

func (c *rkeK8sSystemImageController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), handler))
}

func (c *rkeK8sSystemImageController) OnChange(ctx context.Context, name string, sync RkeK8sSystemImageHandler) {
	c.AddGenericHandler(ctx, name, FromRkeK8sSystemImageHandlerToHandler(sync))
}

func (c *rkeK8sSystemImageController) OnRemove(ctx context.Context, name string, sync RkeK8sSystemImageHandler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), FromRkeK8sSystemImageHandlerToHandler(sync)))
}

func (c *rkeK8sSystemImageController) Enqueue(namespace, name string) {
	c.controller.Enqueue(namespace, name)
}

func (c *rkeK8sSystemImageController) EnqueueAfter(namespace, name string, duration time.Duration) {
	c.controller.EnqueueAfter(namespace, name, duration)
}

func (c *rkeK8sSystemImageController) Informer() cache.SharedIndexInformer {
	return c.controller.Informer()
}

func (c *rkeK8sSystemImageController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *rkeK8sSystemImageController) Cache() RkeK8sSystemImageCache {
	return &rkeK8sSystemImageCache{
		indexer:  c.Informer().GetIndexer(),
		resource: c.groupResource,
	}
}

func (c *rkeK8sSystemImageController) Create(obj *v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error) {
	result := &v3.RkeK8sSystemImage{}
	return result, c.client.Create(context.TODO(), obj.Namespace, obj, result, metav1.CreateOptions{})
}

func (c *rkeK8sSystemImageController) Update(obj *v3.RkeK8sSystemImage) (*v3.RkeK8sSystemImage, error) {
	result := &v3.RkeK8sSystemImage{}
	return result, c.client.Update(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *rkeK8sSystemImageController) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.client.Delete(context.TODO(), namespace, name, *options)
}

func (c *rkeK8sSystemImageController) Get(namespace, name string, options metav1.GetOptions) (*v3.RkeK8sSystemImage, error) {
	result := &v3.RkeK8sSystemImage{}
	return result, c.client.Get(context.TODO(), namespace, name, result, options)
}

func (c *rkeK8sSystemImageController) List(namespace string, opts metav1.ListOptions) (*v3.RkeK8sSystemImageList, error) {
	result := &v3.RkeK8sSystemImageList{}
	return result, c.client.List(context.TODO(), namespace, result, opts)
}

func (c *rkeK8sSystemImageController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.Watch(context.TODO(), namespace, opts)
}

func (c *rkeK8sSystemImageController) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (*v3.RkeK8sSystemImage, error) {
	result := &v3.RkeK8sSystemImage{}
	return result, c.client.Patch(context.TODO(), namespace, name, pt, data, result, metav1.PatchOptions{}, subresources...)
}

type rkeK8sSystemImageCache struct {
	indexer  cache.Indexer
	resource schema.GroupResource
}

func (c *rkeK8sSystemImageCache) Get(namespace, name string) (*v3.RkeK8sSystemImage, error) {
	obj, exists, err := c.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(c.resource, name)
	}
	return obj.(*v3.RkeK8sSystemImage), nil
}

func (c *rkeK8sSystemImageCache) List(namespace string, selector labels.Selector) (ret []*v3.RkeK8sSystemImage, err error) {

	err = cache.ListAllByNamespace(c.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v3.RkeK8sSystemImage))
	})

	return ret, err
}

func (c *rkeK8sSystemImageCache) AddIndexer(indexName string, indexer RkeK8sSystemImageIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v3.RkeK8sSystemImage))
		},
	}))
}

func (c *rkeK8sSystemImageCache) GetByIndex(indexName, key string) (result []*v3.RkeK8sSystemImage, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*v3.RkeK8sSystemImage, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*v3.RkeK8sSystemImage))
	}
	return result, nil
}
