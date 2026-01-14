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

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getOwnerNode recursively chases owner references to find the controlling resource.
// This matches Cryostat's getOwnerNode() algorithm:
// https://github.com/cryostatio/cryostat/blob/main/src/main/java/io/cryostat/discovery/KubeEndpointSlicesDiscovery.java#L523-L541
// 1. Take first "expected" owner Kind from ExpectedOwnerKinds
// 2. If none match, use first owner
// 3. Return nil if no owners (breaks the chain)
func getOwnerNode(
	ctx context.Context,
	c client.Client,
	obj metav1.Object,
	namespace string,
) (*DiscoveryNode, error) {
	owners := obj.GetOwnerReferences()

	// No owners - end of chain
	if len(owners) == 0 {
		return nil, nil
	}

	// Find first "expected" owner kind, or use first owner
	var selectedOwner *metav1.OwnerReference
	for _, expectedKind := range ExpectedOwnerKinds {
		for i := range owners {
			if owners[i].Kind == string(expectedKind) {
				selectedOwner = &owners[i]
				break
			}
		}
		if selectedOwner != nil {
			break
		}
	}

	// No expected kind found, use first owner
	if selectedOwner == nil {
		selectedOwner = &owners[0]
	}

	// Query for the owner resource
	return queryForNode(ctx, c, namespace, selectedOwner.Name, selectedOwner.Kind)
}

// queryForNode fetches a Kubernetes resource and creates a DiscoveryNode for it.
// Matches Cryostat's queryForNode() function:
// https://github.com/cryostatio/cryostat/blob/main/src/main/java/io/cryostat/discovery/KubeEndpointSlicesDiscovery.java#L543-L565
func queryForNode(
	ctx context.Context,
	c client.Client,
	namespace string,
	name string,
	kind string,
) (*DiscoveryNode, error) {
	nodeType := KubeNodeType(kind)

	// Fetch the Kubernetes resource and extract labels
	var labels map[string]string
	var err error

	switch nodeType {
	case NodeTypeDeployment:
		obj := &appsv1.Deployment{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case NodeTypeReplicaSet:
		obj := &appsv1.ReplicaSet{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case NodeTypeStatefulSet:
		obj := &appsv1.StatefulSet{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case NodeTypeDaemonSet:
		obj := &appsv1.DaemonSet{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case NodeTypeReplicationController:
		obj := &corev1.ReplicationController{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case NodeTypePod:
		obj := &corev1.Pod{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	default:
		// Unknown kind - return nil to break chain
		return nil, nil
	}

	if err != nil {
		if errors.IsNotFound(err) {
			// Owner was deleted - return nil to break chain
			return nil, nil
		}
		return nil, err
	}

	// Create DiscoveryNode matching Cryostat's structure
	node := &DiscoveryNode{
		Name:     name,
		NodeType: string(nodeType),
		Labels:   copyLabels(labels),
		Children: []DiscoveryNode{},
	}

	return node, nil
}

// copyLabels creates a deep copy of a label map.
func copyLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return make(map[string]string)
	}
	copy := make(map[string]string, len(labels))
	for k, v := range labels {
		copy[k] = v
	}
	return copy
}
