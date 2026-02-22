package grafquery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/derekurban/grafana-query/internal/cfg"
	"github.com/derekurban/grafana-query/internal/grafana"
	"github.com/derekurban/grafana-query/internal/output"
	"github.com/derekurban/grafana-query/internal/util"
)

func resolveConfigPath(flag string) (string, error) {
	if strings.TrimSpace(flag) != "" {
		return flag, nil
	}
	return cfg.DefaultConfigPath()
}

func loadConfigFromFlags(opts *GlobalOptions) (*cfg.Config, string, error) {
	path, err := resolveConfigPath(opts.ConfigPath)
	if err != nil {
		return nil, "", err
	}
	c, err := cfg.Load(path)
	if err != nil {
		return nil, "", err
	}
	return c, path, nil
}

func buildClient(opts *GlobalOptions) (*grafana.Client, *cfg.Config, *cfg.Context, string, error) {
	c, _, err := loadConfigFromFlags(opts)
	if err != nil {
		return nil, nil, nil, "", err
	}
	ctxCfg, name, err := c.ResolveContext(opts.Context)
	if err != nil {
		return nil, c, nil, "", err
	}
	token, err := cfg.ResolveToken(ctxCfg.Grafana)
	if err != nil {
		return nil, c, nil, "", err
	}
	cl := grafana.New(ctxCfg.Grafana.URL, token)
	return cl, c, ctxCfg, name, nil
}

func resolveSignalSource(ctx context.Context, cl *grafana.Client, signal string, cc *cfg.Context) (*grafana.DataSource, error) {
	sources, err := cl.GetDataSources(ctx)
	if err != nil {
		return nil, err
	}
	if uid := strings.TrimSpace(cc.Sources[signal]); uid != "" {
		if ds := grafana.SourceByUID(sources, uid); ds != nil {
			return ds, nil
		}
	}
	wantType := map[string]string{"logs": "loki", "metrics": "prometheus", "traces": "tempo"}[signal]
	for i := range sources {
		if strings.EqualFold(sources[i].Type, wantType) {
			return &sources[i], nil
		}
	}
	return nil, fmt.Errorf("no datasource mapped for signal %q", signal)
}

func renderByOutput(mode string, payload map[string]any, rows []map[string]any) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "json":
		return output.PrintJSON(rows)
	case "raw":
		return output.PrintRaw(payload)
	case "csv":
		return output.PrintCSV(rows, []string{"Time", "ts", "line", "value"})
	case "table", "auto", "":
		output.PrintTable(rows, []string{"Time", "ts", "line", "value", "service", "level", "message"})
		return nil
	default:
		return fmt.Errorf("unsupported output mode: %s", mode)
	}
}

func parseRawJSONMap(s string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func maybeApplyAliasAndLabels(expr, signal string, c *cfg.Config, ctxCfg *cfg.Context) string {
	expr = util.ExpandAlias(expr, c.Aliases)
	if signal == "logs" || signal == "traces" {
		expr = util.InjectDefaultLabels(expr, ctxCfg.Defaults.Labels)
	}
	return expr
}

func printDataSources(sources []grafana.DataSource) {
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Type == sources[j].Type {
			return sources[i].Name < sources[j].Name
		}
		return sources[i].Type < sources[j].Type
	})
	rows := make([]map[string]any, 0, len(sources))
	for _, s := range sources {
		rows = append(rows, map[string]any{
			"uid":  s.UID,
			"type": s.Type,
			"name": s.Name,
			"url":  s.URL,
		})
	}
	output.PrintTable(rows, []string{"uid", "type", "name", "url"})
}

func mustGetEnvOrPrompt(name, prompt string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v, nil
	}
	fmt.Print(prompt)
	var in string
	_, err := fmt.Scanln(&in)
	return strings.TrimSpace(in), err
}
