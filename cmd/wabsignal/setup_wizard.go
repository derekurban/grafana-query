package wabsignal

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/derekurban/wabii-signal/internal/cfg"
	"golang.org/x/term"
)

type setupWizardState struct {
	Mode            string
	GrafanaAPIURL   string
	StackName       string
	OTLPEndpoint    string
	OTLPInstanceID  string
	QueryToken      string
	ManagementToken string
	CloudStackID    string
	CloudRegion     string
	CloudOrgSlug    string
}

func shouldUseSetupWizard(nonInteractive bool, output string) bool {
	if nonInteractive || isJSONOutput(output) {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func runSetupWizard(state *setupWizardState) error {
	if state == nil {
		return errors.New("setup wizard state is required")
	}
	if cfg.NormalizeMode(state.Mode) == "" {
		state.Mode = cfg.ModeRestrictive
	}

	fmt.Fprintln(os.Stdout, "wabsignal setup is a human-only guided wizard.")
	fmt.Fprintln(os.Stdout, "Run this once per machine as an operator, then let agents use project/query/doctor commands against the configured machine.")
	fmt.Fprintln(os.Stdout)

	required := func(label string) func(string) error {
		return func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s is required", label)
			}
			return nil
		}
	}

	baseForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Setup mode").
				Description("Restrictive keeps project write tokens manual. Full-access lets wabsignal create and rotate project write tokens for you.").
				Options(
					huh.NewOption("Restrictive", cfg.ModeRestrictive),
					huh.NewOption("Full access", cfg.ModeFullAccess),
				).
				Value(&state.Mode),
			huh.NewInput().
				Title("Grafana stack name").
				Description("Preferred. Example: my-stack. Leave blank if you want to provide a full Grafana URL instead.").
				Value(&state.StackName),
			huh.NewInput().
				Title("Grafana API URL").
				Description("Optional override. Example: https://my-stack.grafana.net or https://my-stack.grafana.net/api/ds/query").
				Value(&state.GrafanaAPIURL),
			huh.NewInput().
				Title("OTLP endpoint").
				Description("Grafana OTLP ingest base URL. Example: https://otlp-gateway-prod-us-central-0.grafana.net/otlp").
				Value(&state.OTLPEndpoint).
				Validate(required("OTLP endpoint")),
			huh.NewInput().
				Title("OTLP instance ID").
				Description("Grafana Cloud OTLP instance ID used when building OTLP auth headers for each project write token.").
				Value(&state.OTLPInstanceID).
				Validate(required("OTLP instance ID")),
			huh.NewInput().
				Title("Grafana read token").
				Description("Human-only secret. Stored in your OS keyring and used by wabsignal to query logs, metrics, and traces through the Grafana HTTP API.").
				Value(&state.QueryToken).
				EchoMode(huh.EchoModePassword).
				Validate(required("Grafana read token")),
		).Title("Machine Setup"),
	)
	if err := baseForm.Run(); err != nil {
		return err
	}
	if strings.TrimSpace(state.StackName) == "" && strings.TrimSpace(state.GrafanaAPIURL) == "" {
		return errors.New("grafana stack name or grafana api url is required")
	}

	if cfg.NormalizeMode(state.Mode) != cfg.ModeFullAccess {
		return nil
	}

	fullAccessForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Grafana Cloud policy token").
				Description("Human-only secret. Stored in your OS keyring and used only for creating and rotating managed project write tokens.").
				Value(&state.ManagementToken).
				EchoMode(huh.EchoModePassword).
				Validate(required("Grafana Cloud policy token")),
			huh.NewInput().
				Title("Grafana Cloud stack ID").
				Description("Numeric stack identifier required by the Grafana Cloud access policy API.").
				Value(&state.CloudStackID).
				Validate(required("Grafana Cloud stack ID")),
			huh.NewInput().
				Title("Grafana Cloud region").
				Description("Region identifier used by the Grafana Cloud access policy API. Example: us").
				Value(&state.CloudRegion).
				Validate(required("Grafana Cloud region")),
			huh.NewInput().
				Title("Grafana Cloud org slug").
				Description("Optional metadata only. Safe to leave blank unless you want it recorded in config.").
				Value(&state.CloudOrgSlug),
		).Title("Full-Access Management"),
	)
	return fullAccessForm.Run()
}
