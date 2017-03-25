/*
Copyright 2015 The Kubernetes Authors.

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

package foo

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	apistorage "k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
)

// strategy implements behavior for Foo objects
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Foo
// objects via the REST API.
var Strategy = strategy{api.Scheme, names.SimpleNameGenerator}

// Strategy should implement rest.RESTCreateStrategy
var _ rest.RESTCreateStrategy = Strategy

// Strategy should implement rest.RESTUpdateStrategy
var _ rest.RESTUpdateStrategy = Strategy

func (strategy) NamespaceScoped() bool {
	return true
}

func (strategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	_ = obj.(*api.Foo)
}

func (strategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	cfg := obj.(*api.Foo)

	return validation.ValidateFoo(cfg)
}

// Canonicalize normalizes the object after validation.
func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*api.Foo)
	_ = newObj.(*api.Foo)
}

func (strategy) AllowUnconditionalUpdate() bool {
	return true
}

func (strategy) ValidateUpdate(ctx genericapirequest.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldCfg, newCfg := oldObj.(*api.Foo), newObj.(*api.Foo)

	return validation.ValidateFooUpdate(newCfg, oldCfg)
}

// FooToSelectableFields returns a field set that represents the object for matching purposes.
func FooToSelectableFields(cfg *api.Foo) fields.Set {
	return generic.ObjectMetaFieldsSet(&cfg.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	cfg, ok := obj.(*api.Foo)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a Foo")
	}
	return labels.Set(cfg.ObjectMeta.Labels), FooToSelectableFields(cfg), nil
}

// MatchFoo returns a generic matcher for a given label and field selector.
func MatchFoo(label labels.Selector, field fields.Selector) apistorage.SelectionPredicate {
	return apistorage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}
