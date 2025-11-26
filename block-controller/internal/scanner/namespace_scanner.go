// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scanner

import (
	"context"
	"strconv"
	"time"

	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
	"github.com/bearslyricattack/CompliK/block-controller/internal/utils"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceScanner scans namespaces and applies blocking policies

// TODO(user): Add unit tests for the scanner

type NamespaceScanner struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	LockDuration     time.Duration
	FastScanInterval time.Duration
	SlowScanInterval time.Duration
	ScanBatchSize    int
}

// Start starts the namespace scanner with two tickers for fast and slow scans.
func (s *NamespaceScanner) Start(ctx context.Context) error {
	fastTicker := time.NewTicker(s.FastScanInterval)
	defer fastTicker.Stop()
	slowTicker := time.NewTicker(s.SlowScanInterval)
	defer slowTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-fastTicker.C:
			s.Log.Info("Starting fast scan")
			if err := s.fastScan(ctx); err != nil {
				s.Log.Error(err, "Fast scan failed")
			}
		case <-slowTicker.C:
			s.Log.Info("Starting slow scan (janitor)")
			if err := s.slowScan(ctx); err != nil {
				s.Log.Error(err, "Slow scan failed")
			}
		}
	}
}

func (s *NamespaceScanner) fastScan(ctx context.Context) error {
	log := s.Log.WithName("fast-scan")

	// Process locked namespaces
	var lockedNsList corev1.NamespaceList
	lockedSelector := client.MatchingLabels{constants.StatusLabel: constants.LockedStatus}
	if err := s.List(ctx, &lockedNsList, lockedSelector); err != nil {
		log.Error(err, "failed to list locked namespaces")
		return err
	}
	for _, ns := range lockedNsList.Items {
		if err := s.processNamespace(ctx, ns); err != nil {
			log.Error(err, "failed to process locked namespace", "namespace", ns.Name)
		}
	}

	// Process active namespaces
	var activeNsList corev1.NamespaceList
	activeSelector := client.MatchingLabels{constants.StatusLabel: constants.ActiveStatus}
	if err := s.List(ctx, &activeNsList, activeSelector); err != nil {
		log.Error(err, "failed to list active namespaces")
		return err
	}
	for _, ns := range activeNsList.Items {
		if err := s.processNamespace(ctx, ns); err != nil {
			log.Error(err, "failed to process active namespace", "namespace", ns.Name)
		}
	}

	return nil
}

func (s *NamespaceScanner) slowScan(ctx context.Context) error {
	var continueToken string
	for {
		namespaceList := &corev1.NamespaceList{}
		err := s.List(ctx, namespaceList, client.Limit(s.ScanBatchSize), client.Continue(continueToken))
		if err != nil {
			return err
		}

		for _, namespace := range namespaceList.Items {
			if err := s.processNamespace(ctx, namespace); err != nil {
				s.Log.Error(err, "Failed to process namespace", "namespace", namespace.Name)
			}
		}

		if namespaceList.Continue == "" {
			break
		}
		continueToken = namespaceList.Continue
	}
	return nil
}

func (s *NamespaceScanner) processNamespace(ctx context.Context, namespace corev1.Namespace) error {
	log := s.Log.WithValues("namespace", namespace.Name)

	status, ok := namespace.Labels[constants.StatusLabel]
	if !ok {
		// If label doesn't exist, ensure no quota is present.
		return s.handleUnlock(ctx, &namespace)
	}

	// Check for lock expiration
	if unlockTimestampStr, ok := namespace.Annotations[constants.UnlockTimestampLabel]; ok {
		unlockTime, err := time.Parse(time.RFC3339, unlockTimestampStr)
		if err == nil && time.Now().After(unlockTime) {
			if status == constants.LockedStatus {
				return s.handleLockExpiration(ctx, &namespace)
			}
		}
	}

	switch status {
	case constants.LockedStatus:
		log.Info("namespace is locked, handling lock")
		return s.handleLock(ctx, &namespace)
	case constants.ActiveStatus:
		log.Info("namespace is active, handling unlock", "hasUnlockTimestamp", namespace.Annotations != nil && namespace.Annotations[constants.UnlockTimestampLabel] != "")
		return s.handleUnlock(ctx, &namespace)
	}

	return nil
}

