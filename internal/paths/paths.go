package paths

import (
	"os"
	"path/filepath"
)

func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gloss")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gloss")
}

func StateDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "gloss")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "gloss")
}

func DataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "gloss")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "gloss")
}

func ConfigFile() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func ServerJSON() string {
	return filepath.Join(StateDir(), "server.json")
}

func ServerLock() string {
	return filepath.Join(StateDir(), "server.lock")
}

func DaemonLog() string {
	return filepath.Join(StateDir(), "daemon.log")
}

func HighlightsFile() string {
	return filepath.Join(DataDir(), "highlights.json")
}

func RecentFilesTSV() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "recent-files", "history.tsv")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "recent-files", "history.tsv")
}

func EnsureAll() error {
	for _, d := range []string{ConfigDir(), StateDir(), DataDir()} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}
