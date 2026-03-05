package main

import (
	"log"
	"os"

	dmcmd "github.com/Gthulhu/api/decisionmaker/cmd"
	managercmd "github.com/Gthulhu/api/manager/cmd"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{Use: "manager or decisionmaker"}
)

func main() {
	rootCmd.AddCommand(managercmd.ManagerCmd, dmcmd.DMCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Command execution failed: %v", err)
		os.Exit(1)
	}
}