func (s *NamespaceScanner) handleLock(ctx context.Context, namespace *corev1.Namespace) error {
	log := s.Log.WithValues("namespace", namespace.Name)

	// Ensure unlock timestamp exists
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}
	_, ok := namespace.Annotations[constants.UnlockTimestampLabel]
	if !ok {
		unlockTime := time.Now().Add(s.LockDuration)
		namespace.Annotations[constants.UnlockTimestampLabel] = unlockTime.Format(time.RFC3339)
		if err := s.Update(ctx, namespace); err != nil {
			log.Error(err, "unable to update namespace with unlock timestamp")
			return err
		}
	}

	// Create ResourceQuota if it doesn't exist
	rq := utils.CreateResourceQuota(namespace.Name, false)
	log.Info("creating ResourceQuota")
	if err := s.Create(ctx, rq); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("ResourceQuota already exists")
		} else {
			log.Error(err, "unable to create ResourceQuota")
			return err
		}
	}

	// Scale down deployments
	var deployments appsv1.DeploymentList
	if err := s.List(ctx, &deployments, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list deployments")
		return err
	}

	for _, deployment := range deployments.Items {
		if deployment.Annotations == nil {
			deployment.Annotations = make(map[string]string)
		}
		if *deployment.Spec.Replicas != 0 {
			log.Info("scaling down deployment", "deployment", deployment.Name)
			deployment.Annotations[constants.OriginalReplicasAnnotation] = strconv.Itoa(int(*deployment.Spec.Replicas))
			*deployment.Spec.Replicas = 0
			if err := s.Update(ctx, &deployment); err != nil {
				if errors.IsConflict(err) {
					log.Info("deployment has been modified, requeueing", "deployment", deployment.Name)
					return nil
				}
				log.Error(err, "unable to scale down deployment", "deployment", deployment.Name)
				return err
			}
		}
	}

	// Scale down statefulsets
	var statefulsets appsv1.StatefulSetList
	if err := s.List(ctx, &statefulsets, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list statefulsets")
		return err
	}

	for _, statefulset := range statefulsets.Items {
		if statefulset.Annotations == nil {
			statefulset.Annotations = make(map[string]string)
		}
		if *statefulset.Spec.Replicas != 0 {
			log.Info("scaling down statefulset", "statefulset", statefulset.Name)
			statefulset.Annotations[constants.OriginalReplicasAnnotation] = strconv.Itoa(int(*statefulset.Spec.Replicas))
			*statefulset.Spec.Replicas = 0
			if err := s.Update(ctx, &statefulset); err != nil {
				if errors.IsConflict(err) {
					log.Info("statefulset has been modified, requeueing", "statefulset", statefulset.Name)
					return nil
				}
				log.Error(err, "unable to scale down statefulset", "statefulset", statefulset.Name)
				return err
			}
		}
	}

	// Scale down replicasets
	var replicasets appsv1.ReplicaSetList
	if err := s.List(ctx, &replicasets, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list replicasets")
		return err
	}

	for _, replicaset := range replicasets.Items {
		if replicaset.Annotations == nil {
			replicaset.Annotations = make(map[string]string)
		}
		if *replicaset.Spec.Replicas != 0 {
			log.Info("scaling down replicaset", "replicaset", replicaset.Name)
			replicaset.Annotations[constants.OriginalReplicasAnnotation] = strconv.Itoa(int(*replicaset.Spec.Replicas))
			*replicaset.Spec.Replicas = 0
			if err := s.Update(ctx, &replicaset); err != nil {
				if errors.IsConflict(err) {
					log.Info("replicaset has been modified, requeueing", "replicaset", replicaset.Name)
					return nil
				}
				log.Error(err, "unable to scale down replicaset", "replicaset", replicaset.Name)
				return err
			}
		}
	}

	// Scale down replicationcontrollers
	var rcs corev1.ReplicationControllerList
	if err := s.List(ctx, &rcs, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list replicationcontrollers")
		return err
	}

	for _, rc := range rcs.Items {
		if rc.Annotations == nil {
			rc.Annotations = make(map[string]string)
		}
		if *rc.Spec.Replicas != 0 {
			log.Info("scaling down replicationcontroller", "rc", rc.Name)
			rc.Annotations[constants.OriginalReplicasAnnotation] = strconv.Itoa(int(*rc.Spec.Replicas))
			*rc.Spec.Replicas = 0
			if err := s.Update(ctx, &rc); err != nil {
				if errors.IsConflict(err) {
					log.Info("replicationcontroller has been modified, requeueing", "rc", rc.Name)
					return nil
				}
				log.Error(err, "unable to scale down replicationcontroller", "rc", rc.Name)
				return err
			}
		}
	}

	// Suspend cronjobs
	var cronjobs batchv1.CronJobList
	if err := s.List(ctx, &cronjobs, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list cronjobs")
		return err
	}

	for _, cronjob := range cronjobs.Items {
		if cronjob.Annotations == nil {
			cronjob.Annotations = make(map[string]string)
		}
		if cronjob.Spec.Suspend != nil && !*cronjob.Spec.Suspend {
			log.Info("suspending cronjob", "cronjob", cronjob.Name)
			cronjob.Annotations[constants.OriginalSuspendAnnotation] = strconv.FormatBool(*cronjob.Spec.Suspend)
			*cronjob.Spec.Suspend = true
			if err := s.Update(ctx, &cronjob); err != nil {
				if errors.IsConflict(err) {
					log.Info("cronjob has been modified, requeueing", "cronjob", cronjob.Name)
					return nil
				}
				log.Error(err, "unable to suspend cronjob", "cronjob", cronjob.Name)
				return err
			}
		}
	}

	// Delete pods
	var pods corev1.PodList
	if err := s.List(ctx, &pods, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list pods")
		return err
	}

	for _, pod := range pods.Items {
		isStandalone := true
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "ReplicaSet" || owner.Kind == "StatefulSet" || owner.Kind == "ReplicationController" || owner.Kind == "Job" {
				isStandalone = false
				break
			}
		}

		if isStandalone {
			log.Info("deleting pod", "pod", pod.Name)
			if err := s.Delete(ctx, &pod); err != nil {
				log.Error(err, "unable to delete pod", "pod", pod.Name)
				return err
			}
		}
	}

	return nil
}

