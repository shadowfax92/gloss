package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"gloss/internal/config"
	"gloss/internal/server"
)

var (
	serveForeground bool
	serveNoOpen     bool
	serveQuiet      bool
	servePort       int
	serveFile       string
	serveDetached   bool
)

var serveCmd = &cobra.Command{
	Use:   "serve [path]",
	Short: "Serve a folder of markdown files in the browser",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if serveDetached {
			return runDetachedDaemon()
		}
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return err
		}
		folderAbs := abs
		fileAbs := serveFile
		if !info.IsDir() {
			fileAbs = abs
			folderAbs = filepath.Dir(abs)
		}

		if serveForeground {
			return runForeground(folderAbs)
		}

		client, err := server.EnsureRunning()
		if err != nil {
			return err
		}
		result, err := client.Open(folderAbs, fileAbs)
		if err != nil {
			return err
		}
		url := client.BrowserURL(result.FolderID, result.FileRel)
		if !serveQuiet {
			fmt.Fprintf(os.Stderr, "gloss → %s (daemon pid %d)\n", url, client.Info.PID)
		}
		if !serveNoOpen {
			openBrowser(url)
		}
		return nil
	},
}

// hidden subcommand the spawn flow uses to launch the long-lived daemon.
var serveDaemonRunCmd = &cobra.Command{
	Use:    "_serve",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDetachedDaemon()
	},
}

func init() {
	serveCmd.Flags().BoolVar(&serveForeground, "foreground", false, "run server in foreground (do not detach)")
	serveCmd.Flags().BoolVar(&serveNoOpen, "no-open", false, "do not open the browser")
	serveCmd.Flags().BoolVar(&serveQuiet, "quiet", false, "suppress status output")
	serveCmd.Flags().IntVar(&servePort, "port", 0, "pin port (0 = random)")
	serveCmd.Flags().StringVar(&serveFile, "file", "", "focus this file path in the opened folder")

	serveDaemonRunCmd.Flags().BoolVar(&serveDetached, "detached", false, "internal: run as detached daemon")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(serveDaemonRunCmd)
}

func runDetachedDaemon() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if servePort > 0 {
		cfg.Port = servePort
	}
	return server.RunDetached(cfg)
}

func runForeground(folderAbs string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if servePort > 0 {
		cfg.Port = servePort
	}
	go func() {
		// Wait briefly for the listener to bind, then push the open and open
		// the browser. Avoids the spawn-or-reuse client path entirely.
		client, err := waitForLocalDaemon()
		if err != nil {
			return
		}
		result, err := client.Open(folderAbs, serveFile)
		if err != nil {
			return
		}
		url := client.BrowserURL(result.FolderID, result.FileRel)
		if !serveQuiet {
			fmt.Fprintf(os.Stderr, "gloss → %s\n", url)
		}
		if !serveNoOpen {
			openBrowser(url)
		}
	}()
	return server.RunDetached(cfg)
}

func waitForLocalDaemon() (*server.Client, error) {
	for range 100 {
		c, err := server.Existing()
		if err == nil {
			return c, nil
		}
		sleepMS(20)
	}
	return nil, fmt.Errorf("daemon did not become ready")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
