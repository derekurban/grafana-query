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
	OrgID           string
	GrafanaAPIURL   string
	StackName       string
	OTLPEndpoint    string
	OTLPInstanceID  string
	QueryToken      string
	ManagementToken string
	CloudStackID    string
	CloudRegion     string
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

	introForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Welcome").
				Description(strings.TrimSpace(`
wabsignal setup is a human-only machine bootstrap.

This wizard will walk you through the Grafana Cloud pages to open, what values
to copy, and where each value gets used. After setup is complete, agents can
use project, run, doctor, logs, metrics, traces, query, and correlate without
touching machine-level credentials.
`)).
				Next(true).
				NextLabel("Continue"),
		).Title("Human-only setup"),
	)
	if err := introForm.Run(); err != nil {
		return err
	}

	modeAndOrgForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Access mode").
				Description("Restrictive keeps project write tokens manual. Full-access lets wabii-signal create and rotate project write tokens for you.").
				Options(
					huh.NewOption("Restrictive", cfg.ModeRestrictive),
					huh.NewOption("Full access", cfg.ModeFullAccess),
				).
				Value(&state.Mode),
			huh.NewInput().
				Title("Grafana organization ID").
				Description("Used to build the Grafana Cloud pages this wizard will send you to. Example: derekurban or an org identifier you use in grafana.com URLs.").
				Value(&state.OrgID).
				Validate(required("Grafana organization ID")),
		).Title("Step 1: Choose mode"),
	)
	if err := modeAndOrgForm.Run(); err != nil {
		return err
	}

	stackPortalForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Open your Grafana Cloud organization").
				Description(strings.TrimSpace(fmt.Sprintf(`
Open this page in your browser:

  https://grafana.com/orgs/%s

From there:

1. You will see your stacks.
2. Click "Details" on the stack you want to connect.
3. Keep that stack details page open, because you will use it for both the
   stack name and the OpenTelemetry connection details.
`, strings.TrimSpace(state.OrgID)))).
				Next(true).
				NextLabel("I have the stack details page open"),
		).Title("Step 2: Open the stack"),
	)
	if err := stackPortalForm.Run(); err != nil {
		return err
	}

	otelForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Copy the OpenTelemetry details").
				Description(strings.TrimSpace(`
From the stack details page:

1. Click "OpenTelemetry".
2. Copy the OTLP endpoint URL.
   Example: https://otlp-gateway-prod-region.grafana.net/otlp
3. Copy the OTLP instance ID.
   Example: 1234567
4. Note the stack name as shown in the stack URL or details page.

You will paste those values into the next screen.
`)).
				Next(true).
				NextLabel("I have the OpenTelemetry details"),
		).Title("Step 3: OpenTelemetry"),
	)
	if err := otelForm.Run(); err != nil {
		return err
	}

	if cfg.NormalizeMode(state.Mode) == cfg.ModeFullAccess {
		accessPolicyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Create the full-access policy token").
					Description(strings.TrimSpace(fmt.Sprintf(`
Open this page in your browser:

  https://grafana.com/orgs/%s/access-policies

Create a Grafana Cloud access policy for wabii-signal full-access management.
Make sure it includes access policy management scopes, specifically
"accesspolicies:read" and "accesspolicies:write", then create a token for it.

You will paste that management token into the final setup screen.
`, strings.TrimSpace(state.OrgID)))).
					Next(true).
					NextLabel("I have the full-access policy token"),
			).Title("Step 4: Access policies"),
		)
		if err := accessPolicyForm.Run(); err != nil {
			return err
		}
	}

	serviceAccountForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Create the Grafana read token").
				Description(strings.TrimSpace(fmt.Sprintf(`
Open this page in your browser:

  https://%s.grafana.net/org/serviceaccounts/create

Create a service account with the "Viewer" role, then create a token for it.
Use that token as the Grafana read token for wabii-signal.

Do not use a Grafana Cloud access-policy token as the read token.
`, strings.TrimSpace(state.OrgID)))).
				Next(true).
				NextLabel("I have the Viewer service-account token"),
		).Title("Step 5: Service account"),
	)
	if err := serviceAccountForm.Run(); err != nil {
		return err
	}

	connectionFields := []huh.Field{
		huh.NewInput().
			Title("Stack name").
			Description("The stack subdomain, usually visible on the stack details page. Example: my-stack for https://my-stack.grafana.net").
			Value(&state.StackName).
			Validate(required("stack name")),
		huh.NewInput().
			Title("OTLP endpoint").
			Description("Paste the OTLP endpoint from the stack's OpenTelemetry page. Example: https://otlp-gateway-prod-ca-east-0.grafana.net/otlp").
			Value(&state.OTLPEndpoint).
			Validate(required("OTLP endpoint")),
		huh.NewInput().
			Title("OTLP instance ID").
			Description("Paste the OTLP instance ID from the stack's OpenTelemetry page. This also becomes the stack ID for full-access policy automation.").
			Value(&state.OTLPInstanceID).
			Validate(required("OTLP instance ID")),
		huh.NewInput().
			Title("Grafana read token").
			Description("Paste the Viewer service-account token you created at the Grafana stack service accounts page. Stored in your OS keyring.").
			Value(&state.QueryToken).
			EchoMode(huh.EchoModePassword).
			Validate(required("Grafana read token")),
	}
	if cfg.NormalizeMode(state.Mode) == cfg.ModeFullAccess {
		connectionFields = append(connectionFields,
			huh.NewInput().
				Title("Grafana Cloud management token").
				Description("Paste the full-access Cloud access-policy token you created. Stored in your OS keyring and used only for managed write-token lifecycle.").
				Value(&state.ManagementToken).
				EchoMode(huh.EchoModePassword).
				Validate(required("Grafana Cloud management token")),
		)
	}

	finalTitle := "Step 6: Paste values"
	if cfg.NormalizeMode(state.Mode) != cfg.ModeFullAccess {
		finalTitle = "Step 5: Paste values"
	}
	connectionForm := huh.NewForm(
		huh.NewGroup(connectionFields...).Title(finalTitle),
	)
	if err := connectionForm.Run(); err != nil {
		return err
	}

	state.GrafanaAPIURL = ""
	state.CloudStackID = strings.TrimSpace(state.OTLPInstanceID)
	if strings.TrimSpace(state.CloudRegion) == "" {
		state.CloudRegion = deriveCloudRegionFromOTLPEndpoint(state.OTLPEndpoint)
	}
	return nil
}
