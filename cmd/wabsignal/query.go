package wabsignal

import (
	"fmt"
	"strings"

	"github.com/derekurban/wabii-signal/internal/grafana"
	"github.com/derekurban/wabii-signal/internal/util"
	"github.com/spf13/cobra"
)

func newQueryCmd(opts *GlobalOptions) *cobra.Command {
	var source string
	var from, to, since string
	var queryType string
	var rawPayload string
	var listSources bool
	var describe string
	var maxLines int

	cmd := &cobra.Command{
		Use:   "query <expr>",
		Short: "Run a raw Grafana datasource query through the stack HTTP API",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, _, _, _, err := buildClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := timeoutContext(30)
			defer cancel()

			sources, err := cl.GetDataSources(ctx)
			if err != nil {
				return err
			}
			if listSources {
				printDataSources(sources)
				return nil
			}
			if strings.TrimSpace(describe) != "" {
				ds, err := grafana.ResolveSourceByNameOrUID(sources, describe)
				if err != nil {
					return err
				}
				fmt.Printf("UID: %s\nType: %s\nName: %s\nURL: %s\nDatabase: %s\nAccess: %s\n", ds.UID, ds.Type, ds.Name, ds.URL, ds.Database, ds.Access)
				return nil
			}
			if strings.TrimSpace(source) == "" {
				return fmt.Errorf("--source is required")
			}
			ds, err := grafana.ResolveSourceByNameOrUID(sources, source)
			if err != nil {
				return err
			}

			f, t, err := util.ResolveGrafanaRange(since, from, to)
			if err != nil {
				return err
			}

			payload := grafana.QueryPayload{
				RefID:      "A",
				Datasource: map[string]any{"uid": ds.UID, "type": ds.Type},
				QueryType:  queryType,
				MaxLines:   maxLines,
			}
			if strings.TrimSpace(rawPayload) != "" {
				rawMap, err := parseRawJSONMap(rawPayload)
				if err != nil {
					return fmt.Errorf("invalid --raw-payload: %w", err)
				}
				payload.Raw = rawMap
				if payload.Expr == "" {
					if expr, ok := rawMap["expr"].(string); ok {
						payload.Expr = expr
					}
				}
			} else {
				if len(args) != 1 {
					return fmt.Errorf("expr argument required unless --raw-payload is set")
				}
				payload.Expr = strings.TrimSpace(args[0])
			}

			resp, err := cl.Query(ctx, grafana.QueryRequest{From: f, To: t, Queries: []grafana.QueryPayload{payload}})
			if err != nil {
				return err
			}
			rows, _ := grafana.FrameRows(resp)
			mode := opts.Output
			if mode == "auto" && rawPayload != "" {
				mode = "raw"
			}
			return renderByOutput(mode, resp, rows)
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Datasource name or UID")
	cmd.Flags().StringVar(&from, "from", "", "From timestamp (RFC3339, unix ms, now-...)")
	cmd.Flags().StringVar(&to, "to", "", "To timestamp (RFC3339, unix ms, now)")
	cmd.Flags().StringVar(&since, "since", "1h", "Relative range lookback (for example 30m, 6h)")
	cmd.Flags().StringVar(&queryType, "query-type", "range", "Query type hint (range|instant)")
	cmd.Flags().StringVar(&rawPayload, "raw-payload", "", "Raw JSON object merged into query payload")
	cmd.Flags().BoolVar(&listSources, "list-sources", false, "List datasources and exit")
	cmd.Flags().StringVar(&describe, "describe", "", "Describe a datasource by UID or name")
	cmd.Flags().IntVar(&maxLines, "max-lines", 100, "Max lines for log queries")
	return cmd
}
