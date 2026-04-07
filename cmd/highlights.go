package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gloss/internal/config"
	"gloss/internal/highlights"
)

var highlightsJSON bool

var highlightsCmd = &cobra.Command{
	Use:     "highlights",
	Aliases: []string{"hl"},
	Short:   "Manage saved highlights",
}

var highlightsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved highlights",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		all, err := store.List()
		if err != nil {
			return err
		}
		if highlightsJSON {
			return json.NewEncoder(os.Stdout).Encode(all)
		}
		for _, h := range all {
			stale := ""
			if h.IsStale() {
				stale = " (stale)"
			}
			fmt.Printf("%s  %s:%d-%d%s\n", h.ID, h.AbsPath, h.LineStart, h.LineEnd, stale)
		}
		return nil
	},
}

var highlightsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show one highlight",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		h, err := store.Get(args[0])
		if err != nil {
			return err
		}
		if highlightsJSON {
			return json.NewEncoder(os.Stdout).Encode(h)
		}
		fmt.Printf("%s:%d-%d\n\n%s\n", h.AbsPath, h.LineStart, h.LineEnd, h.Text)
		return nil
	},
}

var highlightsRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Delete a highlight by id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		return store.Delete(args[0])
	},
}

var highlightsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all highlights as markdown to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		return store.Export(os.Stdout)
	},
}

func openStore() (*highlights.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return highlights.New(cfg.Highlights.Path)
}

func init() {
	highlightsListCmd.Flags().BoolVar(&highlightsJSON, "json", false, "output as JSON")
	highlightsShowCmd.Flags().BoolVar(&highlightsJSON, "json", false, "output as JSON")
	highlightsCmd.AddCommand(highlightsListCmd)
	highlightsCmd.AddCommand(highlightsShowCmd)
	highlightsCmd.AddCommand(highlightsRmCmd)
	highlightsCmd.AddCommand(highlightsExportCmd)
	rootCmd.AddCommand(highlightsCmd)
}
