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
	"golang.org/x/sync/errgroup"
)

func newCorrelateCmd(opts *GlobalOptions) *cobra.Command {
	var traceID string
	var service string
	var since, from, to string
	var limit int

	cmd := &cobra.Command{
		Use:   "correlate",
		Short: "Cross-signal correlation (trace-id or service centric)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(traceID) == "" && strings.TrimSpace(service) == "" {
				return fmt.Errorf("set --trace-id or --service")
			}
			if traceID != "" {
				return correlateByTrace(opts, traceID, since, from, to, limit)
			}
			return correlateByService(opts, service, since, from, to, limit)
		},
	}
	cmd.Flags().StringVar(&traceID, "trace-id", "", "Trace ID to correlate")
	cmd.Flags().StringVar(&service, "service", "", "Service to correlate")
	cmd.Flags().StringVar(&since, "since", "30m", "Lookback range")
	cmd.Flags().StringVar(&from, "from", "", "From timestamp")
	cmd.Flags().StringVar(&to, "to", "", "To timestamp")
	cmd.Flags().IntVar(&limit, "limit", 50, "Result limit")
	return cmd
}

func correlateByTrace(opts *GlobalOptions, traceID, since, from, to string, limit int) error {
	cl, _, ctxCfg, _, err := buildClient(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	f, t, err := util.ResolveGrafanaRange(since, from, to)
	if err != nil {
		return err
	}

	traceDS, err := resolveSignalSource(ctx, cl, "traces", ctxCfg)
	if err != nil {
		return err
	}
	logsDS, err := resolveSignalSource(ctx, cl, "logs", ctxCfg)
	if err != nil {
		return err
	}
	metricsDS, err := resolveSignalSource(ctx, cl, "metrics", ctxCfg)
	if err != nil {
		return err
	}

	var traceResp, logsResp, metricsResp map[string]any
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		q := grafana.QueryPayload{RefID: "A", Datasource: map[string]any{"uid": traceDS.UID, "type": traceDS.Type}, Expr: fmt.Sprintf("{ trace_id = \"%s\" }", traceID), QueryType: "range", MaxLines: limit}
		r, e := cl.Query(gctx, grafana.QueryRequest{From: f, To: t, Queries: []grafana.QueryPayload{q}})
		if e == nil {
			traceResp = r
		}
		return e
	})
	g.Go(func() error {
		q := grafana.QueryPayload{RefID: "B", Datasource: map[string]any{"uid": logsDS.UID, "type": logsDS.Type}, Expr: fmt.Sprintf(`{} |= "%s"`, traceID), QueryType: "range", MaxLines: limit}
		r, e := cl.Query(gctx, grafana.QueryRequest{From: f, To: t, Queries: []grafana.QueryPayload{q}})
		if e == nil {
			logsResp = r
		}
		return e
	})
	g.Go(func() error {
		q := grafana.QueryPayload{RefID: "C", Datasource: map[string]any{"uid": metricsDS.UID, "type": metricsDS.Type}, Expr: `sum(rate(http_requests_total{status=~"5.."}[5m]))`, QueryType: "instant", Instant: true}
		r, e := cl.Query(gctx, grafana.QueryRequest{From: f, To: t, Queries: []grafana.QueryPayload{q}})
		if e == nil {
			metricsResp = r
		}
		return e
	})

	if err := g.Wait(); err != nil {
		return err
	}

	traceRows, _ := grafana.FrameRows(traceResp)
	logsRows, _ := grafana.FrameRows(logsResp)
	metricRows, _ := grafana.FrameRows(metricsResp)

	summary := map[string]any{
		"trace_id":      traceID,
		"time_range":    map[string]string{"from": f, "to": t},
		"trace_rows":    len(traceRows),
		"log_rows":      len(logsRows),
		"metric_points": len(metricRows),
		"trace":         traceRows,
		"logs":          logsRows,
		"metrics":       metricRows,
	}

	if opts.Output == "json" || opts.Output == "raw" {
		return output.PrintJSON(summary)
	}

	fmt.Printf("── Trace %s ──\n", traceID)
	fmt.Printf("Trace rows: %d | Log rows: %d | Metric points: %d\n\n", len(traceRows), len(logsRows), len(metricRows))
	fmt.Println("Logs:")
	output.PrintTable(logsRows, []string{"Time", "ts", "line", "message", "service", "level"})
	fmt.Println()
	fmt.Println("Metrics:")
	output.PrintTable(metricRows, []string{"Time", "value", "service"})
	return nil
}

func correlateByService(opts *GlobalOptions, service, since, from, to string, limit int) error {
	cl, _, ctxCfg, _, err := buildClient(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	f, t, err := util.ResolveGrafanaRange(since, from, to)
	if err != nil {
		return err
	}

	logsDS, err := resolveSignalSource(ctx, cl, "logs", ctxCfg)
	if err != nil {
		return err
	}
	metricsDS, err := resolveSignalSource(ctx, cl, "metrics", ctxCfg)
	if err != nil {
		return err
	}
	tracesDS, err := resolveSignalSource(ctx, cl, "traces", ctxCfg)
	if err != nil {
		return err
	}

	logsQ := fmt.Sprintf(`{service="%s"}`, service)
	traceQ := fmt.Sprintf(`{ resource.service.name = "%s" }`, service)
	metricQ := fmt.Sprintf(`sum(rate(http_requests_total{service="%s"}[5m]))`, service)

	queries := []struct {
		name string
		ds   *grafana.DataSource
		expr string
		qt   string
		inst bool
	}{
		{"logs", logsDS, logsQ, "range", false},
		{"metrics", metricsDS, metricQ, "instant", true},
		{"traces", tracesDS, traceQ, "range", false},
	}

	results := map[string][]map[string]any{}
	g, gctx := errgroup.WithContext(ctx)
	for _, q := range queries {
		q := q
		g.Go(func() error {
			payload := grafana.QueryPayload{RefID: strings.ToUpper(q.name[:1]), Datasource: map[string]any{"uid": q.ds.UID, "type": q.ds.Type}, Expr: q.expr, QueryType: q.qt, MaxLines: limit, Instant: q.inst}
			resp, e := cl.Query(gctx, grafana.QueryRequest{From: f, To: t, Queries: []grafana.QueryPayload{payload}})
			if e != nil {
				return e
			}
			rows, _ := grafana.FrameRows(resp)
			results[q.name] = rows
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	summary := map[string]any{
		"service":    service,
		"time_range": map[string]string{"from": f, "to": t},
		"logs":       results["logs"],
		"metrics":    results["metrics"],
		"traces":     results["traces"],
	}
	if opts.Output == "json" || opts.Output == "raw" {
		return output.PrintJSON(summary)
	}

	fmt.Printf("── Service correlate: %s ──\n", service)
	fmt.Printf("Logs: %d | Metrics: %d | Traces: %d\n\n", len(results["logs"]), len(results["metrics"]), len(results["traces"]))
	fmt.Println("Top logs:")
	output.PrintTable(results["logs"], []string{"Time", "ts", "line", "message", "level"})
	fmt.Println()
	fmt.Println("Metrics:")
	output.PrintTable(results["metrics"], []string{"Time", "value", "service"})
	fmt.Println()
	fmt.Println("Traces:")
	output.PrintTable(results["traces"], []string{"traceID", "trace_id", "duration", "service"})
	return nil
}
