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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createDiscoveryConfigMap creates a ConfigMap containing hierarchy.json and metadata.json
// for the Cryostat Agent to read. The ConfigMap is created without an owner reference since
// the Pod doesn't exist yet during webhook mutation. A controller should add the owner reference
// after the Pod is created.
func createDiscoveryConfigMap(ctx context.Context, c client.Client, pod *corev1.Pod, addOwnerRef bool) (*corev1.ConfigMap, error) {
	// Build hierarchy
	hierarchy, err := buildDiscoveryHierarchy(ctx, c, pod)
	if err != nil {
		return nil, fmt.Errorf("failed to build discovery hierarchy: %w", err)
	}

	// Marshal hierarchy to JSON
	hierarchyJSON, err := json.Marshal(hierarchy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hierarchy: %w", err)
	}

	// Extract metadata
	metadata := extractPodMetadata(pod)

	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Generate ConfigMap name
	// Format: cryostat-agent-discovery-{pod-name}
	// Truncate if necessary to stay within Kubernetes name limits (253 chars)
	cmName := fmt.Sprintf("%s%s", DiscoveryConfigMapPrefix, pod.Name)
	if len(cmName) > 253 {
		// Truncate pod name portion to fit
		maxPodNameLen := 253 - len(DiscoveryConfigMapPrefix)
		cmName = fmt.Sprintf("%s%s", DiscoveryConfigMapPrefix, pod.Name[:maxPodNameLen])
	}

	// Create ConfigMap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: pod.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": DiscoveryConfigMapManagedBy,
				"app.kubernetes.io/component":  DiscoveryConfigMapComponent,
			},
		},
		Data: map[string]string{
			"hierarchy.json": string(hierarchyJSON),
			"metadata.json":  string(metadataJSON),
		},
	}

	// Add owner reference if requested (for tests)
	if addOwnerRef && pod.UID != "" {
		cm.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       pod.Name,
				UID:        pod.UID,
				Controller: boolPtr(true),
			},
		}
	}

	return cm, nil
}

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}
