// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "Kubernetes pod information",
}

var podsPidsCmd = &cobra.Command{
	Use:   "pids",
	Short: "Get pod-to-PID mappings from the API server",
	Long: `Get pod-to-PID mappings from the API server.
Note: This calls the decisionmaker endpoint /api/v1/pods/pids.
For Manager Mode, consider using 'nodes pids --node-id <id>' instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newAPIClient()
		resp, err := c.GetPodPIDs()
		if err != nil {
			return fmt.Errorf("get pod PIDs: %w", err)
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
	// pods command is disabled in Manager Mode.
	// Use 'nodes pids --node-id <id>' to query pod-PID mappings for specific nodes.
	// The /api/v1/pods/pids endpoint is from decisionmaker service and should not be
	// directly exposed in Manager Mode CLI.
	
	// podsCmd.AddCommand(podsPidsCmd)
	// rootCmd.AddCommand(podsCmd)
}