func (s *NamespaceScanner) handleUnlock(ctx context.Context, namespace *corev1.Namespace) error {
	log := s.Log.WithValues("namespace", namespace.Name)

	// Delete ResourceQuota if it exists
	var resourceQuota corev1.ResourceQuota
	if err := s.Get(ctx, client.ObjectKey{Name: constants.ResourceQuotaName, Namespace: namespace.Name}, &resourceQuota); err == nil {
		log.Info("deleting ResourceQuota")
		if err := s.Delete(ctx, &resourceQuota); err != nil {
			log.Error(err, "unable to delete ResourceQuota")
			return err
		}
	}

	// Scale up deployments
	var deployments appsv1.DeploymentList
	if err := s.List(ctx, &deployments, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list deployments")
		return err
	}

	for _, deployment := range deployments.Items {
		if deployment.Annotations != nil {
			if replicasStr, ok := deployment.Annotations[constants.OriginalReplicasAnnotation]; ok {
				replicas, err := strconv.Atoi(replicasStr)
				if err != nil {
					log.Error(err, "unable to parse original replicas annotation for deployment", "deployment", deployment.Name)
					continue
				}
				log.Info("scaling up deployment", "deployment", deployment.Name)
				*deployment.Spec.Replicas = int32(replicas)
				delete(deployment.Annotations, constants.OriginalReplicasAnnotation)
				if err := s.Update(ctx, &deployment); err != nil {
					if errors.IsConflict(err) {
						log.Info("deployment has been modified, requeueing", "deployment", deployment.Name)
						return nil
					}
					log.Error(err, "unable to scale up deployment", "deployment", deployment.Name)
					return err
				}
			}
		}
	}

	// Scale up statefulsets
	var statefulsets appsv1.StatefulSetList
	if err := s.List(ctx, &statefulsets, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list statefulsets")
		return err
	}

	for _, statefulset := range statefulsets.Items {
		if statefulset.Annotations != nil {
			if replicasStr, ok := statefulset.Annotations[constants.OriginalReplicasAnnotation]; ok {
				replicas, err := strconv.Atoi(replicasStr)
				if err != nil {
					log.Error(err, "unable to parse original replicas annotation for statefulset", "statefulset", statefulset.Name)
					continue
				}
				log.Info("scaling up statefulset", "statefulset", statefulset.Name)
				*statefulset.Spec.Replicas = int32(replicas)
				delete(statefulset.Annotations, constants.OriginalReplicasAnnotation)
				if err := s.Update(ctx, &statefulset); err != nil {
					if errors.IsConflict(err) {
						log.Info("statefulset has been modified, requeueing", "statefulset", statefulset.Name)
						return nil
					}
					log.Error(err, "unable to scale up statefulset", "statefulset", statefulset.Name)
					return err
				}
			}
		}
	}

	// Scale up replicasets
	var replicasets appsv1.ReplicaSetList
	if err := s.List(ctx, &replicasets, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list replicasets")
		return err
	}

	for _, replicaset := range replicasets.Items {
		if replicaset.Annotations != nil {
			if replicasStr, ok := replicaset.Annotations[constants.OriginalReplicasAnnotation]; ok {
				replicas, err := strconv.Atoi(replicasStr)
				if err != nil {
					log.Error(err, "unable to parse original replicas annotation for replicaset", "replicaset", replicaset.Name)
					continue
				}
				log.Info("scaling up replicaset", "replicaset", replicaset.Name)
				*replicaset.Spec.Replicas = int32(replicas)
				delete(replicaset.Annotations, constants.OriginalReplicasAnnotation)
				if err := s.Update(ctx, &replicaset); err != nil {
					if errors.IsConflict(err) {
						log.Info("replicaset has been modified, requeueing", "replicaset", replicaset.Name)
						return nil
					}
					log.Error(err, "unable to scale up replicaset", "replicaset", replicaset.Name)
					return err
				}
			}
		}
	}

	// Scale up replicationcontrollers
	var rcs corev1.ReplicationControllerList
	if err := s.List(ctx, &rcs, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list replicationcontrollers")
		return err
	}

	for _, rc := range rcs.Items {
		if rc.Annotations != nil {
			if replicasStr, ok := rc.Annotations[constants.OriginalReplicasAnnotation]; ok {
				replicas, err := strconv.Atoi(replicasStr)
				if err != nil {
					log.Error(err, "unable to parse original replicas annotation for replicationcontroller", "rc", rc.Name)
					continue
				}
				log.Info("scaling up replicationcontroller", "rc", rc.Name)
				*rc.Spec.Replicas = int32(replicas)
				delete(rc.Annotations, constants.OriginalReplicasAnnotation)
				if err := s.Update(ctx, &rc); err != nil {
					if errors.IsConflict(err) {
						log.Info("replicationcontroller has been modified, requeueing", "rc", rc.Name)
						return nil
					}
					log.Error(err, "unable to scale up replicationcontroller", "rc", rc.Name)
					return err
				}
			}
		}
	}

	// Unsuspend cronjobs
	var cronjobs batchv1.CronJobList
	if err := s.List(ctx, &cronjobs, client.InNamespace(namespace.Name)); err != nil {
		log.Error(err, "unable to list cronjobs")
		return err
	}

	for _, cronjob := range cronjobs.Items {
		if cronjob.Annotations != nil {
			if suspendStr, ok := cronjob.Annotations[constants.OriginalSuspendAnnotation]; ok {
				suspend, err := strconv.ParseBool(suspendStr)
				if err != nil {
					log.Error(err, "unable to parse original suspend annotation for cronjob", "cronjob", cronjob.Name)
					continue
				}
				log.Info("unsuspending cronjob", "cronjob", cronjob.Name)
				*cronjob.Spec.Suspend = suspend
				delete(cronjob.Annotations, constants.OriginalSuspendAnnotation)
				if err := s.Update(ctx, &cronjob); err != nil {
					if errors.IsConflict(err) {
						log.Info("cronjob has been modified, requeueing", "cronjob", cronjob.Name)
						return nil
					}
					log.Error(err, "unable to unsuspend cronjob", "cronjob", cronjob.Name)
					return err
				}
			}
		}
	}

	// Remove timestamp annotation
	if namespace.Annotations != nil {
		if _, exists := namespace.Annotations[constants.UnlockTimestampLabel]; exists {
			log.Info("removing unlock-timestamp annotation")
			delete(namespace.Annotations, constants.UnlockTimestampLabel)
			if err := s.Update(ctx, namespace); err != nil {
				log.Error(err, "unable to remove timestamp annotation")
				return err
			}
			log.Info("successfully removed unlock-timestamp annotation")
		} else {
			log.Info("no unlock-timestamp annotation found, nothing to clean")
		}
	}

	return nil
}

func (s *NamespaceScanner) handleLockExpiration(ctx context.Context, namespace *corev1.Namespace) error {
	log := s.Log.WithValues("namespace", namespace.Name)
	log.Info("Lock expired, deleting namespace")

	if err := s.Delete(ctx, namespace); err != nil {
		log.Error(err, "unable to delete namespace")
		return err
	}

	return nil
}
