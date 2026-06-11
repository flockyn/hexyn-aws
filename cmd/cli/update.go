package cli

import (
	"os"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hexyn-aws to the latest version",
	Run: func(cmd *cobra.Command, _ []string) {
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			color.Yellow("Note: GITHUB_TOKEN is not set. If the repository is private, the update might fail.")
		}

		source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{APIToken: token})
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
			color.Red("Error updating to latest version: %v", err)
			return
		}
		color.Green("Successfully updated to %s", latest.Version())
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
