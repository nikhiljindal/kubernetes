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

package validation

import (
	"strconv"

	"k8s.io/kubernetes/pkg/api"
	apivalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/expapi"
	"k8s.io/kubernetes/pkg/util"
	errs "k8s.io/kubernetes/pkg/util/fielderrors"
)

// ValidateHorizontalPodAutoscaler can be used to check whether the given autoscaler name is valid.
// Prefix indicates this name will be used as part of generation, in which case trailing dashes are allowed.
func ValidateHorizontalPodAutoscalerName(name string, prefix bool) (bool, string) {
	// TODO: finally move it to pkg/api/validation and use nameIsDNSSubdomain function
	return apivalidation.ValidateReplicationControllerName(name, prefix)
}

// Validates that the given name can be used as a deployment name.
func ValidateDeploymentName(name string, prefix bool) (bool, string) {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

func validateHorizontalPodAutoscalerSpec(autoscaler expapi.HorizontalPodAutoscalerSpec) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if autoscaler.MinCount < 0 {
		allErrs = append(allErrs, errs.NewFieldInvalid("minCount", autoscaler.MinCount, `must be non-negative`))
	}
	if autoscaler.MaxCount < autoscaler.MinCount {
		allErrs = append(allErrs, errs.NewFieldInvalid("maxCount", autoscaler.MaxCount, `must be bigger or equal to minCount`))
	}
	if autoscaler.ScaleRef == nil {
		allErrs = append(allErrs, errs.NewFieldRequired("scaleRef"))
	}
	resource := autoscaler.Target.Resource.String()
	if resource != string(api.ResourceMemory) && resource != string(api.ResourceCPU) {
		allErrs = append(allErrs, errs.NewFieldInvalid("target.resource", resource, "resource not supported by autoscaler"))
	}
	quantity := autoscaler.Target.Quantity.Value()
	if quantity < 0 {
		allErrs = append(allErrs, errs.NewFieldInvalid("target.quantity", quantity, "must be non-negative"))
	}
	return allErrs
}

func ValidatePositiveIntOrPercent(intOrPercent util.IntOrString, fieldName string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if intOrPercent.Kind == util.IntstrString {
		if !util.IsValidPercent(intOrPercent.StrVal) {
			allErrs = append(allErrs, errs.NewFieldInvalid(fieldName, intOrPercent, "value should be int(5) or percentage(5%)"))
		}

	} else if intOrPercent.Kind == util.IntstrInt {
		allErrs = append(allErrs, apivalidation.ValidatePositiveField(int64(intOrPercent.IntVal), fieldName)...)
	}
	return allErrs
}

func getIntOrPercentValue(intOrStringValue util.IntOrString) int {
	if intOrStringValue.Kind == util.IntstrString {
		value, _ := strconv.Atoi(intOrStringValue.StrVal[:len(intOrStringValue.StrVal)-1])
		return value
	}
	return intOrStringValue.IntVal
}

func ValidateRollingUpdateDeployment(rollingUpdate *expapi.RollingUpdateDeployment, fieldName string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	allErrs = append(allErrs, ValidatePositiveIntOrPercent(rollingUpdate.MaxUnavailable, fieldName+"maxUnavailable")...)
	allErrs = append(allErrs, ValidatePositiveIntOrPercent(rollingUpdate.MaxSurge, fieldName+".maxSurge")...)
	// TODO: Validate that maxSurge and maxUnavailable cannot both be zero.
	if getIntOrPercentValue(rollingUpdate.MaxUnavailable) == 0 && getIntOrPercentValue(rollingUpdate.MaxSurge) == 0 {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		allErrs = append(allErrs, errs.NewFieldInvalid(fieldName+".maxUnavailable", rollingUpdate.MaxUnavailable, "cannot be 0 when maxSurge is 0 as well"))
	}
	allErrs = append(allErrs, apivalidation.ValidatePositiveField(int64(rollingUpdate.MinReadySeconds), fieldName+".minReadySeconds")...)
	return allErrs
}

func ValidateDeploymentStrategy(strategy *expapi.DeploymentStrategy, fieldName string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if strategy.RollingUpdate == nil {
		return allErrs
	}
	switch strategy.Type {
	case expapi.DeploymentRecreate:
		allErrs = append(allErrs, errs.NewFieldForbidden("rollingUpdate", "rollingUpdate should be nil when strategy type is "+expapi.DeploymentRecreate))
	case expapi.DeploymentRollingUpdate:
		allErrs = append(allErrs, ValidateRollingUpdateDeployment(strategy.RollingUpdate, "rollingUpdate")...)
	}
	return allErrs
}

// Validates given deployment spec.
func ValidateDeploymentSpec(spec *expapi.DeploymentSpec) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateNonEmptySelector(spec.Selector, "selector")...)
	allErrs = append(allErrs, apivalidation.ValidatePositiveField(int64(spec.Replicas), "replicas")...)
	allErrs = append(allErrs, apivalidation.ValidatePodTemplateSpecForRC(spec.Template, spec.Selector, spec.Replicas, "template")...)
	allErrs = append(allErrs, ValidateDeploymentStrategy(&spec.Strategy, "strategy")...)
	if spec.UniqueLabelKey != nil {
		allErrs = append(allErrs, apivalidation.ValidateLabelName(*spec.UniqueLabelKey, "uniqueLabel")...)
	}
	return allErrs
}

func ValidateHorizontalPodAutoscaler(autoscaler *expapi.HorizontalPodAutoscaler) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&autoscaler.ObjectMeta, true, ValidateHorizontalPodAutoscalerName).Prefix("metadata")...)
	allErrs = append(allErrs, validateHorizontalPodAutoscalerSpec(autoscaler.Spec)...)
	return allErrs
}

func ValidateHorizontalPodAutoscalerUpdate(newAutoscler, oldAutoscaler *expapi.HorizontalPodAutoscaler) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newAutoscler.ObjectMeta, &oldAutoscaler.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, validateHorizontalPodAutoscalerSpec(newAutoscler.Spec)...)
	return allErrs
}

func ValidateThirdPartyResourceUpdate(old, update *expapi.ThirdPartyResource) errs.ValidationErrorList {
	return ValidateThirdPartyResource(update)
}

func ValidateThirdPartyResource(obj *expapi.ThirdPartyResource) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(obj.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldInvalid("name", obj.Name, "name must be non-empty"))
	}
	versions := util.StringSet{}
	for ix := range obj.Versions {
		version := &obj.Versions[ix]
		if len(version.Name) == 0 {
			allErrs = append(allErrs, errs.NewFieldInvalid("name", version, "name can not be empty"))
		}
		if versions.Has(version.Name) {
			allErrs = append(allErrs, errs.NewFieldDuplicate("version", version))
		}
		versions.Insert(version.Name)
	}
	return allErrs
}

func ValidateDeploymentUpdate(old, update *expapi.Deployment) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&update.ObjectMeta, &old.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, validateDeploymentSpec(&update.Spec).Prefix("spec")...)
	return allErrs
}

func ValidateDeployment(obj *expapi.Deployment) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&obj.ObjectMeta, true, ValidateDeploymentName).Prefix("metadata")...)
	allErrs = append(allErrs, validateDeploymentSpec(&obj.Spec).Prefix("spec")...)
	return allErrs
}
