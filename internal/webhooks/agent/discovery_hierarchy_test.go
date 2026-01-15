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

	if node.NodeType != string(KubeNodeTypeReplicaSet) {
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

	if node.NodeType != string(KubeNodeTypeDeployment) {
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

	if node.NodeType != string(KubeNodeTypeStatefulSet) {
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

	if node.NodeType != string(KubeNodeTypeDeployment) {
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

// TestBuildDiscoveryHierarchy_FullChain tests building a complete hierarchy:
// Namespace → Deployment → ReplicaSet → Pod
func TestBuildDiscoveryHierarchy_FullChain(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			Labels: map[string]string{
				"env": "production",
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deployment",
			Namespace: "test-namespace",
			UID:       "deployment-uid",
			Labels: map[string]string{
				"app": "myapp",
			},
		},
	}

	replicaSet := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deployment-abc123",
			Namespace: "test-namespace",
			UID:       "replicaset-uid",
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

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "test-namespace",
			UID:       "pod-uid",
			Labels: map[string]string{
				"app": "myapp",
				"pod": "label",
			},
			Annotations: map[string]string{
				"annotation": "value",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "my-deployment-abc123",
					UID:        "replicaset-uid",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, deployment, replicaSet, pod).
		Build()

	hierarchy, err := buildDiscoveryHierarchy(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify root is Namespace
	if hierarchy.NodeType != string(KubeNodeTypeNamespace) {
		t.Errorf("Expected root NodeType to be Namespace, got %s", hierarchy.NodeType)
	}
	if hierarchy.Name != "test-namespace" {
		t.Errorf("Expected root Name to be 'test-namespace', got %s", hierarchy.Name)
	}
	if hierarchy.Labels["env"] != "production" {
		t.Errorf("Expected namespace label 'env=production', got %v", hierarchy.Labels)
	}

	// Verify Namespace has 1 child (Deployment)
	if len(hierarchy.Children) != 1 {
		t.Fatalf("Expected Namespace to have 1 child, got %d", len(hierarchy.Children))
	}

	deployment_node := hierarchy.Children[0]
	if deployment_node.NodeType != string(KubeNodeTypeDeployment) {
		t.Errorf("Expected child NodeType to be Deployment, got %s", deployment_node.NodeType)
	}
	if deployment_node.Name != "my-deployment" {
		t.Errorf("Expected child Name to be 'my-deployment', got %s", deployment_node.Name)
	}

	// Verify Deployment has 1 child (ReplicaSet)
	if len(deployment_node.Children) != 1 {
		t.Fatalf("Expected Deployment to have 1 child, got %d", len(deployment_node.Children))
	}

	replicaset_node := deployment_node.Children[0]
	if replicaset_node.NodeType != string(KubeNodeTypeReplicaSet) {
		t.Errorf("Expected child NodeType to be ReplicaSet, got %s", replicaset_node.NodeType)
	}
	if replicaset_node.Name != "my-deployment-abc123" {
		t.Errorf("Expected child Name to be 'my-deployment-abc123', got %s", replicaset_node.Name)
	}

	// Verify ReplicaSet has 1 child (Pod)
	if len(replicaset_node.Children) != 1 {
		t.Fatalf("Expected ReplicaSet to have 1 child, got %d", len(replicaset_node.Children))
	}

	pod_node := replicaset_node.Children[0]
	if pod_node.NodeType != string(KubeNodeTypePod) {
		t.Errorf("Expected child NodeType to be Pod, got %s", pod_node.NodeType)
	}
	if pod_node.Name != "my-pod" {
		t.Errorf("Expected child Name to be 'my-pod', got %s", pod_node.Name)
	}

	// Verify Pod has empty children array
	if pod_node.Children == nil {
		t.Error("Expected Pod to have non-nil Children array")
	}
	if len(pod_node.Children) != 0 {
		t.Errorf("Expected Pod to have empty Children array, got %d children", len(pod_node.Children))
	}

	// Verify Pod has all labels
	if pod_node.Labels["app"] != "myapp" {
		t.Errorf("Expected Pod label 'app=myapp', got %v", pod_node.Labels)
	}
	if pod_node.Labels["pod"] != "label" {
		t.Errorf("Expected Pod label 'pod=label', got %v", pod_node.Labels)
	}
}

// TestBuildDiscoveryHierarchy_NoOwners tests a standalone Pod with no owners
func TestBuildDiscoveryHierarchy_NoOwners(t *testing.T) {
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
			Name:            "standalone-pod",
			Namespace:       "test-namespace",
			OwnerReferences: []metav1.OwnerReference{},
			Labels: map[string]string{
				"standalone": "true",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, pod).
		Build()

	hierarchy, err := buildDiscoveryHierarchy(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify root is Namespace
	if hierarchy.NodeType != string(KubeNodeTypeNamespace) {
		t.Errorf("Expected root NodeType to be Namespace, got %s", hierarchy.NodeType)
	}

	// Verify Namespace has 1 child (Pod directly)
	if len(hierarchy.Children) != 1 {
		t.Fatalf("Expected Namespace to have 1 child, got %d", len(hierarchy.Children))
	}

	pod_node := hierarchy.Children[0]
	if pod_node.NodeType != string(KubeNodeTypePod) {
		t.Errorf("Expected child NodeType to be Pod, got %s", pod_node.NodeType)
	}
	if pod_node.Name != "standalone-pod" {
		t.Errorf("Expected child Name to be 'standalone-pod', got %s", pod_node.Name)
	}
	if pod_node.Labels["standalone"] != "true" {
		t.Errorf("Expected Pod label 'standalone=true', got %v", pod_node.Labels)
	}
}

// TestBuildDiscoveryHierarchy_StatefulSet tests StatefulSet → Pod hierarchy
func TestBuildDiscoveryHierarchy_StatefulSet(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-statefulset",
			Namespace: "test-namespace",
			UID:       "statefulset-uid",
			Labels: map[string]string{
				"app": "database",
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-statefulset-0",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "database",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       "my-statefulset",
					UID:        "statefulset-uid",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, statefulSet, pod).
		Build()

	hierarchy, err := buildDiscoveryHierarchy(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify: Namespace → StatefulSet → Pod
	if hierarchy.NodeType != string(KubeNodeTypeNamespace) {
		t.Errorf("Expected root NodeType to be Namespace, got %s", hierarchy.NodeType)
	}

	if len(hierarchy.Children) != 1 {
		t.Fatalf("Expected Namespace to have 1 child, got %d", len(hierarchy.Children))
	}

	statefulset_node := hierarchy.Children[0]
	if statefulset_node.NodeType != string(KubeNodeTypeStatefulSet) {
		t.Errorf("Expected child NodeType to be StatefulSet, got %s", statefulset_node.NodeType)
	}
	if statefulset_node.Name != "my-statefulset" {
		t.Errorf("Expected child Name to be 'my-statefulset', got %s", statefulset_node.Name)
	}

	if len(statefulset_node.Children) != 1 {
		t.Fatalf("Expected StatefulSet to have 1 child, got %d", len(statefulset_node.Children))
	}

	pod_node := statefulset_node.Children[0]
	if pod_node.NodeType != string(KubeNodeTypePod) {
		t.Errorf("Expected child NodeType to be Pod, got %s", pod_node.NodeType)
	}
}

// TestBuildDiscoveryHierarchy_AllLabelsPreserved verifies ALL labels are included at each level
func TestBuildDiscoveryHierarchy_AllLabelsPreserved(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			Labels: map[string]string{
				"kubernetes.io/metadata.name": "test-namespace",
				"custom-ns-label":             "value",
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deployment",
			Namespace: "test-namespace",
			UID:       "deployment-uid",
			Labels: map[string]string{
				"app":                         "myapp",
				"kubernetes.io/managed-by":    "operator",
				"app.kubernetes.io/component": "backend",
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app":                    "myapp",
				"pod-template-hash":      "abc123",
				"kubernetes.io/pod-name": "my-pod",
				"custom-label":           "custom-value",
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

	hierarchy, err := buildDiscoveryHierarchy(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify Namespace has ALL labels including kubernetes.io/*
	if hierarchy.Labels["kubernetes.io/metadata.name"] != "test-namespace" {
		t.Errorf("Expected namespace kubernetes.io label preserved, got %v", hierarchy.Labels)
	}
	if hierarchy.Labels["custom-ns-label"] != "value" {
		t.Errorf("Expected namespace custom label preserved, got %v", hierarchy.Labels)
	}

	// Verify Deployment has ALL labels
	deployment_node := hierarchy.Children[0]
	if deployment_node.Labels["kubernetes.io/managed-by"] != "operator" {
		t.Errorf("Expected deployment kubernetes.io label preserved, got %v", deployment_node.Labels)
	}
	if deployment_node.Labels["app.kubernetes.io/component"] != "backend" {
		t.Errorf("Expected deployment app.kubernetes.io label preserved, got %v", deployment_node.Labels)
	}

	// Verify Pod has ALL labels
	pod_node := deployment_node.Children[0]
	if pod_node.Labels["kubernetes.io/pod-name"] != "my-pod" {
		t.Errorf("Expected pod kubernetes.io label preserved, got %v", pod_node.Labels)
	}
	if pod_node.Labels["pod-template-hash"] != "abc123" {
		t.Errorf("Expected pod-template-hash label preserved, got %v", pod_node.Labels)
	}
	if pod_node.Labels["custom-label"] != "custom-value" {
		t.Errorf("Expected custom label preserved, got %v", pod_node.Labels)
	}
}

// TestBuildDiscoveryHierarchy_EmptyChildrenArray verifies leaf Pod has empty (not nil) children array
func TestBuildDiscoveryHierarchy_EmptyChildrenArray(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "test-namespace",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace, pod).
		Build()

	hierarchy, err := buildDiscoveryHierarchy(context.Background(), fakeClient, pod)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	pod_node := hierarchy.Children[0]

	// Verify Children is not nil
	if pod_node.Children == nil {
		t.Error("Expected Pod Children to be non-nil empty array, got nil")
	}

	// Verify Children is empty
	if len(pod_node.Children) != 0 {
		t.Errorf("Expected Pod Children to be empty array, got %d children", len(pod_node.Children))
	}
}
