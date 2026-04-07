package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"gloss/internal/config"
	"gloss/internal/paths"
)

var configPathFlag bool

var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"cfg"},
	Short:   "Open the gloss config in $EDITOR",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.Load(); err != nil {
			return err
		}
		if configPathFlag {
			fmt.Println(paths.ConfigFile())
			return nil
		}
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nvim"
		}
		c := exec.Command(editor, paths.ConfigFile())
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	configCmd.Flags().BoolVar(&configPathFlag, "path", false, "print config file path and exit")
	rootCmd.AddCommand(configCmd)
}
