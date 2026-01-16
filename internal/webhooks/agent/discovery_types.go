// Copyright The Cryostat Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

// Discovery ConfigMap constants
const (
	DiscoveryConfigMapComponent = "cryostat-agent-discovery"
	DiscoveryConfigMapPrefix    = DiscoveryConfigMapComponent + "-"
	DiscoveryConfigMapManagedBy = "cryostat-operator"
)

// DiscoveryMetadata represents metadata (labels and annotations) that should be
// applied to the Agent's DiscoveryNode.
type DiscoveryMetadata struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// DiscoveryNode represents a node in the discovery tree hierarchy.
type DiscoveryNode struct {
	Name     string            `json:"name"`
	NodeType string            `json:"nodeType"`
	Labels   map[string]string `json:"labels"`
	Children []DiscoveryNode   `json:"children"`
}

// KubeNodeType represents Kubernetes resource types in the discovery hierarchy.
type KubeNodeType string

const (
	KubeNodeTypeNamespace             KubeNodeType = "Namespace"
	KubeNodeTypeStatefulSet           KubeNodeType = "StatefulSet"
	KubeNodeTypeDaemonSet             KubeNodeType = "DaemonSet"
	KubeNodeTypeDeployment            KubeNodeType = "Deployment"
	KubeNodeTypeReplicaSet            KubeNodeType = "ReplicaSet"
	KubeNodeTypeReplicationController KubeNodeType = "ReplicationController"
	KubeNodeTypePod                   KubeNodeType = "Pod"
	KubeNodeTypeDeploymentConfig      KubeNodeType = "DeploymentConfig"
	KubeNodeTypeEndpoint              KubeNodeType = "Endpoint"
	KubeNodeTypeEndpointSlice         KubeNodeType = "EndpointSlice"
)

var ExpectedOwnerKinds = []KubeNodeType{
	KubeNodeTypeDeployment,
	KubeNodeTypeStatefulSet,
	KubeNodeTypeDaemonSet,
	KubeNodeTypeReplicaSet,
	KubeNodeTypeReplicationController,
	KubeNodeTypeDeploymentConfig,
}
