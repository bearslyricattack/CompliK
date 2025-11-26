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

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
)

// NewUnlockCommand creates the unlock command
func NewUnlockCommand(kubeConfig clientcmd.ClientConfig) *cobra.Command {
	opts := NewCommandOptions(kubeConfig)

	cmd := &cobra.Command{
		Use:   "unlock <namespace-name>",
		Short: "Unlock a namespace or multiple namespaces",
		Long: `Unlock one or more namespaces by changing the 'clawcloud.run/status' label to 'active'.
This will restore all workloads in the namespace to their original state and
remove the resource quota that was preventing new resources from being created.`,
		Example: `
  # Unlock a single namespace
  kubectl block unlock my-namespace

  # Unlock with reason
  kubectl block unlock my-namespace --reason="Maintenance completed"

  # Unlock multiple namespaces by selector
  kubectl block unlock --selector=environment=dev

  # Unlock all locked namespaces
  kubectl block unlock --all-locked

  # Unlock namespaces from a file
  kubectl block unlock --file=namespaces.txt

  # Force unlock without confirmation
  kubectl block unlock my-namespace --force

  # Dry run to see what would be unlocked
  kubectl block unlock my-namespace --dry-run
`,
		RunE: opts.runUnlock,
	}

	// Add flags
	cmd.Flags().StringP(&opts.namespace, "namespace", "n", "", "The namespace to create BlockRequest in (default: current namespace)")
	cmd.Flags().StringVarP(&opts.selector, "selector", "l", "", "Label selector to identify namespaces to unlock")
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "File containing list of namespaces to unlock (one per line)")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Unlock all namespaces (excluding system namespaces)")
	cmd.Flags().BoolVar(&opts.allLocked, "all-locked", false, "Unlock all currently locked namespaces")

	AddCommonFlags(cmd, opts)

	return cmd
}

func (o *CommandOptions) runUnlock(cmd *cobra.Command, args []string) error {
	// Initialize
	if err := o.Init(); err != nil {
		return err
	}

	// Determine the list of namespaces to unlock
	var namespaces []string
	var err error

	switch {
	case len(args) > 0:
		// Namespace name directly specified
		namespaces = args
	case opts.allLocked:
		// Unlock all locked namespaces
		namespaces, err = o.getLockedNamespaces()
	case opts.all:
		// Unlock all namespaces
		namespaces, err = o.getAllNamespaces()
	case opts.selector != "":
		// Unlock by selector
		namespaces, err = o.getNamespacesBySelector(opts.selector)
	case opts.file != "":
		// Read from file
		namespaces, err = ReadNamespacesFromFile(opts.file)
	default:
		return fmt.Errorf("you must specify a namespace name, or use --selector, --file, --all, or --all-locked")
	}

	if err != nil {
		return err
	}

	if len(namespaces) == 0 {
		fmt.Println("ℹ️  No namespaces found to unlock")
		return nil
	}

	// Validate namespaces
	for _, ns := range namespaces {
		if err := o.ValidateNamespace(ns); err != nil {
			return fmt.Errorf("invalid namespace %s: %v", ns, err)
		}
	}

	// Display operation plan
	fmt.Printf("🔓 Planning to unlock %d namespace(s):\n", len(namespaces))
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
		fmt.Printf("\n⚠️  This will restore all workloads in the listed namespaces.\n")
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

	// Execute unlock operation
	fmt.Printf("\n🚀 Starting unlock operation...\n")
	successCount := 0
	failureCount := 0

	for _, ns := range namespaces {
		if err := o.unlockNamespace(ns); err != nil {
			fmt.Printf("❌ Failed to unlock namespace %s: %v\n", ns, err)
			failureCount++
		} else {
			fmt.Printf("✅ Successfully unlocked namespace %s\n", ns)
			successCount++
		}
	}

	// Display results
	fmt.Printf("\n📊 Unlock operation completed:\n")
	fmt.Printf("  ✅ Success: %d\n", successCount)
	fmt.Printf("  ❌ Failed: %d\n", failureCount)

	if failureCount > 0 {
		return fmt.Errorf("%d namespace(s) failed to unlock", failureCount)
	}

	return nil
}

// unlockNamespace unlocks a single namespace
func (o *CommandOptions) unlockNamespace(namespace string) error {
	// Check current status
	currentStatus, err := o.GetNamespaceStatus(namespace)
	if err != nil {
		return err
	}

	if currentStatus == constants.ActiveStatus {
		if !o.force {
			fmt.Printf("⚠️  Namespace %s is already unlocked, skipping...\n", namespace)
			return nil
		}
		fmt.Printf("🔄 Namespace %s is already unlocked, ensuring clean state...\n", namespace)
	}

	// Update namespace
	if err := o.updateNamespaceForUnlock(namespace); err != nil {
		return err
	}

	return nil
}

// updateNamespaceForUnlock updates namespace for unlocking
func (o *CommandOptions) updateNamespaceForUnlock(namespace string) error {
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
	ns.Labels[constants.StatusLabel] = constants.ActiveStatus

	// Clear annotations
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}

	// Remove lock-related annotations
	delete(ns.Annotations, constants.UnlockTimestampLabel)
	delete(ns.Annotations, "clawcloud.run/lock-reason")
	delete(ns.Annotations, "clawcloud.run/lock-operator")

	// Add unlock reason
	if opts.reason != "" {
		ns.Annotations["clawcloud.run/unlock-reason"] = opts.reason
		ns.Annotations["clawcloud.run/unlock-operator"] = "kubectl-block"
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

	o.LogVerbose("Successfully unlocked namespace %s", namespace)
	return nil
}

// getLockedNamespaces gets all locked namespaces
func (o *CommandOptions) getLockedNamespaces() ([]string, error) {
	ctx := context.TODO()
	namespaces, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ns := range namespaces.Items {
		if status := ns.Labels[constants.StatusLabel]; status == constants.LockedStatus {
			result = append(result, ns.Name)
		}
	}

	return result, nil
}
