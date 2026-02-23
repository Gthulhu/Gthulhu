// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Obtain a JWT token from the API server",
	Long:  `Request a new JWT token using the public key specified by --public-key.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if publicKey == "" {
			return fmt.Errorf("--public-key is required for token requests")
		}
		c := newAPIClient()
		// Force auth to be disabled for the raw token request since we
		// are obtaining the token itself.
		resp, err := c.RequestToken()
		if err != nil {
			return fmt.Errorf("request token: %w", err)
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
	authCmd.AddCommand(authTokenCmd)
	rootCmd.AddCommand(authCmd)
}
