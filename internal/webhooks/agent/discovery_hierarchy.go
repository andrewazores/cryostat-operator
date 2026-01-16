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
	"fmt"
	"strings"

	"github.com/cryostatio/cryostat-operator/internal/controllers/common/resource_definitions"
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
// https://github.com/cryostatio/cryostat/blob/4adcb762b45a37c741a7848c735f7c5b80bd2256/src/main/java/io/cryostat/discovery/KubeEndpointSlicesDiscovery.java#L523-L541
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

	if selectedOwner == nil {
		selectedOwner = &owners[0]
	}

	return queryForNode(ctx, c, namespace, selectedOwner.Name, selectedOwner.Kind)
}

func queryForNode(
	ctx context.Context,
	c client.Client,
	namespace string,
	name string,
	kind string,
) (*DiscoveryNode, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty for kind %s in namespace %s", kind, namespace)
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty for resource %s of kind %s", name, kind)
	}

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
		Labels:   resource_definitions.CreateMapCopy(labels),
		Children: []DiscoveryNode{},
	}

	return node, nil
}

// buildDiscoveryHierarchy constructs the complete discovery hierarchy for a Pod.
// Returns a tree structure: Namespace → [Deployment/StatefulSet/etc] → ... → Pod
func buildDiscoveryHierarchy(ctx context.Context, c client.Client, pod *corev1.Pod) (*DiscoveryNode, error) {
	podName := pod.Name
	if podName == "" {
		podName = pod.GenerateName
	}
	podName = strings.TrimRight(podName, "-_.")

	if podName == "" {
		return nil, fmt.Errorf("pod name and generateName are both empty")
	}
	if pod.Namespace == "" {
		return nil, fmt.Errorf("pod namespace cannot be empty for pod %s", podName)
	}

	chain := []*DiscoveryNode{}

	// Start with the Pod itself
	podNode := &DiscoveryNode{
		Name:     podName,
		NodeType: string(KubeNodeTypePod),
		Labels:   resource_definitions.CreateMapCopy(pod.Labels),
		Children: []DiscoveryNode{},
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
			break
		}

		chain = append(chain, ownerNode)

		var nextObj metav1.Object
		switch ownerNode.NodeType {
		case string(KubeNodeTypeDeployment):
			deployment := &appsv1.Deployment{}
			err := c.Get(ctx, types.NamespacedName{Name: ownerNode.Name, Namespace: pod.Namespace}, deployment)
			if err != nil {
				if errors.IsNotFound(err) {
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
			nextObj = nil
		}

		if nextObj == nil {
			break
		}
		currentObj = nextObj
	}

	namespace := &corev1.Namespace{}
	err := c.Get(ctx, types.NamespacedName{Name: pod.Namespace}, namespace)
	if err != nil {
		return nil, err
	}

	namespaceNode := &DiscoveryNode{
		Name:     namespace.Name,
		NodeType: string(KubeNodeTypeNamespace),
		Labels:   resource_definitions.CreateMapCopy(namespace.Labels),
		Children: []DiscoveryNode{},
	}

	currentNode := namespaceNode
	for i := len(chain) - 1; i >= 0; i-- {
		childNode := chain[i]
		if childNode.Children == nil {
			childNode.Children = []DiscoveryNode{}
		}
		currentNode.Children = append(currentNode.Children, *childNode)
		if len(currentNode.Children) > 0 {
			currentNode = &currentNode.Children[len(currentNode.Children)-1]
		}
	}

	return namespaceNode, nil
}
