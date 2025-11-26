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

// Package cmd implements the kubectl-block subcommands.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
)

// NewLockCommand creates the lock command
func NewLockCommand(kubeConfig clientcmd.ClientConfig) *cobra.Command {
	opts := NewCommandOptions(kubeConfig)

	cmd := &cobra.Command{
		Use:   "lock <namespace-name>",
		Short: "Lock a namespace or multiple namespaces",
		Long: `Lock one or more namespaces by adding the 'clawcloud.run/status=locked' label.
This will scale down all workloads in the namespace and prevent new resources
from being created until the namespace is unlocked or the lock expires.`,
		Example: `
  # Lock a single namespace
  kubectl block lock my-namespace

  # Lock with duration and reason
  kubectl block lock my-namespace --duration=24h --reason="Maintenance"

  # Lock multiple namespaces by selector
  kubectl block lock --selector=environment=dev

  # Lock all namespaces in a file
  kubectl block lock --file=namespaces.txt

  # Lock all namespaces (use with caution)
  kubectl block lock --all

  # Dry run to see what would be locked
  kubectl block lock my-namespace --dry-run
`,
		RunE: opts.runLock,
	}

	// Add flags
	cmd.Flags().StringP(&opts.namespace, "namespace", "n", "", "The namespace to create BlockRequest in (default: current namespace)")
	cmd.Flags().StringVarP(&opts.selector, "selector", "l", "", "Label selector to identify namespaces to lock")
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "File containing list of namespaces to lock (one per line)")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Lock all namespaces (excluding system namespaces)")

	AddCommonFlags(cmd, opts)

	return cmd
}

func (o *CommandOptions) runLock(cmd *cobra.Command, args []string) error {
	// Initialize
	if err := o.Init(); err != nil {
		return err
	}

	// Determine the list of namespaces to lock
	var namespaces []string
	var err error

	switch {
	case len(args) > 0:
		// Namespace name directly specified
		namespaces = args
	case opts.all:
		// Lock all namespaces
		namespaces, err = o.getAllNamespaces()
	case opts.selector != "":
		// Lock by selector
		namespaces, err = o.getNamespacesBySelector(opts.selector)
	case opts.file != "":
		// Read from file
		namespaces, err = ReadNamespacesFromFile(opts.file)
	default:
		return fmt.Errorf("you must specify a namespace name, or use --selector, --file, or --all")
	}

	if err != nil {
		return err
	}

	if len(namespaces) == 0 {
		fmt.Println("ℹ️  No namespaces found to lock")
		return nil
	}

	// Validate namespaces
	for _, ns := range namespaces {
		if err := o.ValidateNamespace(ns); err != nil {
			return fmt.Errorf("invalid namespace %s: %v", ns, err)
		}
	}

	// Display operation plan
	fmt.Printf("🔒 Planning to lock %d namespace(s):\n", len(namespaces))
	for _, ns := range namespaces {
		status, _ := o.GetNamespaceStatus(ns)
		statusIcon := "🔓"
		if status == constants.LockedStatus {
			statusIcon = "🔒"
		}
		fmt.Printf("  %s %s (current: %s)\n", statusIcon, ns, status)
	}

	// Confirm operation
	if !opts.force && !opts.dryRun {
		fmt.Printf("\n⚠️  This will scale down all workloads in the listed namespaces.\n")
		fmt.Printf("Duration: %s\n", FormatDuration(o.duration))
		fmt.Printf("Reason: %s\n", opts.reason)
		fmt.Print("\nDo you want to continue? [y/N]: ")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))

		if response != "y" && response != "yes" {
			fmt.Println("❌ Operation cancelled")
			return nil
		}
	}

	// Execute lock operation
	fmt.Printf("\n🚀 Starting lock operation...\n")
	successCount := 0
	failureCount := 0

	for _, ns := range namespaces {
		if err := o.lockNamespace(ns); err != nil {
			fmt.Printf("❌ Failed to lock namespace %s: %v\n", ns, err)
			failureCount++
		} else {
			fmt.Printf("✅ Successfully locked namespace %s\n", ns)
			successCount++
		}
	}

	// Display results
	fmt.Printf("\n📊 Lock operation completed:\n")
	fmt.Printf("  ✅ Success: %d\n", successCount)
	fmt.Printf("  ❌ Failed: %d\n", failureCount)

	if failureCount > 0 {
		return fmt.Errorf("%d namespace(s) failed to lock", failureCount)
	}

	return nil
}

// lockNamespace locks a single namespace
func (o *CommandOptions) lockNamespace(namespace string) error {
	// Check current status
	currentStatus, err := o.GetNamespaceStatus(namespace)
	if err != nil {
		return err
	}

	if currentStatus == constants.LockedStatus {
		if !o.force {
			fmt.Printf("⚠️  Namespace %s is already locked, skipping...\n", namespace)
			return nil
		}
		fmt.Printf("🔄 Namespace %s is already locked, re-locking...\n", namespace)
	}

	// Method 1: Lock directly through labels
	if err := o.updateNamespaceForLock(namespace); err != nil {
		return err
	}

	return nil
}

// updateNamespaceForLock updates namespace for locking
func (o *CommandOptions) updateNamespaceForLock(namespace string) error {
	ctx := context.TODO()

	// Get namespace
	ns, err := o.GetNamespace(namespace)
	if err != nil {
		return err
	}

	// Update labels
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels[constants.StatusLabel] = constants.LockedStatus

	// Update annotations
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}

	// Set unlock timestamp
	if o.duration > 0 {
		unlockTime := time.Now().Add(o.duration)
		ns.Annotations[constants.UnlockTimestampLabel] = unlockTime.Format(time.RFC3339)
	}

	// Add operation reason
	if opts.reason != "" {
		ns.Annotations["clawcloud.run/lock-reason"] = opts.reason
		ns.Annotations["clawcloud.run/lock-operator"] = "kubectl-block"
	}

	// Update namespace
	if o.dryRun {
		fmt.Printf("[DRY-RUN] Would update namespace %s\n", namespace)
		return nil
	}

	_, err = o.client.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	o.LogVerbose("Successfully locked namespace %s", namespace)
	return nil
}

// getAllNamespaces gets all non-system namespaces
func (o *CommandOptions) getAllNamespaces() ([]string, error) {
	ctx := context.TODO()
	namespaces, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
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

// getNamespacesBySelector gets namespaces by label selector
func (o *CommandOptions) getNamespacesBySelector(selector string) ([]string, error) {
	ctx := context.TODO()
	options := metav1.ListOptions{
		LabelSelector: selector,
	}

	namespaces, err := o.client.CoreV1().Namespaces().List(ctx, options)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ns := range namespaces.Items {
		result = append(result, ns.Name)
	}

	return result, nil
}
