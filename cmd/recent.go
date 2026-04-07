package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gloss/internal/recent"
	"gloss/internal/reltime"
)

var (
	recentJSON bool
	recentDays int
	recentMax  int
)

var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "List recent .md files opened across all nvim sessions",
	Long:  "Reads ~/.local/share/recent-files/history.tsv and prints markdown files seen in the last N days, most recent first.",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := recent.New()
		entries, err := r.ListMarkdown(recentDays, recentMax)
		if err != nil {
			return err
		}
		if recentJSON {
			return json.NewEncoder(os.Stdout).Encode(entries)
		}
		for _, e := range entries {
			fmt.Printf("%s\t%s\n", reltime.Format(e.Time), e.Path)
		}
		return nil
	},
}

func init() {
	recentCmd.Flags().BoolVar(&recentJSON, "json", false, "output as JSON")
	recentCmd.Flags().IntVar(&recentDays, "days", 30, "include files from the last N days")
	recentCmd.Flags().IntVar(&recentMax, "max", 100, "limit to N entries")
	rootCmd.AddCommand(recentCmd)
}
