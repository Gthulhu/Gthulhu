// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Gthulhu/Gthulhu/cli/client"
	"github.com/spf13/cobra"
)

var strategiesCmd = &cobra.Command{
	Use:   "strategies",
	Short: "Manage scheduling strategies",
}

var strategiesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current scheduling strategies from the API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newAPIClient()
		resp, err := c.GetStrategies()
		if err != nil {
			return fmt.Errorf("get strategies: %w", err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

var strategiesSetFile string

var strategiesSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set scheduling strategies on the API server",
	Long: `Send a JSON file containing scheduling strategies to the API server.

Example JSON file:
{
  "strategies": [
    {
      "priority": true,
      "execution_time": 20000000,
      "selectors": [{"key": "nf", "value": "upf"}],
      "command_regex": ".*"
    }
  ]
}`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if strategiesSetFile == "" {
			return fmt.Errorf("--file is required")
		}
		data, err := os.ReadFile(strategiesSetFile)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		var req client.SchedulingStrategiesRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
		c := newAPIClient()
		resp, err := c.SetStrategies(&req)
		if err != nil {
			return fmt.Errorf("set strategies: %w", err)
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
	strategiesSetCmd.Flags().StringVarP(&strategiesSetFile, "file", "f", "", "Path to JSON file containing strategies")
	strategiesCmd.AddCommand(strategiesGetCmd, strategiesSetCmd)
	rootCmd.AddCommand(strategiesCmd)
}
