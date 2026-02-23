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
	podsCmd.AddCommand(podsPidsCmd)
	rootCmd.AddCommand(podsCmd)
}
