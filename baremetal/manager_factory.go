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
	"github.com/go-logr/logr"
	capm3 "github.com/metal3-io/cluster-api-provider-baremetal/api/v1alpha3"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManagerFactoryInterface interface {
	NewClusterManager(cluster *capi.Cluster,
		metal3Cluster *capm3.Metal3Cluster,
		clusterLog logr.Logger) (ClusterManagerInterface, error)
	NewMachineManager(*capi.Cluster, *capm3.Metal3Cluster, *capi.Machine,
		*capm3.Metal3Machine, logr.Logger) (MachineManagerInterface, error)
}

// ManagerFactory only contains a client
type ManagerFactory struct {
	client client.Client
}

// NewManagerFactory returns a new factory.
func NewManagerFactory(client client.Client) ManagerFactory {
	return ManagerFactory{client: client}
}

// NewClusterManager creates a new ClusterManager
func (f ManagerFactory) NewClusterManager(cluster *capi.Cluster, capm3Cluster *capm3.Metal3Cluster, clusterLog logr.Logger) (ClusterManagerInterface, error) {
	return NewClusterManager(f.client, cluster, capm3Cluster, clusterLog)
}

// NewMachineManager creates a new MachineManager
func (f ManagerFactory) NewMachineManager(capiCluster *capi.Cluster,
	capm3Cluster *capm3.Metal3Cluster,
	capiMachine *capi.Machine, capm3Machine *capm3.Metal3Machine,
	machineLog logr.Logger) (MachineManagerInterface, error) {
	return NewMachineManager(f.client, capiCluster, capm3Cluster, capiMachine,
		capm3Machine, machineLog)
}
