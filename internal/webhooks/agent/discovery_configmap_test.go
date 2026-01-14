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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateDiscoveryConfigMap_Success(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			UID:       "pod-uid-123",
			Labels: map[string]string{
				"app": "myapp",
			},
			Annotations: map[string]string{
				"note": "test",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, pod).
		Build()

	cm, err := createDiscoveryConfigMap(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cm == nil {
		t.Fatal("Expected non-nil ConfigMap")
	}

	// Verify ConfigMap name
	expectedName := "cryostat-agent-discovery-test-pod"
	if cm.Name != expectedName {
		t.Errorf("Expected ConfigMap name '%s', got '%s'", expectedName, cm.Name)
	}

	// Verify ConfigMap namespace
	if cm.Namespace != "test-namespace" {
		t.Errorf("Expected ConfigMap namespace 'test-namespace', got '%s'", cm.Namespace)
	}

	// Verify ConfigMap has both files
	if len(cm.Data) != 2 {
		t.Errorf("Expected 2 data entries, got %d", len(cm.Data))
	}

	if _, exists := cm.Data["hierarchy.json"]; !exists {
		t.Error("Expected hierarchy.json in ConfigMap data")
	}

	if _, exists := cm.Data["metadata.json"]; !exists {
		t.Error("Expected metadata.json in ConfigMap data")
	}

	// Verify OwnerReference
	if len(cm.OwnerReferences) != 1 {
		t.Fatalf("Expected 1 owner reference, got %d", len(cm.OwnerReferences))
	}

	owner := cm.OwnerReferences[0]
	if owner.Kind != "Pod" {
		t.Errorf("Expected owner kind 'Pod', got '%s'", owner.Kind)
	}
	if owner.Name != "test-pod" {
		t.Errorf("Expected owner name 'test-pod', got '%s'", owner.Name)
	}
	if owner.UID != "pod-uid-123" {
		t.Errorf("Expected owner UID 'pod-uid-123', got '%s'", owner.UID)
	}
}

func TestCreateDiscoveryConfigMap_ValidJSON(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deployment",
			Namespace: "test-namespace",
			UID:       "deployment-uid",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "test-namespace",
			UID:       "pod-uid",
			Labels: map[string]string{
				"app": "myapp",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "my-deployment",
					UID:        "deployment-uid",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, deployment, pod).
		Build()

	cm, err := createDiscoveryConfigMap(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify hierarchy.json is valid JSON
	var hierarchy DiscoveryNode
	if err := json.Unmarshal([]byte(cm.Data["hierarchy.json"]), &hierarchy); err != nil {
		t.Errorf("hierarchy.json is not valid JSON: %v", err)
	}

	// Verify hierarchy structure
	if hierarchy.NodeType != string(KubeNodeTypeNamespace) {
		t.Errorf("Expected root NodeType 'Namespace', got '%s'", hierarchy.NodeType)
	}

	// Verify metadata.json is valid JSON
	var metadata DiscoveryMetadata
	if err := json.Unmarshal([]byte(cm.Data["metadata.json"]), &metadata); err != nil {
		t.Errorf("metadata.json is not valid JSON: %v", err)
	}

	// Verify metadata contains Pod labels
	if metadata.Labels["app"] != "myapp" {
		t.Errorf("Expected metadata label 'app=myapp', got %v", metadata.Labels)
	}
}

func TestCreateDiscoveryConfigMap_Naming(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	// Test with a long pod name to verify name length handling
	longName := "very-long-pod-name-that-might-exceed-kubernetes-name-length-limits-when-combined-with-prefix"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      longName,
			Namespace: "test-namespace",
			UID:       "pod-uid",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, pod).
		Build()

	cm, err := createDiscoveryConfigMap(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify ConfigMap name doesn't exceed Kubernetes limits (253 characters)
	if len(cm.Name) > 253 {
		t.Errorf("ConfigMap name exceeds 253 characters: %d", len(cm.Name))
	}

	// Verify name starts with expected prefix
	if len(cm.Name) < len("cryostat-agent-discovery-") {
		t.Errorf("ConfigMap name too short: %s", cm.Name)
	}
}

func TestCreateDiscoveryConfigMap_EmptyMetadata(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	// Pod with no labels or annotations
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			UID:       "pod-uid",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, pod).
		Build()

	cm, err := createDiscoveryConfigMap(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify metadata.json exists and is valid
	var metadata DiscoveryMetadata
	if err := json.Unmarshal([]byte(cm.Data["metadata.json"]), &metadata); err != nil {
		t.Errorf("metadata.json is not valid JSON: %v", err)
	}

	// Verify empty maps are present (not nil)
	if metadata.Labels == nil {
		t.Error("Expected non-nil Labels map")
	}
	if metadata.Annotations == nil {
		t.Error("Expected non-nil Annotations map")
	}
}

func TestCreateDiscoveryConfigMap_DeepHierarchy(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "production",
			Labels: map[string]string{
				"env": "prod",
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend-api",
			Namespace: "production",
			UID:       "deployment-uid-456",
			Labels: map[string]string{
				"app":     "backend",
				"tier":    "api",
				"version": "v2.1.0",
			},
		},
	}

	replicaSet := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend-api-7f8c9d",
			Namespace: "production",
			UID:       "replicaset-uid-789",
			Labels: map[string]string{
				"app":               "backend",
				"tier":              "api",
				"pod-template-hash": "7f8c9d",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "backend-api",
					UID:        "deployment-uid-456",
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend-api-7f8c9d-xk2lp",
			Namespace: "production",
			UID:       "pod-uid-abc",
			Labels: map[string]string{
				"app":               "backend",
				"tier":              "api",
				"pod-template-hash": "7f8c9d",
				"custom-label":      "custom-value",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "8080",
				"deployment-timestamp": "2024-01-15T10:30:00Z",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "backend-api-7f8c9d",
					UID:        "replicaset-uid-789",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, deployment, replicaSet, pod).
		Build()

	cm, err := createDiscoveryConfigMap(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Parse hierarchy.json
	var hierarchy DiscoveryNode
	if err := json.Unmarshal([]byte(cm.Data["hierarchy.json"]), &hierarchy); err != nil {
		t.Fatalf("Failed to unmarshal hierarchy.json: %v", err)
	}

	// Verify Namespace (root) node
	if hierarchy.Name != "production" {
		t.Errorf("Expected root name 'production', got '%s'", hierarchy.Name)
	}
	if hierarchy.NodeType != string(KubeNodeTypeNamespace) {
		t.Errorf("Expected root NodeType 'Namespace', got '%s'", hierarchy.NodeType)
	}
	if hierarchy.Labels["env"] != "prod" {
		t.Errorf("Expected namespace label 'env=prod', got %v", hierarchy.Labels)
	}
	if len(hierarchy.Children) != 1 {
		t.Fatalf("Expected namespace to have 1 child, got %d", len(hierarchy.Children))
	}

	// Verify Deployment node
	deploymentNode := hierarchy.Children[0]
	if deploymentNode.Name != "backend-api" {
		t.Errorf("Expected deployment name 'backend-api', got '%s'", deploymentNode.Name)
	}
	if deploymentNode.NodeType != string(KubeNodeTypeDeployment) {
		t.Errorf("Expected deployment NodeType 'Deployment', got '%s'", deploymentNode.NodeType)
	}
	if deploymentNode.Labels["app"] != "backend" {
		t.Errorf("Expected deployment label 'app=backend', got %v", deploymentNode.Labels)
	}
	if deploymentNode.Labels["tier"] != "api" {
		t.Errorf("Expected deployment label 'tier=api', got %v", deploymentNode.Labels)
	}
	if deploymentNode.Labels["version"] != "v2.1.0" {
		t.Errorf("Expected deployment label 'version=v2.1.0', got %v", deploymentNode.Labels)
	}
	if len(deploymentNode.Children) != 1 {
		t.Fatalf("Expected deployment to have 1 child, got %d", len(deploymentNode.Children))
	}

	// Verify ReplicaSet node
	replicaSetNode := deploymentNode.Children[0]
	if replicaSetNode.Name != "backend-api-7f8c9d" {
		t.Errorf("Expected replicaset name 'backend-api-7f8c9d', got '%s'", replicaSetNode.Name)
	}
	if replicaSetNode.NodeType != string(KubeNodeTypeReplicaSet) {
		t.Errorf("Expected replicaset NodeType 'ReplicaSet', got '%s'", replicaSetNode.NodeType)
	}
	if replicaSetNode.Labels["pod-template-hash"] != "7f8c9d" {
		t.Errorf("Expected replicaset label 'pod-template-hash=7f8c9d', got %v", replicaSetNode.Labels)
	}
	if len(replicaSetNode.Children) != 1 {
		t.Fatalf("Expected replicaset to have 1 child, got %d", len(replicaSetNode.Children))
	}

	// Verify Pod node
	podNode := replicaSetNode.Children[0]
	if podNode.Name != "backend-api-7f8c9d-xk2lp" {
		t.Errorf("Expected pod name 'backend-api-7f8c9d-xk2lp', got '%s'", podNode.Name)
	}
	if podNode.NodeType != string(KubeNodeTypeJvmPod) {
		t.Errorf("Expected pod NodeType 'JVM_POD', got '%s'", podNode.NodeType)
	}
	if podNode.Labels["custom-label"] != "custom-value" {
		t.Errorf("Expected pod label 'custom-label=custom-value', got %v", podNode.Labels)
	}
	if len(podNode.Children) != 0 {
		t.Errorf("Expected pod to have 0 children, got %d", len(podNode.Children))
	}

	// Parse metadata.json
	var metadata DiscoveryMetadata
	if err := json.Unmarshal([]byte(cm.Data["metadata.json"]), &metadata); err != nil {
		t.Fatalf("Failed to unmarshal metadata.json: %v", err)
	}

	// Verify metadata contains ALL Pod labels
	expectedLabels := map[string]string{
		"app":               "backend",
		"tier":              "api",
		"pod-template-hash": "7f8c9d",
		"custom-label":      "custom-value",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, exists := metadata.Labels[key]; !exists {
			t.Errorf("Expected label '%s' to exist in metadata", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected label '%s=%s', got '%s=%s'", key, expectedValue, key, actualValue)
		}
	}

	// Verify metadata contains ALL Pod annotations
	expectedAnnotations := map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "8080",
		"deployment-timestamp": "2024-01-15T10:30:00Z",
	}
	for key, expectedValue := range expectedAnnotations {
		if actualValue, exists := metadata.Annotations[key]; !exists {
			t.Errorf("Expected annotation '%s' to exist in metadata", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected annotation '%s=%s', got '%s=%s'", key, expectedValue, key, actualValue)
		}
	}

	// Verify ConfigMap structure
	if cm.Name != "cryostat-agent-discovery-backend-api-7f8c9d-xk2lp" {
		t.Errorf("Expected ConfigMap name 'cryostat-agent-discovery-backend-api-7f8c9d-xk2lp', got '%s'", cm.Name)
	}
	if cm.Namespace != "production" {
		t.Errorf("Expected ConfigMap namespace 'production', got '%s'", cm.Namespace)
	}

	// Verify OwnerReference points to Pod
	if len(cm.OwnerReferences) != 1 {
		t.Fatalf("Expected 1 owner reference, got %d", len(cm.OwnerReferences))
	}
	if cm.OwnerReferences[0].Name != "backend-api-7f8c9d-xk2lp" {
		t.Errorf("Expected owner name 'backend-api-7f8c9d-xk2lp', got '%s'", cm.OwnerReferences[0].Name)
	}
	if cm.OwnerReferences[0].UID != "pod-uid-abc" {
		t.Errorf("Expected owner UID 'pod-uid-abc', got '%s'", cm.OwnerReferences[0].UID)
	}
}
