package cli

import (
	"fmt"
	"os"

	"hexyn-aws/internal/bootstrap"
	"hexyn-aws/internal/config"
	"hexyn-aws/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Start interactive TUI",
	Run: func(_ *cobra.Command, _ []string) {
		if err := runTUI(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	},
}

// runTUI builds the wired service and launches the interactive TUI.
func runTUI() error {
	cfg := config.New(isLocal)
	_ = cfg.EnsureDirectories()

	svc := bootstrap.NewService(cfg)

	_, err := tea.NewProgram(tui.NewModel(svc, cfg), tea.WithAltScreen()).Run()
	return err
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
