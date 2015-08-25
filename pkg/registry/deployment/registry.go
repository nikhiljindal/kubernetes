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
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/expapi"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

// Registry is an interface implemented by things that know how to store deployment objects.
type Registry interface {
	// Returns a list of deployments having labels which match selector.
	ListDeployments(ctx api.Context, selector labels.Selector) (*expapi.DeploymentList, error)
	// Get a specific deployment.
	GetDeployment(ctx api.Context, deploymentID string) (*expapi.Deployment, error)
	// Create a deployment based on a specification.
	CreateDeployment(ctx api.Context, deployment *expapi.Deployment) error
	// Update an existing deployment.
	UpdateDeployment(ctx api.Context, deployment *expapi.Deployment) error
	// Delete an existing deployment.
	DeleteDeployment(ctx api.Context, deploymentID string) error
}

// storage puts strong typing around storage calls
type storage struct {
	rest.StandardStorage
}

// NewREST returns a new Registry interface for the given Storage. Any mismatched types will panic.
func NewRegistry(s rest.StandardStorage) Registry {
	return &storage{s}
}

func (s *storage) ListDeployments(ctx api.Context, label labels.Selector) (*expapi.DeploymentList, error) {
	obj, err := s.List(ctx, label, fields.Everything())
	if err != nil {
		return nil, err
	}
	return obj.(*expapi.DeploymentList), nil
}

func (s *storage) GetDeployment(ctx api.Context, deploymentID string) (*expapi.Deployment, error) {
	obj, err := s.Get(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	return obj.(*expapi.Deployment), nil
}

func (s *storage) CreateDeployment(ctx api.Context, deployment *expapi.Deployment) error {
	_, err := s.Create(ctx, deployment)
	return err
}

func (s *storage) UpdateDeployment(ctx api.Context, deployment *expapi.Deployment) error {
	_, _, err := s.Update(ctx, deployment)
	return err
}

func (s *storage) DeleteDeployment(ctx api.Context, deploymentID string) error {
	_, err := s.Delete(ctx, deploymentID, nil)
	return err
}
