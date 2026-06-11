// Package cli wires the application's dependencies and exposes the cobra
// command tree (root, ui, update).
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	isLocal    bool
	updateFlag bool
	// Version is overridden at build time via -ldflags.
	Version = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "hexyn-aws",
	Short:   "A production-ready AWS SSM Parameter Store management tool",
	Long:    `Hexyn AWS is a high-performance CLI tool with an interactive TUI for managing AWS SSM Parameter Store and ECS configurations.`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		if updateFlag {
			updateCmd.Run(cmd, args)
			return
		}
		uiCmd.Run(cmd, args)
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVar(&isLocal, "init", false, "Use current directory for configuration (.hexyn-aws)")
	rootCmd.Flags().BoolVar(&updateFlag, "update", false, "Update hexyn-aws to the latest version")
}
