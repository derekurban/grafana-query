package grafquery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/derekurban/grafana-query/internal/grafana"
	"github.com/derekurban/grafana-query/internal/output"
	"github.com/derekurban/grafana-query/internal/util"
	"github.com/spf13/cobra"
)

func newDashCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "dash", Short: "Run and inspect Grafana dashboard queries"}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List dashboards",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, _, _, _, err := buildClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			d, err := cl.SearchDashboards(ctx)
			if err != nil {
				return err
			}
			rows := []map[string]any{}
			for _, it := range d {
				rows = append(rows, map[string]any{"uid": it.UID, "title": it.Title, "type": it.Type, "uri": it.URI})
			}
			output.PrintTable(rows, []string{"uid", "title", "type", "uri"})
			return nil
		},
	})

	var panelName, since string
	cmdRun := &cobra.Command{
		Use:   "run <dashboard-uid>",
		Short: "Run queries from a dashboard panel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, _, ctxCfg, _, err := buildClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			db, err := cl.GetDashboardByUID(ctx, args[0])
			if err != nil {
				return err
			}
			dashObj, _ := db["dashboard"].(map[string]any)
			panels, _ := dashObj["panels"].([]any)
			if len(panels) == 0 {
				return fmt.Errorf("dashboard has no panels")
			}

			var selected map[string]any
			for _, p := range panels {
				pm, _ := p.(map[string]any)
				title, _ := pm["title"].(string)
				if panelName == "" || strings.EqualFold(title, panelName) {
					selected = pm
					break
				}
			}
			if selected == nil {
				return fmt.Errorf("panel %q not found", panelName)
			}

			targets, _ := selected["targets"].([]any)
			if len(targets) == 0 {
				return fmt.Errorf("panel has no targets")
			}
			sources, err := cl.GetDataSources(ctx)
			if err != nil {
				return err
			}

			f, t, err := util.ResolveGrafanaRange(since, "", "")
			if err != nil {
				return err
			}

			rowsOut := []map[string]any{}
			for idx, tr := range targets {
				tm, _ := tr.(map[string]any)
				expr, _ := tm["expr"].(string)
				if strings.TrimSpace(expr) == "" {
					continue
				}
				dsUID := ""
				if dsm, ok := tm["datasource"].(map[string]any); ok {
					if u, ok := dsm["uid"].(string); ok {
						dsUID = u
					}
				}
				if dsUID == "" {
					if u, ok := ctxCfg.Sources["metrics"]; ok {
						dsUID = u
					}
				}
				ds := grafana.SourceByUID(sources, dsUID)
				if ds == nil {
					continue
				}
				ref := fmt.Sprintf("%c", 'A'+idx)
				payload := grafana.QueryPayload{RefID: ref, Datasource: map[string]any{"uid": ds.UID, "type": ds.Type}, Expr: expr, QueryType: "range"}
				resp, err := cl.Query(ctx, grafana.QueryRequest{From: f, To: t, Queries: []grafana.QueryPayload{payload}})
				if err != nil {
					return err
				}
				rows, _ := grafana.FrameRows(resp)
				for _, r := range rows {
					r["panel"] = selected["title"]
					r["refId"] = ref
					r["expr"] = expr
					rowsOut = append(rowsOut, r)
				}
			}
			if opts.Output == "json" || opts.Output == "raw" {
				return output.PrintJSON(rowsOut)
			}
			output.PrintTable(rowsOut, []string{"panel", "refId", "Time", "value", "expr"})
			return nil
		},
	}
	cmdRun.Flags().StringVar(&panelName, "panel", "", "Panel title (default first panel)")
	cmdRun.Flags().StringVar(&since, "since", "1h", "Range lookback for dashboard queries")
	cmd.AddCommand(cmdRun)

	return cmd
}
