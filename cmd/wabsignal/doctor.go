package wabsignal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/derekurban/wabii-signal/internal/cfg"
	"github.com/derekurban/wabii-signal/internal/grafana"
	"github.com/derekurban/wabii-signal/internal/output"
	"github.com/spf13/cobra"
)

func newDoctorCmd(opts *GlobalOptions) *cobra.Command {
	var skipSmokeTest bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate hosted Grafana access, datasource mappings, and OTLP write/read-back",
		Long: strings.TrimSpace(`
Validate the current machine setup and, when a project is selected, the
project-specific write path.

Doctor checks:

- Grafana HTTP API reachability
- logs/metrics/traces datasource resolution
- project write-token presence
- OTLP write and Grafana read-back using a smoke-test trace

This is the fastest way to answer "is the stack configured correctly?" before
debugging a real app run.
`),
		Example: strings.TrimSpace(`
  wabsignal doctor
  wabsignal doctor --project shop-api
  wabsignal doctor --skip-smoke-test
  wabsignal doctor --output json
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			config, _, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			var projectName string
			var project *cfg.Project
			if strings.TrimSpace(opts.Project) != "" || strings.TrimSpace(config.CurrentProject) != "" {
				project, projectName, err = config.ResolveProject(opts.Project)
				if err != nil {
					return err
				}
			}
			client, err := buildSetupClient(config)
			if err != nil {
				return err
			}

			checks := make([]map[string]any, 0, 8)
			failed := false
			record := func(name string, ok bool, detail string) {
				if !ok {
					failed = true
				}
				checks = append(checks, map[string]any{
					"name":   name,
					"status": doctorStatus(ok),
					"detail": detail,
				})
			}

			ctx, cancel := timeoutContext(45)
			defer cancel()

			health, err := client.GetHealth(ctx)
			if err != nil {
				record("grafana_http_api", false, err.Error())
			} else {
				record("grafana_http_api", true, fmt.Sprintf("database=%s version=%s", health.Database, health.Version))
			}

			var traceSource *grafana.DataSource
			for _, signal := range []string{"logs", "metrics", "traces"} {
				ds, resolveErr := resolveSignalSource(ctx, client, signal, config, project)
				if resolveErr != nil {
					record("datasource_"+signal, false, resolveErr.Error())
					continue
				}
				record("datasource_"+signal, true, fmt.Sprintf("%s (%s)", ds.UID, ds.Type))
				if signal == "traces" {
					traceSource = ds
				}
			}

			var smokeResult any
			if project == nil {
				record("project_scope", true, "no current project selected; project-specific checks skipped")
				record("otlp_trace_smoke", true, "skipped: no current project selected")
			} else if strings.TrimSpace(project.WriteToken) == "" {
				record("project_write_token", false, "write token is not configured")
				record("otlp_trace_smoke", false, "cannot validate OTLP ingest without a project write token")
			} else if skipSmokeTest {
				record("project_write_token", true, redactSecret(project.WriteToken))
				record("otlp_trace_smoke", true, "skipped")
			} else if traceSource == nil {
				record("project_write_token", true, redactSecret(project.WriteToken))
				record("otlp_trace_smoke", false, "trace datasource is not available")
			} else {
				record("project_write_token", true, redactSecret(project.WriteToken))
				result, smokeErr := runOTLPTraceSmokeTest(ctx, config, projectName, project, traceSource, client, true)
				smokeResult = result
				if smokeErr != nil {
					record("otlp_trace_smoke", false, smokeErr.Error())
				} else {
					record("otlp_trace_smoke", true, fmt.Sprintf("trace_id=%s", result.TraceID))
				}
			}

			var currentRun any
			if project != nil {
				currentRun = project.CurrentRun
			}

			payload := map[string]any{
				"project":      projectName,
				"setup_mode":   config.Setup.Mode,
				"grafana_api":  config.Setup.GrafanaAPIURL,
				"otlp":         config.Setup.OTLPEndpoint,
				"current_run":  currentRun,
				"checks":       checks,
				"smoke_result": smokeResult,
			}
			if isJSONOutput(opts.Output) {
				if err := output.PrintJSON(payload); err != nil {
					return err
				}
			} else {
				output.PrintTable(checks, []string{"name", "status", "detail"})
				if smokeResult != nil {
					fmt.Println()
					fmt.Println("Smoke test:")
					if resultMap, ok := smokeResult.(*traceSmokeResult); ok {
						_ = output.PrintJSON(resultMap)
					}
				}
			}
			if failed {
				return errors.New("doctor detected failing checks")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&skipSmokeTest, "skip-smoke-test", false, "Skip OTLP trace write and read-back validation")
	return cmd
}

func doctorStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "fail"
}
