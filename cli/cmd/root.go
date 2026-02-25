// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/Gthulhu/Gthulhu/cli/client"
	"github.com/spf13/cobra"
)

var (
	apiURL string
	noAuth bool

	// K8s flags
	kubeconfig string
	namespace  string
)

// rootCmd is the top-level CLI command.
var rootCmd = &cobra.Command{
	Use:   "gthulhu-cli",
	Short: "Gthulhu CLI – manage the Gthulhu sched_ext scheduler",
	Long: `gthulhu-cli is a command-line tool for interacting with the Gthulhu
scheduler (Manager Mode). It can manage scheduling strategies, list nodes,
query pod-PID mappings, and inspect the BPF priority map.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Disable the auto-generated completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().StringVarP(&apiURL, "api-url", "u", "http://127.0.0.1:8080", "Gthulhu API server URL")
	rootCmd.PersistentFlags().BoolVar(&noAuth, "no-auth", false, "Skip JWT authentication")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (defaults to ~/.kube/config)")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace for scheduler pods")
}

// newAPIClient creates a client.Client from the global flags.
func newAPIClient() *client.Client {
	// Enable auth by default unless explicitly disabled with --no-auth
	authEnabled := !noAuth
	return client.NewClient(apiURL, authEnabled)
}
