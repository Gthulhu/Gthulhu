// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "View scheduler metrics",
}

var metricsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current scheduler metrics from the API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newAPIClient()
		resp, err := c.GetMetrics()
		if err != nil {
			return fmt.Errorf("get metrics: %w", err)
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
	metricsCmd.AddCommand(metricsGetCmd)
	rootCmd.AddCommand(metricsCmd)
}
