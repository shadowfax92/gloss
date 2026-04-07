package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"gloss/internal/paths"
	"gloss/internal/server"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Inspect or control the gloss background daemon",
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status (pid, port, log path)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := server.Existing()
		if err != nil {
			fmt.Println("not running")
			fmt.Println("log:", paths.DaemonLog())
			return nil
		}
		fmt.Printf("pid:        %d\n", c.Info.PID)
		fmt.Printf("port:       %d\n", c.Info.Port)
		fmt.Printf("started:    %s\n", c.Info.StartedAt)
		fmt.Printf("server.json %s\n", paths.ServerJSON())
		fmt.Printf("log:        %s\n", paths.DaemonLog())
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Gracefully stop the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := server.Existing()
		if err != nil {
			fmt.Println("not running")
			return nil
		}
		if err := c.Shutdown(); err != nil {
			return err
		}
		fmt.Println("stopped")
		return nil
	},
}

var daemonLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Tail the daemon log",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := exec.Command("tail", "-f", paths.DaemonLog())
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonLogCmd)
	rootCmd.AddCommand(daemonCmd)
}
