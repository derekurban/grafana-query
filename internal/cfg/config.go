package cfg

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ModeRestrictive = "restrictive"
	ModeFullAccess  = "full-access"
)

type Config struct {
	Setup          SetupConfig         `yaml:"setup,omitempty"`
	CurrentProject string              `yaml:"current-project,omitempty"`
	Projects       map[string]*Project `yaml:"projects,omitempty"`
}

type SetupConfig struct {
	Mode           string            `yaml:"mode,omitempty"`
	GrafanaAPIURL  string            `yaml:"grafana-api-url,omitempty"`
	StackName      string            `yaml:"stack-name,omitempty"`
	OTLPEndpoint   string            `yaml:"otlp-endpoint,omitempty"`
	OTLPInstanceID string            `yaml:"otlp-instance-id,omitempty"`
	Sources        map[string]string `yaml:"sources,omitempty"`
	Cloud          CloudSetupConfig  `yaml:"cloud,omitempty"`
}

type CloudSetupConfig struct {
	OrgSlug string `yaml:"org-slug,omitempty"`
	StackID string `yaml:"stack-id,omitempty"`
	Region  string `yaml:"region,omitempty"`
}

type Project struct {
	Name                string            `yaml:"name"`
	PrimaryService      string            `yaml:"primary-service,omitempty"`
	Services            []string          `yaml:"services,omitempty"`
	Sources             map[string]string `yaml:"sources,omitempty"`
	Defaults            DefaultsConfig    `yaml:"defaults,omitempty"`
	QueryScope          QueryScopeConfig  `yaml:"query-scope,omitempty"`
	CurrentRun          *RunState         `yaml:"current-run,omitempty"`
	BootstrapAttributes map[string]string `yaml:"bootstrap-attributes,omitempty"`
	WriteToken          string            `yaml:"write-token,omitempty"`
	ManagedWriteToken   bool              `yaml:"managed-write-token,omitempty"`
	ManagedPolicyID     string            `yaml:"managed-policy-id,omitempty"`
	ManagedPolicyName   string            `yaml:"managed-policy-name,omitempty"`
	ManagedTokenID      string            `yaml:"managed-token-id,omitempty"`
	ManagedTokenName    string            `yaml:"managed-token-name,omitempty"`
}

type RunState struct {
	ID        string `yaml:"id,omitempty"`
	StartedAt string `yaml:"started-at,omitempty"`
}

type DefaultsConfig struct {
	Since  string `yaml:"since,omitempty"`
	Limit  int    `yaml:"limit,omitempty"`
	Output string `yaml:"output,omitempty"`
}

type QueryScopeConfig struct {
	LogServiceLabel    string `yaml:"log-service-label,omitempty"`
	MetricServiceLabel string `yaml:"metric-service-label,omitempty"`
	TraceServiceAttr   string `yaml:"trace-service-attr,omitempty"`
	LogRunLabel        string `yaml:"log-run-label,omitempty"`
	MetricRunLabel     string `yaml:"metric-run-label,omitempty"`
	TraceRunAttr       string `yaml:"trace-run-attr,omitempty"`
}

func DefaultConfigPath() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".config", "wabsignal", "config.yaml"), nil
}

func LegacyConfigPath() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".config", "grafquery", "config.yaml"), nil
}

