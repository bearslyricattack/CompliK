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

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig string
	namespace  string
	duration   time.Duration
	reason     string
	force      bool
	dryRun     bool
	verbose    bool
	selector   string
	all        bool
	lockedOnly bool
	allLocked  bool
	details    bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "kubectl-block",
		Short: "Block controller CLI for managing namespace lifecycle",
		Long: `kubectl-block is a CLI tool for managing Kubernetes namespace lifecycle
through the block controller. It provides commands to lock, unlock, and monitor
namespaces with ease.`,
		Version: "v0.2.0-alpha",
	}

	rootCmd.AddCommand(lockCommand())
	rootCmd.AddCommand(unlockCommand())
	rootCmd.AddCommand(statusCommand())

	// 全局参数
	rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "", "", "Path to the kubeconfig file")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "The namespace to operate in")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "If true, only print the object that would be sent")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func lockCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock <namespace>",
		Short: "Lock a namespace",
		Long:  `Lock a namespace by adding the 'clawcloud.run/status=locked' label.`,
		Example: `
  kubectl block lock my-namespace
  kubectl block lock my-namespace --duration=24h --reason="Maintenance"
  kubectl block lock --selector=environment=dev
  kubectl block lock --all`,
		RunE: runLock,
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 24*time.Hour, "Duration for lock")
	cmd.Flags().StringVarP(&reason, "reason", "r", "Manual operation via kubectl-block", "Reason for the operation")
	cmd.Flags().BoolVar(&force, "force", false, "Force the operation without confirmation")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "The namespace to lock")
	cmd.Flags().StringVar(&selector, "selector", "", "Label selector to identify namespaces to lock")
	cmd.Flags().BoolVar(&all, "all", false, "Lock all namespaces (excluding system namespaces)")

	return cmd
}

func unlockCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock <namespace>",
		Short: "Unlock a namespace",
		Long:  `Unlock a namespace by changing the 'clawcloud.run/status' label to 'active'.`,
		Example: `
  kubectl block unlock my-namespace
  kubectl block unlock my-namespace --reason="Maintenance completed"
  kubectl block unlock --all-locked
  kubectl block unlock --selector=environment=dev`,
		RunE: runUnlock,
	}

	cmd.Flags().StringVarP(&reason, "reason", "r", "Manual operation via kubectl-block", "Reason for the operation")
	cmd.Flags().BoolVar(&force, "force", false, "Force the operation without confirmation")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "The namespace to unlock")
	cmd.Flags().StringVar(&selector, "selector", "", "Label selector to identify namespaces to unlock")
	cmd.Flags().BoolVar(&allLocked, "all-locked", false, "Unlock all currently locked namespaces")

	return cmd
}

func statusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [namespace]",
		Short: "Show the status of namespaces",
		Long:  `Display the current status of namespaces, including lock status and remaining lock time.`,
		Example: `
  kubectl block status my-namespace
  kubectl block status --all
  kubectl block status --locked-only`,
		RunE: runStatus,
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "The namespace to check")
	cmd.Flags().BoolVar(&all, "all", false, "Show status of all namespaces")
	cmd.Flags().BoolVar(&lockedOnly, "locked-only", false, "Show only locked namespaces")
	cmd.Flags().BoolVarP(&details, "details", "D", false, "Show detailed information")

	return cmd
}

