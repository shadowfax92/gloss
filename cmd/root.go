package cmd

import (
	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:           "gloss [path]",
	Short:         "Browser markdown viewer with one-shot highlight-to-clipboard",
	Long:          "gloss serves a folder of markdown files in a clean browser UI with a file tree, GitHub-styled rendering, and a floating selection bar that copies highlighted text to the clipboard with file:line references for pasting into AI agents.",
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveCmd.RunE(cmd, args)
	},
}

func Execute() error {
	return rootCmd.Execute()
}
