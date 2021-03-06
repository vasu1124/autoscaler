/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/autoscaler project
https://github.com/kubernetes/autoscaler/blob/cluster-autoscaler-release-1.1/cluster-autoscaler/cloudprovider/aws/aws_cloud_provider.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

package mcm

import (
	"fmt"
	"strings"

	"github.com/gardener/autoscaler/cluster-autoscaler/cloudprovider"
	"github.com/gardener/autoscaler/cluster-autoscaler/config/dynamic"
	"github.com/gardener/autoscaler/cluster-autoscaler/utils/errors"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schedulercache "k8s.io/kubernetes/pkg/scheduler/cache"
)

const (
	// ProviderName is the cloud provider name for MCM
	ProviderName = "mcm"
)

// MCMCloudProvider implements the cloud provider interface for machine-controller-manager
// Reference: https://github.com/gardener/machine-controller-manager
type mcmCloudProvider struct {
	mcmManager         *McmManager
	machinedeployments []*MachineDeployment
	resourceLimiter    *cloudprovider.ResourceLimiter
}

// BuildMcmCloudProvider builds CloudProvider implementation for machine-controller-manager.
func BuildMcmCloudProvider(mcmManager *McmManager, resourceLimiter *cloudprovider.ResourceLimiter) (cloudprovider.CloudProvider, error) {
	if err := mcmManager.discoveryOpts.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to build an mcm cloud provider: %v", err)
	}
	if mcmManager.discoveryOpts.StaticDiscoverySpecified() {
		return buildStaticallyDiscoveringProvider(mcmManager, mcmManager.discoveryOpts.NodeGroupSpecs, resourceLimiter)
	}
	return nil, fmt.Errorf("Failed to build an mcm cloud provider: Either node group specs or node group auto discovery spec must be specified")
}

func buildStaticallyDiscoveringProvider(mcmManager *McmManager, specs []string, resourceLimiter *cloudprovider.ResourceLimiter) (*mcmCloudProvider, error) {
	mcm := &mcmCloudProvider{
		mcmManager:         mcmManager,
		machinedeployments: make([]*MachineDeployment, 0),
		resourceLimiter:    resourceLimiter,
	}
	for _, spec := range specs {
		if err := mcm.addNodeGroup(spec); err != nil {
			return nil, err
		}
	}
	return mcm, nil
}

// Cleanup stops the go routine that is handling the current view of the MachineDeployment in the form of a cache
func (mcm *mcmCloudProvider) Cleanup() error {
	mcm.mcmManager.Cleanup()
	return nil
}

// addNodeGroup adds node group defined in string spec. Format:
// minNodes:maxNodes:namespace.machineDeploymentName
func (mcm *mcmCloudProvider) addNodeGroup(spec string) error {
	machinedeployment, err := buildMachineDeploymentFromSpec(spec, mcm.mcmManager)
	if err != nil {
		return err
	}
	mcm.addMachineDeployment(machinedeployment)
	return nil
}

func (mcm *mcmCloudProvider) addMachineDeployment(machinedeployment *MachineDeployment) {
	mcm.machinedeployments = append(mcm.machinedeployments, machinedeployment)
	return
}

func (mcm *mcmCloudProvider) Name() string {
	return "machine-controller-manager"
}

// NodeGroups returns all node groups configured for this cloud provider.
func (mcm *mcmCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	result := make([]cloudprovider.NodeGroup, 0, len(mcm.machinedeployments))
	for _, machinedeployment := range mcm.machinedeployments {
		result = append(result, machinedeployment)
	}
	return result
}

// NodeGroupForNode returns the node group for the given node.
func (mcm *mcmCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	ref, err := ReferenceFromProviderID(mcm.mcmManager, node.Spec.ProviderID)
	if err != nil {
		return nil, err
	}

	return mcm.mcmManager.GetMachineDeploymentForMachine(ref)
}

