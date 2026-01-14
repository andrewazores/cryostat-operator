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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractPodMetadata_Success(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app":     "my-app",
				"version": "1.0.0",
				"env":     "production",
			},
			Annotations: map[string]string{
				"description": "Test application",
				"owner":       "platform-team",
			},
		},
	}

	metadata := extractPodMetadata(pod)

	if metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	if len(metadata.Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(metadata.Labels))
	}

	if metadata.Labels["app"] != "my-app" {
		t.Errorf("Expected label 'app' to be 'my-app', got '%s'", metadata.Labels["app"])
	}

	if metadata.Labels["version"] != "1.0.0" {
		t.Errorf("Expected label 'version' to be '1.0.0', got '%s'", metadata.Labels["version"])
	}

	if len(metadata.Annotations) != 2 {
		t.Errorf("Expected 2 annotations, got %d", len(metadata.Annotations))
	}

	if metadata.Annotations["description"] != "Test application" {
		t.Errorf("Expected annotation 'description' to be 'Test application', got '%s'", metadata.Annotations["description"])
	}
}

func TestExtractPodMetadata_EmptyLabels(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels:    map[string]string{},
			Annotations: map[string]string{
				"description": "Test application",
			},
		},
	}

	metadata := extractPodMetadata(pod)

	if metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	if metadata.Labels == nil {
		t.Fatal("Expected labels map to be non-nil")
	}

	if len(metadata.Labels) != 0 {
		t.Errorf("Expected 0 labels, got %d", len(metadata.Labels))
	}

	if len(metadata.Annotations) != 1 {
		t.Errorf("Expected 1 annotation, got %d", len(metadata.Annotations))
	}
}

func TestExtractPodMetadata_EmptyAnnotations(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "my-app",
			},
			Annotations: map[string]string{},
		},
	}

	metadata := extractPodMetadata(pod)

	if metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	if metadata.Annotations == nil {
		t.Fatal("Expected annotations map to be non-nil")
	}

	if len(metadata.Annotations) != 0 {
		t.Errorf("Expected 0 annotations, got %d", len(metadata.Annotations))
	}

	if len(metadata.Labels) != 1 {
		t.Errorf("Expected 1 label, got %d", len(metadata.Labels))
	}
}

func TestExtractPodMetadata_SystemLabelsIncluded(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app":                         "my-app",
				"kubernetes.io/metadata.name": "test-pod",
				"pod-template-hash":           "abc123",
				"cryostat.io/agent.cryostat":  "cryostat",
			},
			Annotations: map[string]string{
				"description":              "Test application",
				"kubernetes.io/created-by": "deployment-controller",
				"prometheus.io/scrape":     "true",
			},
		},
	}

	metadata := extractPodMetadata(pod)

	if metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	// Verify ALL labels are included (no filtering)
	if len(metadata.Labels) != 4 {
		t.Errorf("Expected 4 labels (including system labels), got %d", len(metadata.Labels))
	}

	// Verify system labels are present
	if _, exists := metadata.Labels["kubernetes.io/metadata.name"]; !exists {
		t.Error("Expected kubernetes.io/metadata.name label to be included")
	}

	if _, exists := metadata.Labels["cryostat.io/agent.cryostat"]; !exists {
		t.Error("Expected cryostat.io/agent.cryostat label to be included")
	}

	// Verify ALL annotations are included (no filtering)
	if len(metadata.Annotations) != 3 {
		t.Errorf("Expected 3 annotations (including system annotations), got %d", len(metadata.Annotations))
	}

	// Verify system annotations are present
	if _, exists := metadata.Annotations["kubernetes.io/created-by"]; !exists {
		t.Error("Expected kubernetes.io/created-by annotation to be included")
	}
}

func TestExtractPodMetadata_NilMaps(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod",
			Namespace:   "test-namespace",
			Labels:      nil,
			Annotations: nil,
		},
	}

	metadata := extractPodMetadata(pod)

	if metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	if metadata.Labels == nil {
		t.Fatal("Expected labels map to be initialized (non-nil)")
	}

	if metadata.Annotations == nil {
		t.Fatal("Expected annotations map to be initialized (non-nil)")
	}

	if len(metadata.Labels) != 0 {
		t.Errorf("Expected 0 labels, got %d", len(metadata.Labels))
	}

	if len(metadata.Annotations) != 0 {
		t.Errorf("Expected 0 annotations, got %d", len(metadata.Annotations))
	}
}

func TestExtractPodMetadata_LargeMetadata(t *testing.T) {
	labels := make(map[string]string)
	annotations := make(map[string]string)

	// Create 50 labels and 50 annotations
	for i := 0; i < 50; i++ {
		labels[string(rune('a'+i%26))+string(rune('0'+i/26))] = "value"
		annotations[string(rune('A'+i%26))+string(rune('0'+i/26))] = "annotation-value"
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod",
			Namespace:   "test-namespace",
			Labels:      labels,
			Annotations: annotations,
		},
	}

	metadata := extractPodMetadata(pod)

	if metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	if len(metadata.Labels) != 50 {
		t.Errorf("Expected 50 labels, got %d", len(metadata.Labels))
	}

	if len(metadata.Annotations) != 50 {
		t.Errorf("Expected 50 annotations, got %d", len(metadata.Annotations))
	}
}
