// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Gthulhu/Gthulhu/cli/client"
	"github.com/spf13/cobra"
)

var strategiesCmd = &cobra.Command{
	Use:   "strategies",
	Short: "Manage scheduling strategies",
}

var strategiesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduling strategies created by the authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newAPIClient()
		resp, err := c.GetStrategies()
		if err != nil {
			return fmt.Errorf("list strategies: %w", err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

var (
	createStrategyFile      string
	createStrategyNamespace string
	createStrategyPriority  int
	createStrategyExecTime  int
	createStrategyCommand   string
	createStrategyK8sNS     []string
	createStrategyLabels    []string
)

var strategiesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new scheduling strategy",
	Long: `Create a scheduling strategy from command line flags or JSON file.

Example using flags:
  gthulhu-cli strategies create --namespace my-strategy --priority 100 \\
    --exec-time 20000000 --command ".*" --k8s-namespace default \\
    --label app=nginx --label tier=frontend

Example using JSON file (-f):
  {
    "priority": 100,
    "executionTime": 20000000,
    "commandRegex": ".*",
    "k8sNamespace": ["default"],
    "labelSelectors": [{"key": "app", "value": "nginx"}],
    "strategyNamespace": "my-strategy"
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var req client.CreateScheduleStrategyRequest
		
		if createStrategyFile != "" {
			// Load from file
			data, err := os.ReadFile(createStrategyFile)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			if err := json.Unmarshal(data, &req); err != nil {
				return fmt.Errorf("parse JSON: %w", err)
			}
		} else {
			// Build from flags
			if createStrategyNamespace == "" {
				return fmt.Errorf("--namespace is required")
			}
			
			req.StrategyNamespace = createStrategyNamespace
			req.Priority = createStrategyPriority
			req.ExecutionTime = createStrategyExecTime
			req.CommandRegex = createStrategyCommand
			req.K8sNamespace = createStrategyK8sNS
			
			// Parse label selectors
			for _, label := range createStrategyLabels {
				parts := strings.SplitN(label, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid label format: %s (expected key=value)", label)
				}
				req.LabelSelectors = append(req.LabelSelectors, client.LabelSelector{
					Key:   parts[0],
					Value: parts[1],
				})
			}
		}
		
		c := newAPIClient()
		resp, err := c.CreateStrategy(&req)
		if err != nil {
			return fmt.Errorf("create strategy: %w", err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		fmt.Println("Strategy created successfully")
		return nil
	},
}

var deleteStrategyID string

var strategiesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a scheduling strategy",
	Long:  `Delete a scheduling strategy by its ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if deleteStrategyID == "" {
			return fmt.Errorf("--id is required")
		}
		
		c := newAPIClient()
		resp, err := c.DeleteStrategy(deleteStrategyID)
		if err != nil {
			return fmt.Errorf("delete strategy: %w", err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		fmt.Printf("Strategy %s deleted successfully\\n", deleteStrategyID)
		return nil
	},
}

func init() {
	// Create command flags
	strategiesCreateCmd.Flags().StringVarP(&createStrategyFile, "file", "f", "", "Path to JSON file containing strategy")
	strategiesCreateCmd.Flags().StringVar(&createStrategyNamespace, "namespace", "", "Strategy namespace (required)")
	strategiesCreateCmd.Flags().IntVar(&createStrategyPriority, "priority", 0, "Priority value")
	strategiesCreateCmd.Flags().IntVar(&createStrategyExecTime, "exec-time", 0, "Execution time in nanoseconds")
	strategiesCreateCmd.Flags().StringVar(&createStrategyCommand, "command", "", "Command regex pattern")
	strategiesCreateCmd.Flags().StringSliceVar(&createStrategyK8sNS, "k8s-namespace", nil, "Kubernetes namespace(s)")
	strategiesCreateCmd.Flags().StringSliceVar(&createStrategyLabels, "label", nil, "Label selector (key=value), can be specified multiple times")
	
	// Delete command flags
	strategiesDeleteCmd.Flags().StringVar(&deleteStrategyID, "id", "", "Strategy ID to delete (required)")
	
	strategiesCmd.AddCommand(strategiesListCmd, strategiesCreateCmd, strategiesDeleteCmd)
	rootCmd.AddCommand(strategiesCmd)
}
