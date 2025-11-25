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
	"fmt"
	"os"

	"github.com/bearslyricattack/CompliK/block-controller/cmd/kubectl-block/cmd"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	// 初始化 klog
	klog.InitFlags(nil)

	// 创建根命令
	rootCmd := &cobra.Command{
		Use:   "kubectl-block",
		Short: "Block controller CLI for managing namespace lifecycle",
		Long: `kubectl-block is a CLI tool for managing Kubernetes namespace lifecycle
through the block controller. It provides commands to lock, unlock, and monitor
namespaces with ease.`,
		Version: "v0.2.0",
	}

	// 初始化 kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// 添加子命令
	rootCmd.AddCommand(cmd.NewLockCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewUnlockCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewStatusCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewListCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewCleanupCommand(kubeConfig))
	rootCmd.AddCommand(cmd.NewReportCommand(kubeConfig))

	// 全局参数
	rootCmd.PersistentFlags().StringVarP(&configOverrides.Context.Context, "context", "c", "", "The name of the kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&configOverrides.CurrentContext, "namespace", "n", "", "If present, the namespace scope for this CLI request")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "If true, only print the object that would be sent, without sending it")

	// 执行命令
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var (
	verbose bool
	dryRun  bool
)
