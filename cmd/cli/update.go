package cli

import (
	"errors"
	"os"
	"runtime"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hexyn-aws to the latest version",
	Run: func(cmd *cobra.Command, _ []string) {
		source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
		if err != nil {
			color.Red("Error creating GitHub source: %v", err)
			return
		}

		updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source})
		if err != nil {
			color.Red("Error creating updater: %v", err)
			return
		}

		repo := selfupdate.ParseSlug("flockyn/hexyn-aws")
		latest, found, err := updater.DetectLatest(cmd.Context(), repo)
		if err != nil {
			color.Red("Error detecting latest version: %v", err)
			return
		}
		if !found {
			color.Yellow("No releases found.")
			return
		}
		if latest.LessOrEqual(Version) {
			color.Green("Current version (%s) is up to date.", Version)
			return
		}

		color.Cyan("New version found: %s", latest.Version())
		color.Cyan("Updating...")

		exe, err := os.Executable()
		if err != nil {
			color.Red("Error getting executable path: %v", err)
			return
		}
		if err := updater.UpdateTo(cmd.Context(), latest, exe); err != nil {
			if errors.Is(err, os.ErrPermission) {
				color.Red("Permission denied writing to %s", exe)
				color.Yellow("hexyn-aws is installed in a location that requires elevated privileges.")
				if runtime.GOOS == "windows" {
					color.Yellow("Re-run the update from a terminal opened as Administrator:")
					color.Cyan("  hexyn-aws update")
				} else {
					color.Yellow("Re-run the update with sudo:")
					color.Cyan("  sudo hexyn-aws update")
				}
				return
			}
			color.Red("Error updating to latest version: %v", err)
			return
		}
		color.Green("Successfully updated to %s", latest.Version())
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
