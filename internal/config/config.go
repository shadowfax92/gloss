package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"gloss/internal/paths"
)

type Config struct {
	Port          int          `yaml:"port"`
	OpenBrowser   bool         `yaml:"open_browser"`
	CopyPathStyle string       `yaml:"copy_path_style"`
	Recent        RecentConfig `yaml:"recent"`
	Ignore        []string     `yaml:"ignore"`
	Highlights    HLConfig     `yaml:"highlights"`
}

type RecentConfig struct {
	Days int `yaml:"days"`
	Max  int `yaml:"max"`
}

type HLConfig struct {
	Path string `yaml:"path"`
}

func Default() Config {
	return Config{
		Port:          0,
		OpenBrowser:   true,
		CopyPathStyle: "tilde",
		Recent: RecentConfig{
			Days: 30,
			Max:  100,
		},
		Ignore: []string{
			"node_modules",
			".git",
			".next",
			"dist",
			"vendor",
			"target",
		},
		Highlights: HLConfig{
			Path: paths.HighlightsFile(),
		},
	}
}

const fileHeader = `# Gloss configuration
# Edit with: gloss config
# Print path: gloss config --path

`

func Load() (*Config, error) {
	path := paths.ConfigFile()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return createDefault(path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	cfg.normalize()
	return &cfg, nil
}

func (c *Config) normalize() {
	c.Highlights.Path = expandTilde(c.Highlights.Path)
	if c.CopyPathStyle == "" {
		c.CopyPathStyle = "tilde"
	}
	if c.Recent.Days <= 0 {
		c.Recent.Days = 30
	}
	if c.Recent.Max <= 0 {
		c.Recent.Max = 100
	}
}

func createDefault(path string) (*Config, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	cfg := Default()
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(fileHeader+string(out)), 0644); err != nil {
		return nil, err
	}
	cfg.normalize()
	return &cfg, nil
}

func expandTilde(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	if p == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	return p
}