func DefaultConfig() *Config {
	return &Config{
		Setup:    SetupConfig{},
		Projects: map[string]*Project{},
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
	c.normalize()
	return c, nil
}

func Save(path string, c *Config) error {
	if c == nil {
		return errors.New("config nil")
	}
	c.normalize()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func LoadWithMigration(path string) (*Config, bool, error) {
	c, err := Load(path)
	if err != nil {
		return nil, false, err
	}
	if hasConfigFile(path) {
		return c, false, nil
	}

	legacyPath, err := LegacyConfigPath()
	if err != nil {
		return c, false, nil
	}
	if !hasConfigFile(legacyPath) {
		return c, false, nil
	}

	migrated, err := migrateLegacy(legacyPath)
	if err != nil {
		return nil, false, err
	}
	if err := Save(path, migrated); err != nil {
		return nil, false, err
	}
	return migrated, true, nil
}

func (c *Config) SetupComplete() bool {
	if c == nil {
		return false
	}
	mode := NormalizeMode(c.Setup.Mode)
	if mode == "" {
		return false
	}
	if strings.TrimSpace(c.Setup.GrafanaAPIURL) == "" {
		return false
	}
	if strings.TrimSpace(c.Setup.OTLPEndpoint) == "" {
		return false
	}
	if strings.TrimSpace(c.Setup.OTLPInstanceID) == "" {
		return false
	}
	if mode == ModeFullAccess {
		return strings.TrimSpace(c.Setup.Cloud.Region) != "" && strings.TrimSpace(c.Setup.Cloud.StackID) != ""
	}
	return true
}

func (c *Config) ResolveProject(name string) (*Project, string, error) {
	if c == nil {
		return nil, "", errors.New("config nil")
	}
	projectName := strings.TrimSpace(name)
	if projectName == "" {
		projectName = strings.TrimSpace(c.CurrentProject)
	}
	if projectName == "" {
		return nil, "", errors.New("no current project set (run: wabsignal project create or wabsignal project use)")
	}
	project, ok := c.Projects[projectName]
	if !ok || project == nil {
		return nil, "", fmt.Errorf("project %q not found", projectName)
	}
	project.normalize(projectName)
	return project, projectName, nil
}

func (p *Project) AllServices() []string {
	if p == nil {
		return nil
	}
	services := append([]string{p.PrimaryService}, p.Services...)
	return uniqStrings(services)
}

func (p *Project) EnsureDefaults() {
	if p == nil {
		return
	}
	if strings.TrimSpace(p.Defaults.Since) == "" {
		p.Defaults.Since = "1h"
	}
	if p.Defaults.Limit <= 0 {
		p.Defaults.Limit = 100
	}
	if strings.TrimSpace(p.Defaults.Output) == "" {
		p.Defaults.Output = "auto"
	}
	if strings.TrimSpace(p.QueryScope.LogServiceLabel) == "" {
		p.QueryScope.LogServiceLabel = "service_name"
	}
	if strings.TrimSpace(p.QueryScope.MetricServiceLabel) == "" {
		p.QueryScope.MetricServiceLabel = "service_name"
	}
	if strings.TrimSpace(p.QueryScope.TraceServiceAttr) == "" {
		p.QueryScope.TraceServiceAttr = "resource.service.name"
	}
	if strings.TrimSpace(p.QueryScope.LogRunLabel) == "" {
		p.QueryScope.LogRunLabel = "wabsignal_run_id"
	}
	if strings.TrimSpace(p.QueryScope.MetricRunLabel) == "" {
		p.QueryScope.MetricRunLabel = "wabsignal_run_id"
	}
	if strings.TrimSpace(p.QueryScope.TraceRunAttr) == "" {
		p.QueryScope.TraceRunAttr = "resource.wabsignal_run_id"
	}
	if p.Sources == nil {
		p.Sources = map[string]string{}
	}
}

func NormalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModeRestrictive:
		return ModeRestrictive
	case ModeFullAccess:
		return ModeFullAccess
	default:
		return ""
	}
}

func NormalizeGrafanaAPIURL(rawURL, stackName string) (string, string, error) {
	stackName = strings.TrimSpace(stackName)
	rawURL = strings.TrimSpace(rawURL)
	switch {
	case rawURL != "":
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", "", fmt.Errorf("invalid grafana api url: %w", err)
		}
		if u.Scheme == "" {
			u.Scheme = "https"
		}
		if u.Host == "" && u.Path != "" {
			u.Host = u.Path
			u.Path = ""
		}
		u.Path = ""
		u.RawQuery = ""
		u.Fragment = ""
		host := strings.TrimSuffix(strings.TrimSpace(u.Hostname()), ".grafana.net")
		return strings.TrimRight(u.String(), "/"), host, nil
	case stackName != "":
		return fmt.Sprintf("https://%s.grafana.net", stackName), stackName, nil
	default:
		return "", "", errors.New("either grafana api url or stack name is required")
	}
}

