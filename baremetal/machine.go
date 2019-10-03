/*
Copyright 2019 The Kubernetes Authors.

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

package baremetal

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
)

// Machine implement a service for managing the docker containers hosting a kubernetes nodes.
type Machine struct {
	log       logr.Logger
	cluster   string
	machine   string
	image     string
}

// NewMachine returns a new Machine service for the given Cluster/DockerCluster pair.
func NewMachine(cluster string, machine string, image *infrav1.Image, logger logr.Logger) (*Machine, error) {
	if cluster == "" {
		return nil, errors.New("cluster is required when creating a docker.Machine")
	}
	if machine == "" {
		return nil, errors.New("machine is required when creating a docker.Machine")
	}
	if logger == nil {
		return nil, errors.New("logger is required when creating a docker.Machine")
	}

	return &Machine{
		cluster:   cluster,
		machine:   machine,
		image:     image.URL,
		log:       logger,
	}, nil
}

// ContainerName return the name of the container for this machine
func (m *Machine) ContainerName() string {
	return machineContainerName(m.cluster, m.machine)
}

// ProviderID return the provider identifier for this machine
func (m *Machine) ProviderID() string {
	return fmt.Sprintf("docker:////%s", m.ContainerName())
}

func machineContainerName(cluster, machine string) string {
	return fmt.Sprintf("%s-%s", cluster, machine)
}

// Create creates a docker container hosting a Kubernetes node.
func (m *Machine) Create(role string, version *string) error {
	// Create if not exists.
	return nil
}

// ExecBootstrap runs bootstrap on a node, this is generally `kubeadm <init|join>`
func (m *Machine) ExecBootstrap(data string) error {
	return nil
}

// SetNodeProviderID sets the docker provider ID for the kubernetes node
func (m *Machine) SetNodeProviderID() error {
	return nil
}

// KubeadmReset will run `kubeadm reset` on the machine.
func (m *Machine) KubeadmReset() error {
	return nil
}

// Delete deletes a docker container hosting a Kubernetes node.
func (m *Machine) Delete() error {
	return nil
}
