/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package validation

import (
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/expapi"
	"k8s.io/kubernetes/pkg/util"
)

func TestValidateHorizontalPodAutoscaler(t *testing.T) {
	successCases := []expapi.HorizontalPodAutoscaler{
		{
			ObjectMeta: api.ObjectMeta{
				Name:      "myautoscaler",
				Namespace: api.NamespaceDefault,
			},
			Spec: expapi.HorizontalPodAutoscalerSpec{
				ScaleRef: &expapi.SubresourceReference{
					Subresource: "scale",
				},
				MinCount: 1,
				MaxCount: 5,
				Target:   expapi.ResourceConsumption{Resource: api.ResourceCPU, Quantity: resource.MustParse("0.8")},
			},
		},
	}
	for _, successCase := range successCases {
		if errs := ValidateHorizontalPodAutoscaler(&successCase); len(errs) != 0 {
			t.Errorf("expected success: %v", errs)
		}
	}

	errorCases := map[string]expapi.HorizontalPodAutoscaler{
		"must be non-negative": {
			ObjectMeta: api.ObjectMeta{
				Name:      "myautoscaler",
				Namespace: api.NamespaceDefault,
			},
			Spec: expapi.HorizontalPodAutoscalerSpec{
				ScaleRef: &expapi.SubresourceReference{
					Subresource: "scale",
				},
				MinCount: -1,
				MaxCount: 5,
				Target:   expapi.ResourceConsumption{Resource: api.ResourceCPU, Quantity: resource.MustParse("0.8")},
			},
		},
		"must be bigger or equal to minCount": {
			ObjectMeta: api.ObjectMeta{
				Name:      "myautoscaler",
				Namespace: api.NamespaceDefault,
			},
			Spec: expapi.HorizontalPodAutoscalerSpec{
				ScaleRef: &expapi.SubresourceReference{
					Subresource: "scale",
				},
				MinCount: 7,
				MaxCount: 5,
				Target:   expapi.ResourceConsumption{Resource: api.ResourceCPU, Quantity: resource.MustParse("0.8")},
			},
		},
		"invalid value": {
			ObjectMeta: api.ObjectMeta{
				Name:      "myautoscaler",
				Namespace: api.NamespaceDefault,
			},
			Spec: expapi.HorizontalPodAutoscalerSpec{
				ScaleRef: &expapi.SubresourceReference{
					Subresource: "scale",
				},
				MinCount: 1,
				MaxCount: 5,
				Target:   expapi.ResourceConsumption{Resource: api.ResourceCPU, Quantity: resource.MustParse("-0.8")},
			},
		},
		"resource not supported": {
			ObjectMeta: api.ObjectMeta{
				Name:      "myautoscaler",
				Namespace: api.NamespaceDefault,
			},
			Spec: expapi.HorizontalPodAutoscalerSpec{
				ScaleRef: &expapi.SubresourceReference{
					Subresource: "scale",
				},
				MinCount: 1,
				MaxCount: 5,
				Target:   expapi.ResourceConsumption{Resource: api.ResourceName("NotSupportedResource"), Quantity: resource.MustParse("0.8")},
			},
		},
		"required value": {
			ObjectMeta: api.ObjectMeta{
				Name:      "myautoscaler",
				Namespace: api.NamespaceDefault,
			},
			Spec: expapi.HorizontalPodAutoscalerSpec{
				MinCount: 1,
				MaxCount: 5,
				Target:   expapi.ResourceConsumption{Resource: api.ResourceCPU, Quantity: resource.MustParse("0.8")},
			},
		},
	}

	for k, v := range errorCases {
		errs := ValidateHorizontalPodAutoscaler(&v)
		if len(errs) == 0 {
			t.Errorf("expected failure for %s", k)
		} else if !strings.Contains(errs[0].Error(), k) {
			t.Errorf("unexpected error: %v, expected: %s", errs[0], k)
		}
	}
}

