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

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/cryostatio/cryostat-operator/internal/webhooks/agent"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DiscoveryConfigMapReconciler reconciles discovery ConfigMaps to add owner references
type DiscoveryConfigMapReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile adds owner references to discovery ConfigMaps
func (r *DiscoveryConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the ConfigMap
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, req.NamespacedName, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap deleted, nothing to do
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if this is a discovery ConfigMap
	if configMap.Labels["app.kubernetes.io/component"] != agent.DiscoveryConfigMapComponent {
		// Not a discovery ConfigMap, ignore
		return ctrl.Result{}, nil
	}

	// Check if owner reference already exists
	if len(configMap.OwnerReferences) > 0 {
		// Owner reference already set, nothing to do
		return ctrl.Result{}, nil
	}

	// Extract Pod name from ConfigMap name
	// Format: cryostat-agent-discovery-{pod-name}
	podName := strings.TrimPrefix(configMap.Name, agent.DiscoveryConfigMapPrefix)
	if podName == configMap.Name {
		// ConfigMap name doesn't match expected format
		log.Info("ConfigMap name doesn't match expected format", "name", configMap.Name)
		return ctrl.Result{}, nil
	}

	// Fetch the Pod
	pod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      podName,
		Namespace: configMap.Namespace,
	}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Pod doesn't exist yet, requeue
			log.Info("Pod not found yet, will retry", "pod", podName)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// Add owner reference
	ownerRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       pod.Name,
		UID:        pod.UID,
	}
	configMap.OwnerReferences = append(configMap.OwnerReferences, ownerRef)

	// Update the ConfigMap
	err = r.Update(ctx, configMap)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update ConfigMap with owner reference: %w", err)
	}

	log.Info("Added owner reference to discovery ConfigMap", "configMap", configMap.Name, "pod", pod.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *DiscoveryConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}
