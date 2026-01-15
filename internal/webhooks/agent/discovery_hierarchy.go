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
	discoveryv1 "k8s.io/api/discovery/v1"
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

	var labels map[string]string
	var err error

	switch nodeType {
	case KubeNodeTypeDeployment:
		obj := &appsv1.Deployment{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case KubeNodeTypeReplicaSet:
		obj := &appsv1.ReplicaSet{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case KubeNodeTypeStatefulSet:
		obj := &appsv1.StatefulSet{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case KubeNodeTypeDaemonSet:
		obj := &appsv1.DaemonSet{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case KubeNodeTypeReplicationController:
		obj := &corev1.ReplicationController{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case KubeNodeTypePod:
		obj := &corev1.Pod{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case KubeNodeTypeEndpoint:
		obj := &corev1.Endpoints{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	case KubeNodeTypeEndpointSlice:
		obj := &discoveryv1.EndpointSlice{}
		err = c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj)
		if err == nil {
			labels = obj.Labels
		}
	default:
		return nil, nil
	}

	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	node := &DiscoveryNode{
		Name:     name,
		NodeType: string(nodeType),
		Labels:   copyLabels(labels),
		Children: []DiscoveryNode{},
	}

	return node, nil
}

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

// buildDiscoveryHierarchy constructs the complete discovery hierarchy for a Pod.
// Returns a tree structure: Namespace → [Deployment/StatefulSet/etc] → ... → Pod
// This matches Cryostat's discovery model where the root is always the Namespace.
func buildDiscoveryHierarchy(ctx context.Context, c client.Client, pod *corev1.Pod) (*DiscoveryNode, error) {
	// Build chain from Pod up to top-level owner
	chain := []*DiscoveryNode{}

	// Start with the Pod itself
	podNode := &DiscoveryNode{
		Name:     pod.Name,
		NodeType: string(KubeNodeTypePod),
		Labels:   copyLabels(pod.Labels),
		Children: []DiscoveryNode{}, // Empty array, not nil
	}
	chain = append(chain, podNode)

	// Chase owner references up the chain
	currentObj := metav1.Object(pod)
	for {
		ownerNode, err := getOwnerNode(ctx, c, currentObj, pod.Namespace)
		if err != nil {
			return nil, err
		}
		if ownerNode == nil {
			// No more owners - end of chain
			break
		}

		// Add owner to chain
		chain = append(chain, ownerNode)

		// Continue from this owner
		// We need to fetch the actual object to get its owner references
		var nextObj metav1.Object
		switch ownerNode.NodeType {
		case string(KubeNodeTypeDeployment):
			deployment := &appsv1.Deployment{}
			err := c.Get(ctx, types.NamespacedName{Name: ownerNode.Name, Namespace: pod.Namespace}, deployment)
			if err != nil {
				if errors.IsNotFound(err) {
					// Owner was deleted - stop here
					break
				}
				return nil, err
			}
			nextObj = deployment
		case string(KubeNodeTypeStatefulSet):
			statefulSet := &appsv1.StatefulSet{}
			err := c.Get(ctx, types.NamespacedName{Name: ownerNode.Name, Namespace: pod.Namespace}, statefulSet)
			if err != nil {
				if errors.IsNotFound(err) {
					break
				}
				return nil, err
			}
			nextObj = statefulSet
		case string(KubeNodeTypeDaemonSet):
			daemonSet := &appsv1.DaemonSet{}
			err := c.Get(ctx, types.NamespacedName{Name: ownerNode.Name, Namespace: pod.Namespace}, daemonSet)
			if err != nil {
				if errors.IsNotFound(err) {
					break
				}
				return nil, err
			}
			nextObj = daemonSet
		case string(KubeNodeTypeReplicaSet):
			replicaSet := &appsv1.ReplicaSet{}
			err := c.Get(ctx, types.NamespacedName{Name: ownerNode.Name, Namespace: pod.Namespace}, replicaSet)
			if err != nil {
				if errors.IsNotFound(err) {
					break
				}
				return nil, err
			}
			nextObj = replicaSet
		case string(KubeNodeTypeReplicationController):
			replicationController := &corev1.ReplicationController{}
			err := c.Get(ctx, types.NamespacedName{Name: ownerNode.Name, Namespace: pod.Namespace}, replicationController)
			if err != nil {
				if errors.IsNotFound(err) {
					break
				}
				return nil, err
			}
			nextObj = replicationController
		case string(KubeNodeTypeEndpoint):
			endpoint := &corev1.Endpoints{}
			err := c.Get(ctx, types.NamespacedName{Name: ownerNode.Name, Namespace: pod.Namespace}, endpoint)
			if err != nil {
				if errors.IsNotFound(err) {
					break
				}
				return nil, err
			}
			nextObj = endpoint
		case string(KubeNodeTypeEndpointSlice):
			endpointSlice := &discoveryv1.EndpointSlice{}
			err := c.Get(ctx, types.NamespacedName{Name: ownerNode.Name, Namespace: pod.Namespace}, endpointSlice)
			if err != nil {
				if errors.IsNotFound(err) {
					break
				}
				return nil, err
			}
			nextObj = endpointSlice
		default:
			// Unknown type - stop here
			break
		}

		if nextObj == nil {
			break
		}
		currentObj = nextObj
	}

	// Fetch Namespace to get its labels
	namespace := &corev1.Namespace{}
	err := c.Get(ctx, types.NamespacedName{Name: pod.Namespace}, namespace)
	if err != nil {
		return nil, err
	}

	// Create Namespace node as root
	namespaceNode := &DiscoveryNode{
		Name:     namespace.Name,
		NodeType: string(KubeNodeTypeNamespace),
		Labels:   copyLabels(namespace.Labels),
		Children: []DiscoveryNode{},
	}

	// Build hierarchy from top (Namespace) down to Pod
	// Chain is currently: [Pod, ReplicaSet, Deployment, ...]
	// We need to reverse it and nest: Namespace → Deployment → ReplicaSet → Pod
	currentNode := namespaceNode
	for i := len(chain) - 1; i >= 0; i-- {
		childNode := chain[i]
		// Initialize Children array if needed
		if childNode.Children == nil {
			childNode.Children = []DiscoveryNode{}
		}
		currentNode.Children = append(currentNode.Children, *childNode)
		// Move to the child for next iteration
		if len(currentNode.Children) > 0 {
			currentNode = &currentNode.Children[len(currentNode.Children)-1]
		}
	}

	return namespaceNode, nil
}
