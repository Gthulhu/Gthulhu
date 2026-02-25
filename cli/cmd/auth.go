// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/Gthulhu/Gthulhu/cli/client"
	"github.com/spf13/cobra"
)

var (
	username string
	password string
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login with username and password",
	Long: `Authenticate to the API server using username and password to obtain a JWT token.
The token will be saved to /tmp/gthulhu-token-{UID}.json for subsequent requests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if username == "" {
			return fmt.Errorf("--username is required")
		}
		if password == "" {
			return fmt.Errorf("--password is required")
		}
		c := newAPIClient()
		resp, err := c.Login(username, password)
		if err != nil {
			return fmt.Errorf("login: %w", err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored authentication token",
	Long:  `Remove the stored JWT token from local storage.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := client.ClearToken(); err != nil {
			return fmt.Errorf("logout: %w", err)
		}
		fmt.Println("Successfully logged out. Token has been cleared.")
		return nil
	},
}

func init() {
	authLoginCmd.Flags().StringVarP(&username, "username", "U", "", "Username for login")
	authLoginCmd.Flags().StringVarP(&password, "password", "P", "", "Password for login")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}
