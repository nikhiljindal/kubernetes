/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package deployment

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/expapi"
	"k8s.io/kubernetes/pkg/expapi/validation"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	errs "k8s.io/kubernetes/pkg/util/fielderrors"
)

// deploymentStrategy implements behavior for Deployments.
type deploymentStrategy struct {
	runtime.ObjectTyper
	api.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Deployment
// objects via the REST API.
var Strategy = deploymentStrategy{api.Scheme, api.SimpleNameGenerator}

// NamespaceScoped is true for deployment.
func (deploymentStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (deploymentStrategy) PrepareForCreate(obj runtime.Object) {
	_ = obj.(*expapi.Deployment)
}

// Validate validates a new deployment.
func (deploymentStrategy) Validate(ctx api.Context, obj runtime.Object) errs.ValidationErrorList {
	deployment := obj.(*expapi.Deployment)
	return validation.ValidateDeployment(deployment)
}

// AllowCreateOnUpdate is false for deployments.
func (deploymentStrategy) AllowCreateOnUpdate() bool {
	return false
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (deploymentStrategy) PrepareForUpdate(obj, old runtime.Object) {
	_ = obj.(*expapi.Deployment)
}

// ValidateUpdate is the default update validation for an end user.
func (deploymentStrategy) ValidateUpdate(ctx api.Context, obj, old runtime.Object) errs.ValidationErrorList {
	return validation.ValidateDeploymentUpdate(obj.(*expapi.Deployment), old.(*expapi.Deployment))
}

func (deploymentStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// Matchdeployment returns a generic matcher for a given label and field selector.
func MatchDeployment(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		deployment, ok := obj.(*expapi.Deployment)
		if !ok {
			return false, fmt.Errorf("not a deployment")
		}
		return label.Matches(labels.Set(deployment.Labels)), nil
	})
}
