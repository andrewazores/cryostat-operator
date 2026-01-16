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
	"strings"

	"github.com/cryostatio/cryostat-operator/internal/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	lowerAlphanumerics = "abcdefghijklmnopqrstuvwxyz0123456789"
	randomSuffixLength = 5
	nameLengthLimit    = 253
)

// createDiscoveryConfigMap creates a ConfigMap containing hierarchy.json and metadata.json
// for the Cryostat Agent to read. The ConfigMap is created without an owner reference since
// the Pod doesn't exist yet during webhook mutation.
func createDiscoveryConfigMap(ctx context.Context, c client.Client, pod *corev1.Pod, addOwnerRef bool) (*corev1.ConfigMap, error) {
	hierarchy, err := buildDiscoveryHierarchy(ctx, c, pod)
	if err != nil {
		return nil, fmt.Errorf("failed to build discovery hierarchy: %w", err)
	}

	hierarchyJSON, err := json.Marshal(hierarchy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hierarchy: %w", err)
	}

	metadata := extractPodMetadata(pod)

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Format: cryostat-agent-discovery-{pod-name}
	// If pod.Name is empty (during webhook mutation before pod creation),
	// use GenerateName with a random suffix
	var cmName string
	if pod.Name != "" {
		cmName = fmt.Sprintf("%s%s", DiscoveryConfigMapPrefix, pod.Name)
	} else if pod.GenerateName != "" {
		// Use GenerateName with random suffix to ensure uniqueness
		// This ConfigMap will be orphaned and cleaned up by the controller
		// once the actual pod name is known
		osUtils := &common.DefaultOSUtils{}
		cmName = fmt.Sprintf("%s%s%s",
			DiscoveryConfigMapPrefix,
			pod.GenerateName,
			osUtils.GenRandomString(randomSuffixLength, lowerAlphanumerics),
		)
	} else {
		return nil, fmt.Errorf("pod has neither Name nor GenerateName set")
	}

	if len(cmName) > nameLengthLimit {
		cmName = cmName[:nameLengthLimit]
		cmName = strings.TrimRight(cmName, "-.")
	}

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

	if addOwnerRef && pod.UID != "" {
		controller := true
		cm.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       pod.Name,
				UID:        pod.UID,
				Controller: &controller,
			},
		}
	}

	return cm, nil
}
