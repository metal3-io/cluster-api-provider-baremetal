/*
Copyright 2018 The Kubernetes Authors.

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
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// LoadBalancer manages the load balancer for a specific docker cluster.
type LoadBalancer struct {
	log       logr.Logger
	name      string
}

// NewLoadBalancer returns a new helper for managing a docker loadbalancer with a given name.
func NewLoadBalancer(name string, logger logr.Logger) (*LoadBalancer, error) {
	if name == "" {
		return nil, errors.New("name is required when creating a docker.LoadBalancer")
	}
	if logger == nil {
		return nil, errors.New("logger is required when creating a docker.LoadBalancer")
	}

	return &LoadBalancer{
		name:      name,
		log:       logger,
	}, nil
}

// Create creates a docker container hosting a load balancer for the cluster.
func (s *LoadBalancer) Create() error {
	// Create if not exists.
	return nil
}

// UpdateConfiguration updates the external load balancer configuration with new control plane nodes.
func (s *LoadBalancer) UpdateConfiguration() error {
	return nil
}

// IP returns the load balancer IP address
func (s *LoadBalancer) IP() (string, error) {
	return "", nil
}

// Delete the docker containers hosting a loadbalancer for the cluster.
func (s *LoadBalancer) Delete() error {
	return nil
}
