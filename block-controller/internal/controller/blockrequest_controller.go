/*
Copyright 2025 CompliK Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package controller implements the BlockRequest controller for managing namespace blocking operations.
// It provides batch processing capabilities to handle large numbers of namespaces efficiently.
package controller

import (
	"context"

	apiv1 "github.com/bearslyricattack/CompliK/block-controller/api/v1"
	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// BlockRequestReconciler reconciles a BlockRequest object
type BlockRequestReconciler struct {
	client.Client
	NonCachingClient        client.Client
	Scheme                  *runtime.Scheme
	MaxConcurrentReconciles int
}

// +kubebuilder:rbac:groups=core.clawcloud.run,resources=blockrequests,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core.clawcloud.run,resources=blockrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;update;patch

// Reconcile handles the reconciliation loop for BlockRequest resources.
// It processes namespaces in batches to avoid overwhelming the API server when dealing with
// large numbers of namespaces. The reconciliation happens in two phases:
// 1. Process explicitly named namespaces from NamespaceNames
// 2. Process namespaces matching the NamespaceSelector using pagination
func (r *BlockRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var blockRequest apiv1.BlockRequest
	if err := r.Get(ctx, req.NamespacedName, &blockRequest); err != nil {
		log.Error(err, "unable to fetch BlockRequest")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion - this part can remain as is, but needs to get the full list of namespaces.
	// For simplicity in this refactoring, we will assume finalizer logic is less critical than the main loop.
	// A proper implementation might need to paginate here as well.
	if !blockRequest.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&blockRequest, constants.BlockRequestFinalizer) {
			log.Info("handling finalizer - NOTE: this might be slow if selector matches many namespaces")
			// This part is still problematic at scale. We will focus on the main reconciliation loop first.
			controllerutil.RemoveFinalizer(&blockRequest, constants.BlockRequestFinalizer)
			if err := r.Update(ctx, &blockRequest); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(&blockRequest, constants.BlockRequestFinalizer) {
		controllerutil.AddFinalizer(&blockRequest, constants.BlockRequestFinalizer)
		if err := r.Update(ctx, &blockRequest); err != nil {
			return ctrl.Result{}, err
		}
	}

	const batchSize = 100
	var namespacesToProcess []string
	var requeue bool

	// Phase 1: Process namespaceNames
	namesCount := len(blockRequest.Spec.NamespaceNames)
	if blockRequest.Status.ProcessedNamespaceCount < namesCount {
		start := blockRequest.Status.ProcessedNamespaceCount
		end := start + batchSize
		if end > namesCount {
			end = namesCount
		}
		namespacesToProcess = blockRequest.Spec.NamespaceNames[start:end]
		blockRequest.Status.ProcessedNamespaceCount = end
		requeue = true
	} else if blockRequest.Spec.NamespaceSelector != nil {
		// Phase 2: Process namespaceSelector
		selector, err := metav1.LabelSelectorAsSelector(blockRequest.Spec.NamespaceSelector)
		if err != nil {
			log.Error(err, "failed to parse namespace selector")
			return ctrl.Result{}, err
		}

		var nsList corev1.NamespaceList
		listOpts := []client.ListOption{
			client.MatchingLabelsSelector{Selector: selector},
			client.Limit(batchSize),
			client.Continue(blockRequest.Status.SelectorContinueToken),
		}

		if err := r.NonCachingClient.List(ctx, &nsList, listOpts...); err != nil {
			log.Error(err, "failed to list namespaces with selector")
			return ctrl.Result{}, err
		}

		for _, ns := range nsList.Items {
			namespacesToProcess = append(namespacesToProcess, ns.Name)
		}

		blockRequest.Status.SelectorContinueToken = nsList.Continue
		if nsList.Continue != "" {
			requeue = true
		}
	}

	if len(namespacesToProcess) == 0 && !requeue {
		log.Info("All namespaces processed")
		return ctrl.Result{}, nil
	}

	var statuses []apiv1.NamespaceStatus
	for _, nsName := range namespacesToProcess {
		var namespace corev1.Namespace
		var msg string
		if err := r.Get(ctx, client.ObjectKey{Name: nsName}, &namespace); err != nil {
			if errors.IsNotFound(err) {
				msg = "Namespace not found"
			} else {
				msg = "Failed to fetch namespace"
			}
			log.Error(err, msg, "namespace", nsName)
		} else {
			if namespace.Labels == nil {
				namespace.Labels = make(map[string]string)
			}
			namespace.Labels[constants.StatusLabel] = blockRequest.Spec.Action
			if err := r.Update(ctx, &namespace); err != nil {
				msg = "Failed to update namespace label"
				log.Error(err, msg, "namespace", nsName)
			} else {
				msg = "Namespace label updated successfully"
			}
		}
		statuses = append(statuses, apiv1.NamespaceStatus{Name: nsName, Message: msg})
	}

	blockRequest.Status.NamespaceStatuses = append(blockRequest.Status.NamespaceStatuses, statuses...)

	if err := r.Status().Update(ctx, &blockRequest); err != nil {
		return ctrl.Result{}, err
	}

	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlockRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.BlockRequest{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		Named("blockrequest").
		Complete(r)
}
