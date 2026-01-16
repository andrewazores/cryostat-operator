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

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch;create
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile adds owner references to discovery ConfigMaps
func (r *DiscoveryConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, req.NamespacedName, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if configMap.Labels["app.kubernetes.io/component"] != agent.DiscoveryConfigMapComponent {
		return ctrl.Result{}, nil
	}

	if len(configMap.OwnerReferences) > 0 {
		return ctrl.Result{}, nil
	}

	// Extract Pod name from ConfigMap name
	// Format: cryostat-agent-discovery-{pod-name} or
	//         cryostat-agent-discovery-{pod-generateName}-{random-suffix}
	nameWithoutPrefix := strings.TrimPrefix(configMap.Name, agent.DiscoveryConfigMapPrefix)
	if nameWithoutPrefix == configMap.Name {
		log.Info("ConfigMap name doesn't match expected format", "name", configMap.Name)
		return ctrl.Result{}, nil
	}

	podName := nameWithoutPrefix

	pod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      podName,
		Namespace: configMap.Namespace,
	}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			lastHyphen := strings.LastIndex(podName, "-")
			if lastHyphen > 0 {
				potentialGenerateName := podName[:lastHyphen+1] // Include the trailing hyphen

				podList := &corev1.PodList{}
				if err := r.List(ctx, podList, client.InNamespace(configMap.Namespace)); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to list pods: %w", err)
				}

				var foundPod *corev1.Pod
				for i := range podList.Items {
					p := &podList.Items[i]
					if strings.HasPrefix(p.Name, potentialGenerateName) {
						foundPod = p
						break
					}
				}

				if foundPod != nil {
					pod = foundPod
					log.Info("Found pod by GenerateName prefix", "configMap", configMap.Name, "pod", pod.Name)
				} else {
					log.Info("Pod not found yet, will retry", "expectedName", podName, "generateNamePrefix", potentialGenerateName)
					return ctrl.Result{Requeue: true}, nil
				}
			} else {
				log.Info("Pod not found yet, will retry", "pod", podName)
				return ctrl.Result{Requeue: true}, nil
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       pod.Name,
		UID:        pod.UID,
	}
	configMap.OwnerReferences = append(configMap.OwnerReferences, ownerRef)

	err = r.Update(ctx, configMap)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update ConfigMap with owner reference: %w", err)
	}

	log.Info("Added owner reference to discovery ConfigMap", "configMap", configMap.Name, "pod", pod.Name)
	return ctrl.Result{}, nil
}

func (r *DiscoveryConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}
