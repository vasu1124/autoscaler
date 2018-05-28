/*
Copyright 2017 The Gardener Authors.

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

// Package validation is used to validate all the machine CRD objects
package validation

import (
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
)

const nameFmt string = `[-a-z0-9]+`
const nameMaxLength int = 63

var nameRegexp = regexp.MustCompile("^" + nameFmt + "$")

// validateName is the validation function for object names.
func validateName(value string, prefix bool) []string {
	var errs []string
	if len(value) > nameMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(nameMaxLength))
	}
	if !nameRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(nameFmt, "name-40d-0983-1b89"))
	}

	return errs
}

// ValidateAWSMachineClass validates a AWSMachineClass and returns a list of errors.
func ValidateAWSMachineClass(AWSMachineClass *machine.AWSMachineClass) field.ErrorList {
	return internalValidateAWSMachineClass(AWSMachineClass)
}

func internalValidateAWSMachineClass(AWSMachineClass *machine.AWSMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&AWSMachineClass.ObjectMeta, true, /*namespace*/
		validateName,
		field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateAWSMachineClassSpec(&AWSMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateAWSMachineClassSpec(spec *machine.AWSMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.AMI {
		allErrs = append(allErrs, field.Required(fldPath.Child("ami"), "AMI is required"))
	}
	if "" == spec.Region {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "Region is required"))
	}
	if "" == spec.MachineType {
		allErrs = append(allErrs, field.Required(fldPath.Child("machineType"), "MachineType is required"))
	}
	if "" == spec.IAM.Name {
		allErrs = append(allErrs, field.Required(fldPath.Child("iam.name"), "IAM Name is required"))
	}
	if "" == spec.KeyName {
		allErrs = append(allErrs, field.Required(fldPath.Child("keyName"), "KeyName is required"))
	}

	allErrs = append(allErrs, validateBlockDevices(spec.BlockDevices, field.NewPath("spec.blockDevices"))...)
	allErrs = append(allErrs, validateNetworkInterfaces(spec.NetworkInterfaces, field.NewPath("spec.networkInterfaces"))...)
	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, field.NewPath("spec.secretRef"))...)
	allErrs = append(allErrs, validateAWSClassSpecTags(spec.Tags, field.NewPath("spec.tags"))...)

	return allErrs
}

func validateAWSClassSpecTags(tags map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	clusterName := ""
	nodeRole := ""

	for key := range tags {
		if strings.Contains(key, "kubernetes.io/cluster/") {
			clusterName = key
		} else if strings.Contains(key, "kubernetes.io/role/") {
			nodeRole = key
		}
	}

	if clusterName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io/cluster/"), "Tag required of the form kubernetes.io/cluster/****"))
	}
	if nodeRole == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io/role/"), "Tag required of the form kubernetes.io/role/****"))
	}

	return allErrs
}

func validateBlockDevices(blockDevices []machine.AWSBlockDeviceMappingSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(blockDevices) > 1 {
		allErrs = append(allErrs, field.Required(fldPath.Child(""), "Can only specify one (root) block device"))
	} else if len(blockDevices) == 1 {
		if blockDevices[0].Ebs.VolumeSize == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("ebs.volumeSize"), "Please mention a valid ebs volume size"))
		}
		if blockDevices[0].Ebs.VolumeType == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("ebs.volumeType"), "Please mention a valid ebs volume type"))
		}
	}
	return allErrs
}

func validateNetworkInterfaces(networkInterfaces []machine.AWSNetworkInterfaceSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(networkInterfaces) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child(""), "Mention at least one NetworkInterface"))
	} else {
		for i := range networkInterfaces {
			if "" == networkInterfaces[i].SubnetID {
				allErrs = append(allErrs, field.Required(fldPath.Child("subnetID"), "SubnetID is required"))
			}

			if 0 == len(networkInterfaces[i].SecurityGroupIDs) {
				allErrs = append(allErrs, field.Required(fldPath.Child("securityGroupIDs"), "Mention at least one securityGroupID"))
			} else {
				for j := range networkInterfaces[i].SecurityGroupIDs {
					if "" == networkInterfaces[i].SecurityGroupIDs[j] {
						output := strings.Join([]string{"securityGroupIDs cannot be blank for networkInterface:", strconv.Itoa(i), " securityGroupID:", strconv.Itoa(j)}, "")
						allErrs = append(allErrs, field.Required(fldPath.Child("securityGroupIDs"), output))
					}
				}
			}
		}
	}
	return allErrs
}

func validateSecretRef(reference *corev1.SecretReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if "" == reference.Name {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	}

	if "" == reference.Namespace {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "namespace is required"))
	}
	return allErrs
}