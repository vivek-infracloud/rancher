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
	v3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
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

type SourceCodeProviderHandler func(string, *v3.SourceCodeProvider) (*v3.SourceCodeProvider, error)

type SourceCodeProviderController interface {
	generic.ControllerMeta
	SourceCodeProviderClient

	OnChange(ctx context.Context, name string, sync SourceCodeProviderHandler)
	OnRemove(ctx context.Context, name string, sync SourceCodeProviderHandler)
	Enqueue(name string)
	EnqueueAfter(name string, duration time.Duration)

	Cache() SourceCodeProviderCache
}

type SourceCodeProviderClient interface {
	Create(*v3.SourceCodeProvider) (*v3.SourceCodeProvider, error)
	Update(*v3.SourceCodeProvider) (*v3.SourceCodeProvider, error)

	Delete(name string, options *metav1.DeleteOptions) error
	Get(name string, options metav1.GetOptions) (*v3.SourceCodeProvider, error)
	List(opts metav1.ListOptions) (*v3.SourceCodeProviderList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v3.SourceCodeProvider, err error)
}

type SourceCodeProviderCache interface {
	Get(name string) (*v3.SourceCodeProvider, error)
	List(selector labels.Selector) ([]*v3.SourceCodeProvider, error)

	AddIndexer(indexName string, indexer SourceCodeProviderIndexer)
	GetByIndex(indexName, key string) ([]*v3.SourceCodeProvider, error)
}

type SourceCodeProviderIndexer func(obj *v3.SourceCodeProvider) ([]string, error)

type sourceCodeProviderController struct {
	controller    controller.SharedController
	client        *client.Client
	gvk           schema.GroupVersionKind
	groupResource schema.GroupResource
}

func NewSourceCodeProviderController(gvk schema.GroupVersionKind, resource string, namespaced bool, controller controller.SharedControllerFactory) SourceCodeProviderController {
	c := controller.ForResourceKind(gvk.GroupVersion().WithResource(resource), gvk.Kind, namespaced)
	return &sourceCodeProviderController{
		controller: c,
		client:     c.Client(),
		gvk:        gvk,
		groupResource: schema.GroupResource{
			Group:    gvk.Group,
			Resource: resource,
		},
	}
}

func FromSourceCodeProviderHandlerToHandler(sync SourceCodeProviderHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v3.SourceCodeProvider
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v3.SourceCodeProvider))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *sourceCodeProviderController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v3.SourceCodeProvider))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateSourceCodeProviderDeepCopyOnChange(client SourceCodeProviderClient, obj *v3.SourceCodeProvider, handler func(obj *v3.SourceCodeProvider) (*v3.SourceCodeProvider, error)) (*v3.SourceCodeProvider, error) {
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

func (c *sourceCodeProviderController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}

func (c *sourceCodeProviderController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), handler))
}

func (c *sourceCodeProviderController) OnChange(ctx context.Context, name string, sync SourceCodeProviderHandler) {
	c.AddGenericHandler(ctx, name, FromSourceCodeProviderHandlerToHandler(sync))
}

func (c *sourceCodeProviderController) OnRemove(ctx context.Context, name string, sync SourceCodeProviderHandler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), FromSourceCodeProviderHandlerToHandler(sync)))
}

func (c *sourceCodeProviderController) Enqueue(name string) {
	c.controller.Enqueue("", name)
}

func (c *sourceCodeProviderController) EnqueueAfter(name string, duration time.Duration) {
	c.controller.EnqueueAfter("", name, duration)
}

func (c *sourceCodeProviderController) Informer() cache.SharedIndexInformer {
	return c.controller.Informer()
}

func (c *sourceCodeProviderController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *sourceCodeProviderController) Cache() SourceCodeProviderCache {
	return &sourceCodeProviderCache{
		indexer:  c.Informer().GetIndexer(),
		resource: c.groupResource,
	}
}

func (c *sourceCodeProviderController) Create(obj *v3.SourceCodeProvider) (*v3.SourceCodeProvider, error) {
	result := &v3.SourceCodeProvider{}
	return result, c.client.Create(context.TODO(), "", obj, result, metav1.CreateOptions{})
}

func (c *sourceCodeProviderController) Update(obj *v3.SourceCodeProvider) (*v3.SourceCodeProvider, error) {
	result := &v3.SourceCodeProvider{}
	return result, c.client.Update(context.TODO(), "", obj, result, metav1.UpdateOptions{})
}

func (c *sourceCodeProviderController) Delete(name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.client.Delete(context.TODO(), "", name, *options)
}

func (c *sourceCodeProviderController) Get(name string, options metav1.GetOptions) (*v3.SourceCodeProvider, error) {
	result := &v3.SourceCodeProvider{}
	return result, c.client.Get(context.TODO(), "", name, result, options)
}

func (c *sourceCodeProviderController) List(opts metav1.ListOptions) (*v3.SourceCodeProviderList, error) {
	result := &v3.SourceCodeProviderList{}
	return result, c.client.List(context.TODO(), "", result, opts)
}

func (c *sourceCodeProviderController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.Watch(context.TODO(), "", opts)
}

func (c *sourceCodeProviderController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*v3.SourceCodeProvider, error) {
	result := &v3.SourceCodeProvider{}
	return result, c.client.Patch(context.TODO(), "", name, pt, data, result, metav1.PatchOptions{}, subresources...)
}

type sourceCodeProviderCache struct {
	indexer  cache.Indexer
	resource schema.GroupResource
}

func (c *sourceCodeProviderCache) Get(name string) (*v3.SourceCodeProvider, error) {
	obj, exists, err := c.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(c.resource, name)
	}
	return obj.(*v3.SourceCodeProvider), nil
}

func (c *sourceCodeProviderCache) List(selector labels.Selector) (ret []*v3.SourceCodeProvider, err error) {

	err = cache.ListAll(c.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v3.SourceCodeProvider))
	})

	return ret, err
}

func (c *sourceCodeProviderCache) AddIndexer(indexName string, indexer SourceCodeProviderIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v3.SourceCodeProvider))
		},
	}))
}

func (c *sourceCodeProviderCache) GetByIndex(indexName, key string) (result []*v3.SourceCodeProvider, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*v3.SourceCodeProvider, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*v3.SourceCodeProvider))
	}
	return result, nil
}
