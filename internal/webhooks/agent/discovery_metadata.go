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
	corev1 "k8s.io/api/core/v1"
)

// extractPodMetadata extracts ALL labels and annotations from a Pod.
// No filtering is applied - all labels and annotations are included, even kubernetes.io/* ones.
// This ensures complete metadata visibility in Cryostat's discovery tree.
// The Agent will apply these to its internal cryostat and platform maps as needed.
func extractPodMetadata(pod *corev1.Pod) *DiscoveryMetadata {
	return &DiscoveryMetadata{
		Labels:      copyLabels(pod.Labels),
		Annotations: copyLabels(pod.Annotations),
	}
}