func runLock(cmd *cobra.Command, args []string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	var namespaces []string
	switch {
	case len(args) > 0:
		namespaces = args
	case all:
		namespaces, err = getAllNamespaces(clientset)
	case selector != "":
		namespaces, err = getNamespacesBySelector(clientset, selector)
	default:
		return fmt.Errorf("you must specify a namespace name, or use --selector or --all")
	}

	if err != nil {
		return err
	}

	fmt.Printf("🔒 Planning to lock %d namespace(s):\n", len(namespaces))
	for _, ns := range namespaces {
		status, _ := getNamespaceStatus(clientset, ns)
		statusIcon := "🔓"
		if status == "locked" {
			statusIcon = "🔒"
		}
		fmt.Printf("  %s %s (current: %s)\n", statusIcon, ns, status)
	}

	if !force && !dryRun {
		fmt.Printf("\n⚠️  This will scale down all workloads in the listed namespaces.\n")
		fmt.Printf("Duration: %s\n", formatDuration(duration))
		fmt.Printf("Reason: %s\n", reason)
		fmt.Print("\nDo you want to continue? [y/N]: ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("❌ Operation cancelled")
			return nil
		}
	}

	fmt.Printf("\n🚀 Starting lock operation...\n")
	successCount := 0
	failureCount := 0

	for _, ns := range namespaces {
		if err := lockNamespace(clientset, ns); err != nil {
			fmt.Printf("❌ Failed to lock namespace %s: %v\n", ns, err)
			failureCount++
		} else {
			fmt.Printf("✅ Successfully locked namespace %s\n", ns)
			successCount++
		}
	}

	fmt.Printf("\n📊 Lock operation completed:\n")
	fmt.Printf("  ✅ Success: %d\n", successCount)
	fmt.Printf("  ❌ Failed: %d\n", failureCount)

	if failureCount > 0 {
		return fmt.Errorf("%d namespace(s) failed to lock", failureCount)
	}

	return nil
}

func runUnlock(cmd *cobra.Command, args []string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	var namespaces []string
	switch {
	case len(args) > 0:
		namespaces = args
	case allLocked:
		namespaces, err = getLockedNamespaces(clientset)
	default:
		return fmt.Errorf("you must specify a namespace name, or use --all-locked")
	}

	if err != nil {
		return err
	}

	fmt.Printf("🔓 Planning to unlock %d namespace(s):\n", len(namespaces))
	for _, ns := range namespaces {
		fmt.Printf("  🔒 %s\n", ns)
	}

	if !force && !dryRun {
		fmt.Printf("\nDo you want to continue? [y/N]: ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("❌ Operation cancelled")
			return nil
		}
	}

	fmt.Printf("\n🚀 Starting unlock operation...\n")
	successCount := 0
	failureCount := 0

	for _, ns := range namespaces {
		if err := unlockNamespace(clientset, ns); err != nil {
			fmt.Printf("❌ Failed to unlock namespace %s: %v\n", ns, err)
			failureCount++
		} else {
			fmt.Printf("✅ Successfully unlocked namespace %s\n", ns)
			successCount++
		}
	}

	fmt.Printf("\n📊 Unlock operation completed:\n")
	fmt.Printf("  ✅ Success: %d\n", successCount)
	fmt.Printf("  ❌ Failed: %d\n", failureCount)

	if failureCount > 0 {
		return fmt.Errorf("%d namespace(s) failed to unlock", failureCount)
	}

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	var namespaces []string
	switch {
	case len(args) > 0:
		namespaces = args
	case all:
		namespaces, err = getAllNamespaces(clientset)
	case lockedOnly:
		namespaces, err = getLockedNamespaces(clientset)
	default:
		return fmt.Errorf("you must specify a namespace name, or use --all or --locked-only")
	}

	if err != nil {
		return err
	}

	fmt.Printf("📊 Namespace Status Report\n\n")
	fmt.Printf("NAMESPACE\tSTATUS\tREMAINING\tWORKLOADS\n")

	for _, ns := range namespaces {
		status, err := getNamespaceStatus(clientset, ns)
		if err != nil {
			fmt.Printf("%s\tError\t\t-\n", ns)
			continue
		}

		statusIcon := "🔓"
		if status == "locked" {
			statusIcon = "🔒"
		}

		remaining := getRemainingTime(clientset, ns)
		if remaining == "" {
			remaining = "-"
		}

		workloads := countWorkloads(clientset, ns)

		fmt.Printf("%s\t%s %s\t%s\t%d\n", ns, statusIcon, status, remaining, workloads)
	}

	return nil
}