func hasConfigFile(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (c *Config) normalize() {
	if c.Projects == nil {
		c.Projects = map[string]*Project{}
	}
	c.Setup.Mode = NormalizeMode(c.Setup.Mode)
	c.Setup.GrafanaAPIURL = strings.TrimRight(strings.TrimSpace(c.Setup.GrafanaAPIURL), "/")
	c.Setup.StackName = strings.TrimSpace(c.Setup.StackName)
	c.Setup.OTLPEndpoint = strings.TrimSpace(c.Setup.OTLPEndpoint)
	c.Setup.OTLPInstanceID = strings.TrimSpace(c.Setup.OTLPInstanceID)
	if c.Setup.Sources == nil {
		c.Setup.Sources = map[string]string{}
	}
	c.Setup.Cloud.OrgSlug = strings.TrimSpace(c.Setup.Cloud.OrgSlug)
	c.Setup.Cloud.StackID = strings.TrimSpace(c.Setup.Cloud.StackID)
	c.Setup.Cloud.Region = strings.TrimSpace(c.Setup.Cloud.Region)

	for name, project := range c.Projects {
		if project == nil {
			delete(c.Projects, name)
			continue
		}
		project.normalize(name)
	}
}

func (p *Project) normalize(name string) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = strings.TrimSpace(name)
	}
	p.PrimaryService = strings.TrimSpace(p.PrimaryService)
	p.Services = uniqStrings(p.Services)
	if p.CurrentRun != nil {
		p.CurrentRun.ID = strings.TrimSpace(p.CurrentRun.ID)
		p.CurrentRun.StartedAt = strings.TrimSpace(p.CurrentRun.StartedAt)
		if p.CurrentRun.ID == "" {
			p.CurrentRun = nil
		}
	}
	if p.BootstrapAttributes == nil {
		p.BootstrapAttributes = map[string]string{}
	}
	p.EnsureDefaults()
}

func uniqStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

type legacyConfig struct {
	CurrentContext string                    `yaml:"current-context"`
	Contexts       map[string]*legacyContext `yaml:"contexts"`
}

type legacyContext struct {
	Grafana  legacyGrafanaConfig `yaml:"grafana"`
	Sources  map[string]string   `yaml:"sources,omitempty"`
	Defaults legacyDefaults      `yaml:"defaults,omitempty"`
}

type legacyGrafanaConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token,omitempty"`
}

type legacyDefaults struct {
	Since  string            `yaml:"since,omitempty"`
	Limit  int               `yaml:"limit,omitempty"`
	Output string            `yaml:"output,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

func migrateLegacy(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	legacy := &legacyConfig{}
	if err := yaml.Unmarshal(b, legacy); err != nil {
		return nil, err
	}

	out := DefaultConfig()
	names := make([]string, 0, len(legacy.Contexts))
	for name := range legacy.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		ctx := legacy.Contexts[name]
		if ctx == nil {
			continue
		}
		if looksLocalGrafana(ctx.Grafana.URL) {
			continue
		}

		project := &Project{
			Name:    name,
			Sources: cloneStringMap(ctx.Sources),
			Defaults: DefaultsConfig{
				Since:  ctx.Defaults.Since,
				Limit:  ctx.Defaults.Limit,
				Output: ctx.Defaults.Output,
			},
		}
		if svc := firstNonEmpty(
			ctx.Defaults.Labels["service_name"],
			ctx.Defaults.Labels["service"],
			ctx.Defaults.Labels["resource.service.name"],
		); svc != "" {
			project.PrimaryService = svc
		}
		project.normalize(name)
		out.Projects[name] = project
		if out.Setup.GrafanaAPIURL == "" {
			out.Setup.GrafanaAPIURL = strings.TrimRight(strings.TrimSpace(ctx.Grafana.URL), "/")
		}
	}

	if _, ok := out.Projects[legacy.CurrentContext]; ok {
		out.CurrentProject = legacy.CurrentContext
	}
	return out, nil
}

func looksLocalGrafana(rawURL string) bool {
	rawURL = strings.TrimSpace(strings.ToLower(rawURL))
	return strings.Contains(rawURL, "localhost") || strings.Contains(rawURL, "127.0.0.1")
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