func validDeployment() *expapi.Deployment {
	return &expapi.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name:      "abc",
			Namespace: api.NamespaceDefault,
		},
		Spec: expapi.DeploymentSpec{
			Selector: map[string]string{
				"name": "abc",
			},
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Name:      "abc",
					Namespace: api.NamespaceDefault,
					Labels: map[string]string{
						"name": "abc",
					},
				},
				Spec: api.PodSpec{
					RestartPolicy: api.RestartPolicyAlways,
					DNSPolicy:     api.DNSDefault,
					Containers: []api.Container{
						{
							Name:            "nginx",
							Image:           "image",
							ImagePullPolicy: api.PullNever,
						},
					},
				},
			},
		},
	}
}

func TestValidateDeployment(t *testing.T) {
	successCases := []*expapi.Deployment{
		validDeployment(),
	}
	for _, successCase := range successCases {
		if errs := ValidateDeployment(successCase); len(errs) != 0 {
			t.Errorf("expected success: %v", errs)
		}
	}

	errorCases := map[string]*expapi.Deployment{}
	errorCases["metadata.name: required value"] = &expapi.Deployment{
		ObjectMeta: api.ObjectMeta{
			Namespace: api.NamespaceDefault,
		},
	}
	// selector should match the labels in pod template.
	invalidSelectorDeployment := validDeployment()
	invalidSelectorDeployment.Spec.Selector = map[string]string{
		"name": "def",
	}
	errorCases["selector does not match labels"] = invalidSelectorDeployment

	// RestartPolicy should be always.
	invalidRestartPolicyDeployment := validDeployment()
	invalidRestartPolicyDeployment.Spec.Template.Spec.RestartPolicy = api.RestartPolicyNever
	errorCases["unsupported value 'Never'"] = invalidRestartPolicyDeployment

	// invalid unique label key.
	invalidUniqueLabelDeployment := validDeployment()
	invalidUniqueLabel := "abc/def/ghi"
	invalidUniqueLabelDeployment.Spec.UniqueLabelKey = &invalidUniqueLabel
	errorCases["spec.uniqueLabel: invalid value"] = invalidUniqueLabelDeployment

	// rollingUpdate should be nil for recreate.
	invalidRecreateDeployment := validDeployment()
	invalidRecreateDeployment.Spec.Strategy = expapi.DeploymentStrategy{
		Type:          expapi.DeploymentRecreate,
		RollingUpdate: &expapi.RollingUpdateDeployment{},
	}
	errorCases["rollingUpdate should be nil when strategy type is Recreate"] = invalidRecreateDeployment

	// MaxSurge should be in the form of 20%.
	invalidMaxSurgeDeployment := validDeployment()
	invalidMaxSurgeDeployment.Spec.Strategy = expapi.DeploymentStrategy{
		Type: expapi.DeploymentRollingUpdate,
		RollingUpdate: &expapi.RollingUpdateDeployment{
			MaxSurge: util.NewIntOrStringFromString("20Percent"),
		},
	}
	errorCases["value should be int(5) or percentage(5%)"] = invalidMaxSurgeDeployment

	// MaxSurge and MaxIncrement cannot both be zero.
	invalidRollingUpdateDeployment := validDeployment()
	invalidRollingUpdateDeployment.Spec.Strategy = expapi.DeploymentStrategy{
		Type: expapi.DeploymentRollingUpdate,
		RollingUpdate: &expapi.RollingUpdateDeployment{
			MaxSurge:       util.NewIntOrStringFromString("0%"),
			MaxUnavailable: util.NewIntOrStringFromInt(0),
		},
	}
	errorCases["cannot be 0 when maxSurge is 0 as well"] = invalidRollingUpdateDeployment

	for k, v := range errorCases {
		errs := ValidateDeployment(v)
		if len(errs) == 0 {
			t.Errorf("expected failure for %s", k)
		} else if !strings.Contains(errs[0].Error(), k) {
			t.Errorf("unexpected error: %v, expected: %s", errs[0], k)
		}
	}
}