// Helper functions
func lockNamespace(clientset *kubernetes.Clientset, namespace string) error {
	if dryRun {
		fmt.Printf("[DRY-RUN] Would lock namespace %s\n", namespace)
		return nil
	}

	ctx := context.TODO()
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels["clawcloud.run/status"] = "locked"

	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	if duration > 0 {
		unlockTime := time.Now().Add(duration)
		ns.Annotations["clawcloud.run/unlock-timestamp"] = unlockTime.Format(time.RFC3339)
	}
	ns.Annotations["clawcloud.run/lock-reason"] = reason

	_, err = clientset.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	return err
}

func unlockNamespace(clientset *kubernetes.Clientset, namespace string) error {
	if dryRun {
		fmt.Printf("[DRY-RUN] Would unlock namespace %s\n", namespace)
		return nil
	}

	ctx := context.TODO()
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels["clawcloud.run/status"] = "active"

	if ns.Annotations != nil {
		delete(ns.Annotations, "clawcloud.run/unlock-timestamp")
		delete(ns.Annotations, "clawcloud.run/lock-reason")
	}

	_, err = clientset.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	return err
}

func getNamespaceStatus(clientset *kubernetes.Clientset, namespace string) (string, error) {
	ctx := context.TODO()
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	status := ns.Labels["clawcloud.run/status"]
	if status == "" {
		status = "active"
	}

	return status, nil
}

func getRemainingTime(clientset *kubernetes.Clientset, namespace string) string {
	ctx := context.TODO()
	ns, err := clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return ""
	}

	if ns.Annotations == nil {
		return ""
	}

	unlockTimeStr := ns.Annotations["clawcloud.run/unlock-timestamp"]
	if unlockTimeStr == "" {
		return ""
	}

	unlockTime, err := time.Parse(time.RFC3339, unlockTimeStr)
	if err != nil {
		return ""
	}

	if time.Now().After(unlockTime) {
		return "expired"
	}

	remaining := unlockTime.Sub(time.Now())
	if remaining < time.Minute {
		return fmt.Sprintf("%ds", int(remaining.Seconds()))
	} else if remaining < time.Hour {
		return fmt.Sprintf("%dm", int(remaining.Minutes()))
	} else if remaining < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(remaining.Hours()), int(remaining.Minutes())%60)
	} else {
		days := int(remaining.Hours()) / 24
		hours := int(remaining.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}

func getAllNamespaces(clientset *kubernetes.Clientset) ([]string, error) {
	ctx := context.TODO()
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []string
	systemNamespaces := map[string]bool{
		"kube-system":     true,
		"kube-public":     true,
		"kube-node-lease": true,
		"default":         true,
		"block-system":    true,
	}

	for _, ns := range namespaces.Items {
		if !systemNamespaces[ns.Name] {
			result = append(result, ns.Name)
		}
	}

	return result, nil
}

func getLockedNamespaces(clientset *kubernetes.Clientset) ([]string, error) {
	ctx := context.TODO()
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ns := range namespaces.Items {
		if ns.Labels["clawcloud.run/status"] == "locked" {
			result = append(result, ns.Name)
		}
	}

	return result, nil
}

func getNamespacesBySelector(clientset *kubernetes.Clientset, selector string) ([]string, error) {
	ctx := context.TODO()
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ns := range namespaces.Items {
		result = append(result, ns.Name)
	}

	return result, nil
}

func countWorkloads(clientset *kubernetes.Clientset, namespace string) int {
	ctx := context.TODO()
	count := 0

	// 简化的工作负载计数
	deployments, _ := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	count += len(deployments.Items)

	statefulSets, _ := clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	count += len(statefulSets.Items)

	return count
}

func formatDuration(duration time.Duration) string {
	if duration == 0 {
		return "permanent"
	}

	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24

	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dh", hours)
}
