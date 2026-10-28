package pilot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type TabScope string

const (
	TabsOwn TabScope = "own"
	TabsAll TabScope = "all"
)

type Upstream struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type Config struct {
	Upstream     Upstream `json:"upstream"`
	Tabs         TabScope `json:"tabs"`
	AllowHosts   []string `json:"allowHosts"`
	ConfirmHosts []string `json:"confirmHosts"`
	AuditLog     string   `json:"auditLog"`
	GrantMinutes int      `json:"grantMinutes"`

	SkipConfirmations bool `json:"-"`

	dir string
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	cfg.dir = filepath.Dir(abs)
	return &cfg, cfg.validate()
}

func (c *Config) validate() error {
	if c.Upstream.Command == "" {
		return fmt.Errorf("upstream.command is required")
	}
	switch c.Tabs {
	case "":
		c.Tabs = TabsOwn
	case TabsOwn, TabsAll:
	default:
		return fmt.Errorf("tabs must be %q or %q, got %q", TabsOwn, TabsAll, c.Tabs)
	}
	if c.GrantMinutes < 0 {
		return fmt.Errorf("grantMinutes must be 0 (off) or more, got %d", c.GrantMinutes)
	}
	if len(c.AllowHosts) == 0 {
		return fmt.Errorf("allowHosts is required; use [\"*\"] to allow every host")
	}
	return nil
}

func (c *Config) Dir() string {
	return c.dir
}

func (c *Config) resolve(path string) string {
	if path == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.dir, path)
}

func (c *Config) UpstreamCommand() string {
	if strings.ContainsRune(c.Upstream.Command, filepath.Separator) {
		return c.resolve(c.Upstream.Command)
	}
	return c.Upstream.Command
}

func (c *Config) AuditPath() string {
	return c.resolve(c.AuditLog)
}
