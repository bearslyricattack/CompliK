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

// Package main is the entry point for the kubectl-block CLI plugin.
// It provides commands to manage Kubernetes namespace lifecycle through the block controller.
package main

import (
	"fmt"
	"os"

	"github.com/bearslyricattack/CompliK/block-controller/cmd/kubectl-block/cmd"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	// Initialize klog
	klog.InitFlags(nil)

	// Create root command
	rootCmd := &cobra.Command{
		Use:   "kubectl-block",
		Short: "Block controller CLI for managing namespace lifecycle",
		Long: `kubectl-block is a CLI tool for managing Kubernetes namespace lifecycle
through the block controller. It provides commands to lock, unlock, and monitor
namespaces with ease.`,
		Version: "v0.2.0",
	}

	// Initialize kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Add subcommands
	rootCmd.AddCommand(cmd.NewLockCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewUnlockCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewStatusCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewListCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewCleanupCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewReportCommand(kubeConfig))

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configOverrides.Context.Context, "context", "c", "", "The name of the kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&configOverrides.CurrentContext, "namespace", "n", "", "If present, the namespace scope for this CLI request")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "If true, only print the object that would be sent, without sending it")

	// Execute command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var (
	verbose bool
	dryRun  bool
)
