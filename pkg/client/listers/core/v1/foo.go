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

// This file was automatically generated by lister-gen

package v1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	api "k8s.io/kubernetes/pkg/api"
	v1 "k8s.io/kubernetes/pkg/api/v1"
)

// FooLister helps list Foos.
type FooLister interface {
	// List lists all Foos in the indexer.
	List(selector labels.Selector) (ret []*v1.Foo, err error)
	// Foos returns an object that can list and get Foos.
	Foos(namespace string) FooNamespaceLister
	FooListerExpansion
}

// fooLister implements the FooLister interface.
type fooLister struct {
	indexer cache.Indexer
}

// NewFooLister returns a new FooLister.
func NewFooLister(indexer cache.Indexer) FooLister {
	return &fooLister{indexer: indexer}
}

// List lists all Foos in the indexer.
func (s *fooLister) List(selector labels.Selector) (ret []*v1.Foo, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.Foo))
	})
	return ret, err
}

// Foos returns an object that can list and get Foos.
func (s *fooLister) Foos(namespace string) FooNamespaceLister {
	return fooNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// FooNamespaceLister helps list and get Foos.
type FooNamespaceLister interface {
	// List lists all Foos in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1.Foo, err error)
	// Get retrieves the Foo from the indexer for a given namespace and name.
	Get(name string) (*v1.Foo, error)
	FooNamespaceListerExpansion
}

// fooNamespaceLister implements the FooNamespaceLister
// interface.
type fooNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Foos in the indexer for a given namespace.
func (s fooNamespaceLister) List(selector labels.Selector) (ret []*v1.Foo, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.Foo))
	})
	return ret, err
}

// Get retrieves the Foo from the indexer for a given namespace and name.
func (s fooNamespaceLister) Get(name string) (*v1.Foo, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(api.Resource("foo"), name)
	}
	return obj.(*v1.Foo), nil
}
