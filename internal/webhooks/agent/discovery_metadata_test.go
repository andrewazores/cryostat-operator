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
				"app":     "test-app",
				"version": "1.0.0",
			},
			Annotations: map[string]string{
				"description": "test pod",
			},
		},
	}

	metadata := extractPodMetadata(pod)

	if metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	if len(metadata.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(metadata.Labels))
	}

	if metadata.Labels["app"] != "test-app" {
		t.Errorf("Expected label 'app' to be 'test-app', got '%s'", metadata.Labels["app"])
	}

	if metadata.Labels["version"] != "1.0.0" {
		t.Errorf("Expected label 'version' to be '1.0.0', got '%s'", metadata.Labels["version"])
	}

	if len(metadata.Annotations) != 1 {
		t.Errorf("Expected 1 annotation, got %d", len(metadata.Annotations))
	}

	if metadata.Annotations["description"] != "test pod" {
		t.Errorf("Expected annotation 'description' to be 'test pod', got '%s'", metadata.Annotations["description"])
	}
}

func TestExtractPodMetadata_EmptyLabels(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels:    map[string]string{},
			Annotations: map[string]string{
				"note": "no labels",
			},
		},
	}

	metadata := extractPodMetadata(pod)

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
				"app": "test",
			},
			Annotations: map[string]string{},
		},
	}

	metadata := extractPodMetadata(pod)

	if len(metadata.Labels) != 1 {
		t.Errorf("Expected 1 label, got %d", len(metadata.Labels))
	}

	if len(metadata.Annotations) != 0 {
		t.Errorf("Expected 0 annotations, got %d", len(metadata.Annotations))
	}
}

func TestExtractPodMetadata_SystemLabelsIncluded(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app":                    "myapp",
				"kubernetes.io/pod-name": "test-pod",
				"app.kubernetes.io/name": "myapp",
			},
			Annotations: map[string]string{
				"kubernetes.io/config": "value",
			},
		},
	}

	metadata := extractPodMetadata(pod)

	// Verify kubernetes.io/* labels are included
	if metadata.Labels["kubernetes.io/pod-name"] != "test-pod" {
		t.Errorf("Expected kubernetes.io label preserved, got %v", metadata.Labels)
	}

	if metadata.Labels["app.kubernetes.io/name"] != "myapp" {
		t.Errorf("Expected app.kubernetes.io label preserved, got %v", metadata.Labels)
	}

	// Verify kubernetes.io/* annotations are included
	if metadata.Annotations["kubernetes.io/config"] != "value" {
		t.Errorf("Expected kubernetes.io annotation preserved, got %v", metadata.Annotations)
	}
}

func TestExtractPodMetadata_NilMaps(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			// Labels and Annotations are nil
		},
	}

	metadata := extractPodMetadata(pod)

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

	for i := 0; i < 100; i++ {
		labels[string(rune('a'+i%26))+string(rune('0'+i/26))] = "value"
	}

	for i := 0; i < 50; i++ {
		annotations[string(rune('a'+i%26))+string(rune('0'+i/26))] = "annotation-value"
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

	if len(metadata.Labels) != 100 {
		t.Errorf("Expected 100 labels, got %d", len(metadata.Labels))
	}

	if len(metadata.Annotations) != 50 {
		t.Errorf("Expected 50 annotations, got %d", len(metadata.Annotations))
	}
}
