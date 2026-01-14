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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetOwnerNode_NoOwner(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "standalone-pod",
			Namespace:       "test-namespace",
			OwnerReferences: []metav1.OwnerReference{},
		},
	}

	node, err := getOwnerNode(context.Background(), fakeClient, pod, "test-namespace")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if node != nil {
		t.Errorf("Expected nil node for pod with no owners, got %v", node)
	}
}

func TestGetOwnerNode_ReplicaSet(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rs",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app":               "my-app",
				"pod-template-hash": "abc123",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rs).Build()

	controller := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-rs",
					Controller: &controller,
				},
			},
		},
	}

	node, err := getOwnerNode(context.Background(), fakeClient, pod, "test-namespace")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if node == nil {
		t.Fatal("Expected non-nil node")
	}

	if node.Name != "test-rs" {
		t.Errorf("Expected node name 'test-rs', got '%s'", node.Name)
	}

	if node.NodeType != string(NodeTypeReplicaSet) {
		t.Errorf("Expected node type 'ReplicaSet', got '%s'", node.NodeType)
	}

	if len(node.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(node.Labels))
	}

	if node.Labels["app"] != "my-app" {
		t.Errorf("Expected label 'app' to be 'my-app', got '%s'", node.Labels["app"])
	}
}

func TestGetOwnerNode_Deployment(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app":     "my-app",
				"version": "1.0.0",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploy).Build()

	controller := true
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rs",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
					Controller: &controller,
				},
			},
		},
	}

	node, err := getOwnerNode(context.Background(), fakeClient, rs, "test-namespace")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if node == nil {
		t.Fatal("Expected non-nil node")
	}

	if node.Name != "test-deployment" {
		t.Errorf("Expected node name 'test-deployment', got '%s'", node.Name)
	}

	if node.NodeType != string(NodeTypeDeployment) {
		t.Errorf("Expected node type 'Deployment', got '%s'", node.NodeType)
	}
}

func TestGetOwnerNode_StatefulSet(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "my-stateful-app",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()

	controller := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       "test-statefulset",
					Controller: &controller,
				},
			},
		},
	}

	node, err := getOwnerNode(context.Background(), fakeClient, pod, "test-namespace")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if node == nil {
		t.Fatal("Expected non-nil node")
	}

	if node.Name != "test-statefulset" {
		t.Errorf("Expected node name 'test-statefulset', got '%s'", node.Name)
	}

	if node.NodeType != string(NodeTypeStatefulSet) {
		t.Errorf("Expected node type 'StatefulSet', got '%s'", node.NodeType)
	}
}

func TestGetOwnerNode_MultipleOwners_PreferExpected(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
	}

	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rs",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "my-app",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploy, rs).Build()

	controller := true
	obj := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "child-rs",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				// ReplicaSet listed first
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-rs",
					Controller: &controller,
				},
				// Deployment listed second - but should be preferred
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
					Controller: &controller,
				},
			},
		},
	}

	node, err := getOwnerNode(context.Background(), fakeClient, obj, "test-namespace")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if node == nil {
		t.Fatal("Expected non-nil node")
	}

	// Should prefer Deployment over ReplicaSet (based on ExpectedOwnerKinds priority)
	if node.Name != "test-deployment" {
		t.Errorf("Expected node name 'test-deployment' (preferred kind), got '%s'", node.Name)
	}

	if node.NodeType != string(NodeTypeDeployment) {
		t.Errorf("Expected node type 'Deployment', got '%s'", node.NodeType)
	}
}

func TestGetOwnerNode_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	controller := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "deleted-deployment",
					Controller: &controller,
				},
			},
		},
	}

	node, err := getOwnerNode(context.Background(), fakeClient, pod, "test-namespace")

	// Should return nil node (not error) when owner is not found
	if err != nil {
		t.Fatalf("Expected no error for missing owner, got %v", err)
	}

	if node != nil {
		t.Errorf("Expected nil node for deleted owner, got %v", node)
	}
}

func TestQueryForNode_Deployment(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app":                      "my-app",
				"kubernetes.io/managed-by": "operator",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploy).Build()

	node, err := queryForNode(context.Background(), fakeClient, "test-namespace", "test-deployment", "Deployment")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if node == nil {
		t.Fatal("Expected non-nil node")
	}

	if node.Name != "test-deployment" {
		t.Errorf("Expected node name 'test-deployment', got '%s'", node.Name)
	}

	if node.NodeType != "Deployment" {
		t.Errorf("Expected node type 'Deployment', got '%s'", node.NodeType)
	}

	// Verify ALL labels are included
	if len(node.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(node.Labels))
	}

	if node.Labels["kubernetes.io/managed-by"] != "operator" {
		t.Error("Expected system label to be included")
	}

	if len(node.Children) != 0 {
		t.Errorf("Expected empty children array, got %d children", len(node.Children))
	}
}

func TestQueryForNode_UnknownKind(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	node, err := queryForNode(context.Background(), fakeClient, "test-namespace", "unknown-resource", "UnknownKind")

	if err != nil {
		t.Fatalf("Expected no error for unknown kind, got %v", err)
	}

	if node != nil {
		t.Errorf("Expected nil node for unknown kind, got %v", node)
	}
}

func TestCopyLabels_NilInput(t *testing.T) {
	result := copyLabels(nil)

	if result == nil {
		t.Fatal("Expected non-nil map")
	}

	if len(result) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(result))
	}
}

func TestCopyLabels_WithData(t *testing.T) {
	input := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	result := copyLabels(input)

	if result == nil {
		t.Fatal("Expected non-nil map")
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result))
	}

	if result["key1"] != "value1" {
		t.Errorf("Expected 'value1', got '%s'", result["key1"])
	}

	// Verify it's a copy, not the same map
	input["key3"] = "value3"
	if _, exists := result["key3"]; exists {
		t.Error("Expected copy to be independent of original")
	}
}
