package cmd

import (
	"os"

	"hexyn-aws/internal/aws"

	"github.com/spf13/cobra"
)

var (
	isLocal    bool
	updateFlag bool
	Version    = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "hexyn-aws",
	Short:   "A production-ready AWS SSM Parameter Store management tool",
	Long:    `Hexyn AWS is a high-performance CLI tool with an interactive TUI for managing AWS SSM Parameter Store and ECS configurations.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		aws.SetBaseDir(isLocal)
	},
	Run: func(cmd *cobra.Command, args []string) {
		if updateFlag {
			updateCmd.Run(cmd, args)
			return
		}
		uiCmd.Run(cmd, args)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVar(&isLocal, "init", false, "Use current directory for configuration (.hexyn-aws)")
	rootCmd.Flags().BoolVar(&updateFlag, "update", false, "Update hexyn-aws to the latest version")
}
