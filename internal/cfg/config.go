package cfg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	CurrentContext string              `yaml:"current-context"`
	Contexts       map[string]*Context `yaml:"contexts"`
	Aliases        map[string]string   `yaml:"aliases,omitempty"`
}

type Context struct {
	Grafana  GrafanaConfig     `yaml:"grafana"`
	Sources  map[string]string `yaml:"sources,omitempty"`
	Defaults DefaultsConfig    `yaml:"defaults,omitempty"`
}

type GrafanaConfig struct {
	URL          string `yaml:"url"`
	Token        string `yaml:"token,omitempty"`
	TokenCommand string `yaml:"token-command,omitempty"`
}

type DefaultsConfig struct {
	Since  string            `yaml:"since,omitempty"`
	Limit  int               `yaml:"limit,omitempty"`
	Output string            `yaml:"output,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

func DefaultConfigPath() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".config", "grafquery", "config.yaml"), nil
}

func DefaultConfig() *Config {
	return &Config{
		CurrentContext: "",
		Contexts:       map[string]*Context{},
		Aliases:        map[string]string{},
	}
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	c := DefaultConfig()
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, err
	}
	if c.Contexts == nil {
		c.Contexts = map[string]*Context{}
	}
	if c.Aliases == nil {
		c.Aliases = map[string]string{}
	}
	return c, nil
}

func Save(path string, c *Config) error {
	if c == nil {
		return errors.New("config nil")
	}
	if c.Contexts == nil {
		c.Contexts = map[string]*Context{}
	}
	if c.Aliases == nil {
		c.Aliases = map[string]string{}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func (c *Config) ResolveContext(name string) (*Context, string, error) {
	if c == nil {
		return nil, "", errors.New("config nil")
	}
	ctxName := strings.TrimSpace(name)
	if ctxName == "" {
		ctxName = strings.TrimSpace(c.CurrentContext)
	}
	if ctxName == "" {
		return nil, "", errors.New("no current context set (run: grafquery init)")
	}
	ctx, ok := c.Contexts[ctxName]
	if !ok || ctx == nil {
		return nil, "", fmt.Errorf("context %q not found", ctxName)
	}
	return ctx, ctxName, nil
}

func ResolveToken(g GrafanaConfig) (string, error) {
	if s := strings.TrimSpace(g.Token); s != "" {
		return os.ExpandEnv(s), nil
	}
	if cmdStr := strings.TrimSpace(g.TokenCommand); cmdStr != "" {
		cmd := exec.Command("sh", "-lc", cmdStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("token-command failed: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		t := strings.TrimSpace(string(out))
		if t == "" {
			return "", errors.New("token-command returned empty token")
		}
		return t, nil
	}
	return "", errors.New("no token configured (set grafana.token or grafana.token-command)")
}
