/*
Copyright 2025 gitlayzer.

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

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
)

// NewCleanupCommand 创建 cleanup 命令
func NewCleanupCommand(kubeConfig clientcmd.ClientConfig) *cobra.Command {
	opts := NewCommandOptions(kubeConfig)

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up expired or orphaned resources",
		Long: `Clean up expired locks, orphaned BlockRequests, and other resources
that are no longer needed. This helps maintain a clean cluster state.`,
		Example: `
  # Clean up expired locks
  kubectl block cleanup --expired-only

  # Clean up orphaned BlockRequests
  kubectl block cleanup --orphaned-requests

  # Clean up expired annotations
  kubectl block cleanup --annotations

  # Clean up everything (use with caution)
  kubectl block cleanup --all

  # Dry run to see what would be cleaned
  kubectl block cleanup --dry-run

  # Force cleanup without confirmation
  kubectl block cleanup --force
`,
		RunE: opts.runCleanup,
	}

	// 添加参数
	cmd.Flags().BoolVar(&opts.expiredOnly, "expired-only", false, "Clean up only expired locks")
	cmd.Flags().BoolVar(&opts.orphanedRequests, "orphaned-requests", false, "Clean up orphaned BlockRequests")
	cmd.Flags().BoolVar(&opts.cleanupAnnotations, "annotations", false, "Clean up orphaned annotations")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Clean up all cleanup targets (use with caution)")
	cmd.Flags().DurationVar(&opts.olderThan, "older-than", 0, "Clean up resources older than this duration")

	AddCommonFlags(cmd, opts)

	return cmd
}

var (
	expiredOnly        bool
	orphanedRequests   bool
	cleanupAnnotations bool
	olderThan          time.Duration
)

func (o *CommandOptions) runCleanup(cmd *cobra.Command, args []string) error {
	// 初始化
	if err := o.Init(); err != nil {
		return err
	}

	// 确定清理目标
	var targets []string
	switch {
	case o.expiredOnly:
		targets = []string{"expired-locks"}
	case o.orphanedRequests:
		targets = []string{"orphaned-requests"}
	case o.cleanupAnnotations:
		targets = []string{"orphaned-annotations"}
	case o.all:
		targets = []string{"expired-locks", "orphaned-requests", "orphaned-annotations"}
	default:
		return fmt.Errorf("you must specify cleanup targets with flags")
	}

	// 显示清理计划
	fmt.Printf("🧹 Planning cleanup operation:\n")
	for _, target := range targets {
		fmt.Printf("  - %s\n", target)
	}
	if o.olderThan > 0 {
		fmt.Printf("  - Resources older than: %s\n", o.olderThan.String())
	}

	// 确认操作
	if !opts.force && !opts.dryRun {
		fmt.Printf("\n⚠️  This will permanently remove the selected resources.\n")
		fmt.Print("Do you want to continue? [y/N]: ")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))

		if response != "y" && response != "yes" {
			fmt.Println("❌ Operation cancelled")
			return nil
		}
	}

	// 执行清理
	fmt.Printf("\n🚀 Starting cleanup operation...\n")
	successCount := 0
	failureCount := 0

	for _, target := range targets {
		if err := o.cleanupTarget(target); err != nil {
			fmt.Printf("❌ Failed to clean up %s: %v\n", target, err)
			failureCount++
		} else {
			fmt.Printf("✅ Successfully cleaned up %s\n", target)
			successCount++
		}
	}

	// 显示结果
	fmt.Printf("\n📊 Cleanup operation completed:\n")
	fmt.Printf("  ✅ Success: %d\n", successCount)
	fmt.Printf("  ❌ Failed: %d\n", failureCount)

	if failureCount > 0 {
		return fmt.Errorf("%d cleanup target(s) failed", failureCount)
	}

	return nil
}

// cleanupTarget 清理特定目标
func (o *CommandOptions) cleanupTarget(target string) error {
	switch target {
	case "expired-locks":
		return o.cleanupExpiredLocks()
	case "orphaned-requests":
		return o.cleanupOrphanedRequests()
	case "orphaned-annotations":
		return o.cleanupOrphanedAnnotations()
	default:
		return fmt.Errorf("unknown cleanup target: %s", target)
	}
}

// cleanupExpiredLocks 清理过期的锁
func (o *CommandOptions) cleanupExpiredLocks() error {
	ctx := context.TODO()

	// 获取所有 namespace
	namespaces, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	expiredCount := 0
	for _, ns := range namespaces.Items {
		// 检查是否有过期的时间戳
		if ns.Annotations != nil {
			if unlockTimeStr := ns.Annotations[constants.UnlockTimestampLabel]; unlockTimeStr != "" {
				if unlockTime, err := time.Parse(time.RFC3339, unlockTimeStr); err == nil {
					if time.Now().After(unlockTime) {
						// 检查年龄限制
						if o.olderThan > 0 && time.Since(ns.CreationTimestamp.Time) < o.olderThan {
							continue
						}

						// 清理过期锁
						if err := o.cleanupExpiredLock(&ns); err != nil {
							o.LogError(err, "Failed to clean up expired lock for namespace %s", ns.Name)
							continue
						}
						expiredCount++
					}
				}
			}
		}
	}

	if expiredCount > 0 {
		fmt.Printf("  Cleaned up %d expired locks\n", expiredCount)
	} else {
		fmt.Printf("  No expired locks found\n")
	}

	return nil
}

// cleanupExpiredLock 清理单个 namespace 的过期锁
func (o *CommandOptions) cleanupExpiredLock(ns *corev1.Namespace) error {
	if o.dryRun {
		fmt.Printf("[DRY-RUN] Would clean up expired lock for namespace %s\n", ns.Name)
		return nil
	}

	ctx := context.TODO()

	// 移除锁定标签
	if ns.Labels != nil && ns.Labels[constants.StatusLabel] == constants.LockedStatus {
		delete(ns.Labels, constants.StatusLabel)
	}

	// 清理注解
	if ns.Annotations != nil {
		delete(ns.Annotations, constants.UnlockTimestampLabel)
		delete(ns.Annotations, "clawcloud.run/lock-reason")
		delete(ns.Annotations, "clawcloud.run/lock-operator")
	}

	// 更新 namespace
	_, err := o.client.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	o.LogVerbose("Cleaned up expired lock for namespace %s", ns.Name)
	return nil
}

// cleanupOrphanedRequests 清理孤立的 BlockRequest
func (o *CommandOptions) cleanupOrphanedRequests() error {
	ctx := context.TODO()

	// 获取所有 BlockRequest
	requests, err := o.listBlockRequests()
	if err != nil {
		return err
	}

	orphanedCount := 0
	for _, req := range requests {
		isOrphaned := false

		// 检查目标 namespace 是否存在
		for _, targetNs := range req.Spec.NamespaceNames {
			_, err := o.client.CoreV1().Namespaces().Get(ctx, targetNs, metav1.GetOptions{})
			if err != nil {
				// Namespace 不存在，可能是孤立的 BlockRequest
				isOrphaned = true
				break
			}

			// 检查年龄限制
			if o.olderThan > 0 && time.Since(req.CreationTimestamp.Time) < o.olderThan {
				isOrphaned = false
				break
			}
		}

		if isOrphaned {
			if err := o.deleteBlockRequest(req); err != nil {
				o.LogError(err, "Failed to delete orphaned BlockRequest %s", req.Name)
				continue
			}
			orphanedCount++
		}
	}

	if orphanedCount > 0 {
		fmt.Printf("  Cleaned up %d orphaned BlockRequests\n", orphanedCount)
	} else {
		fmt.Printf("  No orphaned BlockRequests found\n")
	}

	return nil
}

// deleteBlockRequest 删除 BlockRequest
func (o *CommandOptions) deleteBlockRequest(req *blockv1.BlockRequest) error {
	if o.dryRun {
		fmt.Printf("[DRY-RUN] Would delete BlockRequest %s/%s\n", req.Namespace, req.Name)
		return nil
	}

	ctx := context.TODO()
	err := o.blockClient.Delete().
		Namespace(req.Namespace).
		Resource("blockrequests").
		Name(req.Name).
		Do(ctx).
		Error()

	if err != nil {
		return err
	}

	o.LogVerbose("Deleted orphaned BlockRequest %s/%s", req.Namespace, req.Name)
	return nil
}

// cleanupOrphanedAnnotations 清理孤立的注解
func (o *CommandOptions) cleanupOrphanedAnnotations() error {
	ctx := context.TODO()

	// 获取所有 namespace
	namespaces, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	annotationCount := 0
	for _, ns := range namespaces.Items {
		cleaned := 0

		// 检查状态
		status := ns.Labels[constants.StatusLabel]
		if status == "" {
			status = "active"
		}

		// 清理不匹配的注解
		if ns.Annotations != nil {
			// 如果状态是 active，不应该有锁定相关的注解
			if status == constants.ActiveStatus {
				if _, exists := ns.Annotations[constants.UnlockTimestampLabel]; exists {
					delete(ns.Annotations, constants.UnlockTimestampLabel)
					cleaned++
				}
				if _, exists := ns.Annotations["clawcloud.run/lock-reason"]; exists {
					delete(ns.Annotations, "clawcloud.run/lock-reason")
					cleaned++
				}
				if _, exists := ns.Annotations["clawcloud.run/lock-operator"]; exists {
					delete(ns.Annotations, "clawcloud.run/lock-operator")
					cleaned++
				}
			}

			// 如果状态是 locked，应该有解锁时间戳
			if status == constants.LockedStatus {
				if _, exists := ns.Annotations[constants.UnlockTimestampLabel]; !exists {
					// 添加默认解锁时间
					unlockTime := time.Now().Add(24 * time.Hour)
					ns.Annotations[constants.UnlockTimestampLabel] = unlockTime.Format(time.RFC3339)
					cleaned++
				}
			}
		}

		// 如果有清理操作，更新 namespace
		if cleaned > 0 {
			if o.dryRun {
				fmt.Printf("[DRY-RUN] Would clean up %d orphaned annotations in namespace %s\n", cleaned, ns.Name)
			} else {
				_, err := o.client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
				if err != nil {
					o.LogError(err, "Failed to update namespace %s for annotation cleanup", ns.Name)
					continue
				}
				annotationCount += cleaned
			}
		}
	}

	if annotationCount > 0 {
		fmt.Printf("  Cleaned up %d orphaned annotations\n", annotationCount)
	} else {
		fmt.Printf("  No orphaned annotations found\n")
	}

	return nil
}
