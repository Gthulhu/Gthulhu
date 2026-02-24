// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Kubernetes node operations",
}

var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Kubernetes nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newAPIClient()
		resp, err := c.ListNodes()
		if err != nil {
			return fmt.Errorf("list nodes: %w", err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

var nodesPidsNodeID string

var nodesPidsCmd = &cobra.Command{
	Use:   "pids",
	Short: "Get pod-PID mappings for a specific node",
	Long:  `Retrieve all pods and their process IDs running on a specific Kubernetes node.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if nodesPidsNodeID == "" {
			return fmt.Errorf("--node-id is required")
		}

		c := newAPIClient()
		resp, err := c.GetNodePodPIDMapping(nodesPidsNodeID)
		if err != nil {
			return fmt.Errorf("get node pod-PID mapping: %w", err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	nodesPidsCmd.Flags().StringVar(&nodesPidsNodeID, "node-id", "", "Node ID to query (required)")

	nodesCmd.AddCommand(nodesListCmd, nodesPidsCmd)
	rootCmd.AddCommand(nodesCmd)
}