// Pricing returns pricing model for this cloud provider or error if not available.
func (mcm *mcmCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider.
func (mcm *mcmCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return []string{}, nil
}

// NewNodeGroup builds a theoretical node group based on the node definition provided. The node group is not automatically
// created on the cloud provider side. The node group is not returned by NodeGroups() until it is created.
func (mcm *mcmCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string,
	taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.).
func (mcm *mcmCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return mcm.resourceLimiter, nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
// TODO: Implement this method to update the machinedeployments dynamically
func (mcm *mcmCloudProvider) Refresh() error {
	return nil
}

// Ref contains a reference to the name of the machine-deployment.
type Ref struct {
	Name      string
	Namespace string
}

// ReferenceFromProviderID extracts the Ref from providerId. It returns corresponding machine-name to providerid.
func ReferenceFromProviderID(m *McmManager, id string) (*Ref, error) {
	machines, err := m.machineclient.Machines(m.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Could not list machines due to error: %s", err)
	}

	var Name, Namespace string
	for _, machine := range machines.Items {
		machineID := strings.Split(machine.Spec.ProviderID, "/")
		nodeID := strings.Split(id, "/")
		if machineID[len(machineID)-1] == nodeID[len(nodeID)-1] {
			Name = machine.Name
			Namespace = machine.Namespace
			break
		}
	}
	if Name == "" {
		return nil, fmt.Errorf("Could not find any machine corresponds to node %+v", id)
	}
	return &Ref{
		Name:      Name,
		Namespace: Namespace,
	}, nil
}

// MachineDeployment implements NodeGroup interface.
type MachineDeployment struct {
	Ref

	mcmManager *McmManager

	minSize int
	maxSize int
}

// MaxSize returns maximum size of the node group.
func (machinedeployment *MachineDeployment) MaxSize() int {
	return machinedeployment.maxSize
}

// MinSize returns minimum size of the node group.
func (machinedeployment *MachineDeployment) MinSize() int {
	return machinedeployment.minSize
}

// TargetSize returns the current TARGET size of the node group. It is possible that the
// number is different from the number of nodes registered in Kubernetes.
func (machinedeployment *MachineDeployment) TargetSize() (int, error) {
	size, err := machinedeployment.mcmManager.GetMachineDeploymentSize(machinedeployment)
	return int(size), err
}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one.
// TODO: Implement this to check if machine-deployment really exists.
func (machinedeployment *MachineDeployment) Exist() bool {
	return true
}

// Create creates the node group on the cloud provider side.
func (machinedeployment *MachineDeployment) Create() (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrAlreadyExist
}

// Autoprovisioned returns true if the node group is autoprovisioned.
func (machinedeployment *MachineDeployment) Autoprovisioned() bool {
	return false
}

// Delete deletes the node group on the cloud provider side.
// This will be executed only for autoprovisioned node groups, once their size drops to 0.
func (machinedeployment *MachineDeployment) Delete() error {
	return cloudprovider.ErrNotImplemented
}

// IncreaseSize of the Machinedeployment.
func (machinedeployment *MachineDeployment) IncreaseSize(delta int) error {
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	size, err := machinedeployment.mcmManager.GetMachineDeploymentSize(machinedeployment)
	if err != nil {
		return err
	}
	if int(size)+delta > machinedeployment.MaxSize() {
		return fmt.Errorf("size increase too large - desired:%d max:%d", int(size)+delta, machinedeployment.MaxSize())
	}
	return machinedeployment.mcmManager.SetMachineDeploymentSize(machinedeployment, size+int64(delta))
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes if the size
// when there is an option to just decrease the target.
func (machinedeployment *MachineDeployment) DecreaseTargetSize(delta int) error {
	if delta >= 0 {
		return fmt.Errorf("size decrease size must be negative")
	}
	size, err := machinedeployment.mcmManager.GetMachineDeploymentSize(machinedeployment)
	if err != nil {
		return err
	}
	nodes, err := machinedeployment.mcmManager.GetMachineDeploymentNodes(machinedeployment)
	if err != nil {
		return err
	}

	if int(size)+delta < len(nodes) {
		return fmt.Errorf("attempt to delete existing nodes targetSize:%d delta:%d existingNodes: %s", size, delta, nodes)
	}

	return machinedeployment.mcmManager.SetMachineDeploymentSize(machinedeployment, size+int64(delta))
}

// Belongs returns true if the given node belongs to the NodeGroup.
// TODO: Implement this to iterate over machines under machinedeployment, and return true if node exists in list.
func (machinedeployment *MachineDeployment) Belongs(node *apiv1.Node) (bool, error) {
	ref, err := ReferenceFromProviderID(machinedeployment.mcmManager, node.Spec.ProviderID)
	if err != nil {
		return false, err
	}
	targetMd, err := machinedeployment.mcmManager.GetMachineDeploymentForMachine(ref)
	if err != nil {
		return false, err
	}
	if targetMd == nil {
		return false, fmt.Errorf("%s doesn't belong to a known MachinDeployment", node.Name)
	}
	if targetMd.Id() != machinedeployment.Id() {
		return false, nil
	}
	return true, nil
}

// DeleteNodes deletes the nodes from the group.
func (machinedeployment *MachineDeployment) DeleteNodes(nodes []*apiv1.Node) error {
	size, err := machinedeployment.mcmManager.GetMachineDeploymentSize(machinedeployment)
	if err != nil {
		return err
	}
	if int(size) <= machinedeployment.MinSize() {
		return fmt.Errorf("min size reached, nodes will not be deleted")
	}
	machines := make([]*Ref, 0, len(nodes))
	for _, node := range nodes {
		belongs, err := machinedeployment.Belongs(node)
		if err != nil {
			return err
		}
		ref, err := ReferenceFromProviderID(machinedeployment.mcmManager, node.Spec.ProviderID)
		if belongs != true {
			return fmt.Errorf("%s belongs to a different machinedeployment than %s", node.Name, machinedeployment.Id())
		}
		machines = append(machines, ref)
	}
	return machinedeployment.mcmManager.DeleteMachines(machines)
}

// Id returns machinedeployment id.
func (machinedeployment *MachineDeployment) Id() string {
	return machinedeployment.Name
}

// Debug returns a debug string for the Asg.
func (machinedeployment *MachineDeployment) Debug() string {
	return fmt.Sprintf("%s (%d:%d)", machinedeployment.Id(), machinedeployment.MinSize(), machinedeployment.MaxSize())
}

// Nodes returns a list of all nodes that belong to this node group.
func (machinedeployment *MachineDeployment) Nodes() ([]string, error) {
	return machinedeployment.mcmManager.GetMachineDeploymentNodes(machinedeployment)
}

// TemplateNodeInfo returns a node template for this node group.
func (machinedeployment *MachineDeployment) TemplateNodeInfo() (*schedulercache.NodeInfo, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func buildMachineDeploymentFromSpec(value string, mcmManager *McmManager) (*MachineDeployment, error) {
	spec, err := dynamic.SpecFromString(value, true)

	if err != nil {
		return nil, fmt.Errorf("failed to parse node group spec: %v", err)
	}
	s := strings.Split(spec.Name, ".")
	Namespace, Name := s[0], s[1]

	machinedeployment := buildMachineDeployment(mcmManager, spec.MinSize, spec.MaxSize, Namespace, Name)
	return machinedeployment, nil
}

func buildMachineDeployment(mcmManager *McmManager, minSize int, maxSize int, namespace string, name string) *MachineDeployment {
	return &MachineDeployment{
		mcmManager: mcmManager,
		minSize:    minSize,
		maxSize:    maxSize,
		Ref: Ref{
			Name:      name,
			Namespace: namespace,
		},
	}
}
