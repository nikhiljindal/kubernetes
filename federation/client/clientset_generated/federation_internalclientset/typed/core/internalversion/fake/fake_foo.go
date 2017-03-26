/*
Copyright 2017 The Kubernetes Authors.

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

package fake

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	api "k8s.io/kubernetes/pkg/api"
)

// FakeFoos implements FooInterface
type FakeFoos struct {
	Fake *FakeCore
	ns   string
}

var foosResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "foos"}

func (c *FakeFoos) Create(foo *api.Foo) (result *api.Foo, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(foosResource, c.ns, foo), &api.Foo{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Foo), err
}

func (c *FakeFoos) Update(foo *api.Foo) (result *api.Foo, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(foosResource, c.ns, foo), &api.Foo{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Foo), err
}

func (c *FakeFoos) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(foosResource, c.ns, name), &api.Foo{})

	return err
}

func (c *FakeFoos) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(foosResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.FooList{})
	return err
}

func (c *FakeFoos) Get(name string, options v1.GetOptions) (result *api.Foo, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(foosResource, c.ns, name), &api.Foo{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Foo), err
}

func (c *FakeFoos) List(opts v1.ListOptions) (result *api.FooList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(foosResource, c.ns, opts), &api.FooList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.FooList{}
	for _, item := range obj.(*api.FooList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested foos.
func (c *FakeFoos) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(foosResource, c.ns, opts))

}

// Patch applies the patch and returns the patched foo.
func (c *FakeFoos) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.Foo, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(foosResource, c.ns, name, data, subresources...), &api.Foo{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.Foo), err
}
