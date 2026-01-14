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

// DiscoveryMetadata represents metadata (labels and annotations) that should be
// applied to the Agent's DiscoveryNode. This matches the format expected by the
// Cryostat Agent's DiscoveryFileReader.
type DiscoveryMetadata struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// DiscoveryNode represents a node in the discovery tree hierarchy.
// This structure matches Cryostat's DiscoveryNode data model exactly to ensure
// compatibility with the Agent's discovery file reader and Cryostat's discovery system.
type DiscoveryNode struct {
	Name     string            `json:"name"`
	NodeType string            `json:"nodeType"`
	Labels   map[string]string `json:"labels"`
	Children []DiscoveryNode   `json:"children"`
}

// KubeNodeType represents Kubernetes resource types in the discovery hierarchy.
// These match Cryostat's KubeDiscoveryNodeType enum values exactly.
// Reference: https://github.com/cryostatio/cryostat/blob/main/src/main/java/io/cryostat/discovery/KubeDiscoveryNodeType.java
type KubeNodeType string

const (
	// Cryostat's exact enum values for Kubernetes resources
	KubeNodeTypeNamespace             KubeNodeType = "Namespace"             // Kubernetes Namespace
	KubeNodeTypeDeployment            KubeNodeType = "Deployment"            // Deployment
	KubeNodeTypeReplicaSet            KubeNodeType = "ReplicaSet"            // ReplicaSet
	KubeNodeTypeStatefulSet           KubeNodeType = "StatefulSet"           // StatefulSet
	KubeNodeTypeDaemonSet             KubeNodeType = "DaemonSet"             // DaemonSet
	KubeNodeTypeReplicationController KubeNodeType = "ReplicationController" // ReplicationController
	KubeNodeTypeJvmPod                KubeNodeType = "JVM_POD"               // Pod with JVM
	KubeNodeTypeDeploymentConfig      KubeNodeType = "DeploymentConfig"      // OpenShift DeploymentConfig
)

// ExpectedOwnerKinds defines the priority order for selecting owner references
// when multiple owners exist. This matches Cryostat's logic of taking the first
// "expected" owner Kind from known NodeTypes.
var ExpectedOwnerKinds = []KubeNodeType{
	KubeNodeTypeDeployment,
	KubeNodeTypeStatefulSet,
	KubeNodeTypeDaemonSet,
	KubeNodeTypeReplicaSet,
	KubeNodeTypeReplicationController,
	KubeNodeTypeDeploymentConfig,
}
