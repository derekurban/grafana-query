package wabsignal

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/derekurban/wabii-signal/internal/cfg"
	"github.com/derekurban/wabii-signal/internal/cloudapi"
	"github.com/derekurban/wabii-signal/internal/grafana"
	"github.com/derekurban/wabii-signal/internal/output"
	"github.com/derekurban/wabii-signal/internal/secret"
	"github.com/spf13/cobra"
)

func newSetupCmd(opts *GlobalOptions) *cobra.Command {
	var (
		mode            string
		grafanaAPIURL   string
		stackName       string
		otlpEndpoint    string
		otlpInstanceID  string
		queryToken      string
		managementToken string
		cloudStackID    string
		cloudRegion     string
		cloudOrgSlug    string
		nonInteractive  bool
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure hosted Grafana HTTP API access and OTLP ingest",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode = cfg.NormalizeMode(mode)
			if mode == "" {
				return fmt.Errorf("--mode must be %q or %q", cfg.ModeRestrictive, cfg.ModeFullAccess)
			}

			var err error
			grafanaAPIURL, stackName, err = cfg.NormalizeGrafanaAPIURL(grafanaAPIURL, stackName)
			if err != nil {
				return err
			}
			grafanaAPIURL, err = normalizeURL(grafanaAPIURL, "grafana api url")
			if err != nil {
				return err
			}
			otlpEndpoint, err = promptOrValue(otlpEndpoint, "OTLP endpoint", false, nonInteractive)
			if err != nil {
				return err
			}
			otlpEndpoint, err = normalizeURL(otlpEndpoint, "otlp endpoint")
			if err != nil {
				return err
			}
			otlpInstanceID, err = promptOrValue(otlpInstanceID, "OTLP instance ID", false, nonInteractive)
			if err != nil {
				return err
			}
			queryToken, err = promptOrValue(queryToken, "Grafana service account token", true, nonInteractive)
			if err != nil {
				return err
			}

			if mode == cfg.ModeFullAccess {
				managementToken, err = promptOrValue(managementToken, "Grafana Cloud access-policy management token", true, nonInteractive)
				if err != nil {
					return err
				}
				cloudStackID, err = promptOrValue(cloudStackID, "Grafana Cloud stack ID", false, nonInteractive)
				if err != nil {
					return err
				}
				cloudRegion, err = promptOrValue(cloudRegion, "Grafana Cloud region", false, nonInteractive)
				if err != nil {
					return err
				}
			}

			ctx, cancel := timeoutContext(25)
			defer cancel()

			client := grafana.New(grafanaAPIURL, queryToken)
			health, err := client.GetHealth(ctx)
			if err != nil {
				return fmt.Errorf("grafana http api validation failed: %w", err)
			}
			discoveredSources, sources, err := discoverSignalSources(ctx, client)
			if err != nil {
				return fmt.Errorf("datasource discovery failed: %w", err)
			}
			setupConfig := cfg.SetupConfig{
				Mode:           mode,
				GrafanaAPIURL:  grafanaAPIURL,
				StackName:      stackName,
				OTLPEndpoint:   strings.TrimSpace(otlpEndpoint),
				OTLPInstanceID: strings.TrimSpace(otlpInstanceID),
				Sources:        discoveredSources,
				Cloud: cfg.CloudSetupConfig{
					OrgSlug: strings.TrimSpace(cloudOrgSlug),
					StackID: strings.TrimSpace(cloudStackID),
					Region:  strings.TrimSpace(cloudRegion),
				},
			}

			writeValidation := map[string]any{
				"status": "deferred",
				"reason": "restrictive mode validates OTLP writes when a project write token is attached",
			}
			if mode == cfg.ModeFullAccess {
				cloudClient := cloudapi.New(managementToken)
				if err := cloudClient.Validate(ctx, cloudRegion, cloudStackID); err != nil {
					return fmt.Errorf("grafana cloud policy validation failed: %w", err)
				}
				smokeResult, err := validateFullAccessSetupWritePath(ctx, setupConfig, managementToken, client, sources)
				if err != nil {
					return fmt.Errorf("otlp smoke test failed: %w", err)
				}
				writeValidation = map[string]any{
					"status": "validated",
					"trace":  smokeResult,
				}
			}

			config, path, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			config.Setup = setupConfig
			for _, project := range config.Projects {
				if project == nil {
					continue
				}
				if project.Sources == nil {
					project.Sources = map[string]string{}
				}
				for signal, uid := range discoveredSources {
					if strings.TrimSpace(project.Sources[signal]) == "" {
						project.Sources[signal] = uid
					}
				}
			}

			if err := secret.SetQueryToken(queryToken); err != nil {
				return fmt.Errorf("failed to store read token in keyring: %w", err)
			}
			if mode == cfg.ModeFullAccess {
				if err := secret.SetManagementToken(managementToken); err != nil {
					return fmt.Errorf("failed to store policy token in keyring: %w", err)
				}
			} else if err := secret.DeleteManagementToken(); err != nil {
				return fmt.Errorf("failed to clear unused policy token: %w", err)
			}

			if err := cfg.Save(path, config); err != nil {
				return err
			}

			payload := map[string]any{
				"setup":              formatSetupSummary(mode, config)[0],
				"grafana_health":     health,
				"discovered_sources": discoveredSources,
				"keyring":            setupSecretSummary(mode, queryToken, managementToken),
				"write_validation":   writeValidation,
				"config_path":        path,
				"setup_complete":     config.SetupComplete(),
				"next_recommended":   "wabsignal project create <project-name> <primary-service> [extra-services...]",
			}
			if isJSONOutput(opts.Output) {
				return output.PrintJSON(payload)
			}

			fmt.Println("Hosted Grafana setup saved.")
			output.PrintTable(formatSetupSummary(mode, config), []string{
				"mode", "grafana_api_url", "stack_name", "otlp_endpoint", "otlp_instance_id", "cloud_stack_id", "cloud_region",
			})
			fmt.Println()
			fmt.Println("Discovered data sources:")
			rows := make([]map[string]any, 0, len(discoveredSources))
			for signal, uid := range discoveredSources {
				rows = append(rows, map[string]any{"signal": signal, "uid": uid})
			}
			output.PrintTable(rows, []string{"signal", "uid"})
			fmt.Println()
			fmt.Println("Secrets stored in OS keyring:")
			fmt.Printf("  read token: %s\n", redactSecret(queryToken))
			if mode == cfg.ModeFullAccess {
				fmt.Printf("  policy token: %s\n", redactSecret(managementToken))
			}
			fmt.Println()
			fmt.Printf("Grafana health: database=%s version=%s\n", health.Database, health.Version)
			if mode == cfg.ModeFullAccess {
				if trace, ok := writeValidation["trace"].(*traceSmokeResult); ok && trace != nil {
					fmt.Printf("OTLP write path validated with smoke trace %s.\n", trace.TraceID)
				}
			} else {
				fmt.Println("OTLP write validation is deferred until `wabsignal project create` attaches a project write token.")
			}
			fmt.Println("Next:")
			fmt.Println("  wabsignal project create <project-name> <primary-service> [extra-services...]")
			return nil
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "", "Access mode: restrictive or full-access")
	cmd.Flags().StringVar(&grafanaAPIURL, "grafana-api-url", "", "Grafana stack URL or full /api/ds/query URL")
	cmd.Flags().StringVar(&stackName, "stack-name", "", "Grafana stack name used to construct https://<stack>.grafana.net")
	cmd.Flags().StringVar(&otlpEndpoint, "otlp-endpoint", "", "Grafana Cloud OTLP endpoint")
	cmd.Flags().StringVar(&otlpInstanceID, "otlp-instance-id", "", "Grafana Cloud OTLP instance ID")
	cmd.Flags().StringVar(&queryToken, "query-token", "", "Grafana service account token for HTTP API reads")
	cmd.Flags().StringVar(&managementToken, "policy-token", "", "Grafana Cloud access-policy management token (full-access only)")
	cmd.Flags().StringVar(&cloudStackID, "cloud-stack-id", "", "Grafana Cloud numeric stack ID")
	cmd.Flags().StringVar(&cloudRegion, "cloud-region", "", "Grafana Cloud region")
	cmd.Flags().StringVar(&cloudOrgSlug, "cloud-org-slug", "", "Grafana Cloud org slug (optional metadata)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Fail instead of prompting for missing values")
	return cmd
}

func normalizeURL(rawURL, label string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid %s: %w", label, err)
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	if parsed.Host == "" && parsed.Path != "" {
		parsed.Host = parsed.Path
		parsed.Path = ""
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func setupSecretSummary(mode, queryToken, managementToken string) map[string]any {
	summary := map[string]any{
		"read_token": redactSecret(queryToken),
	}
	if cfg.NormalizeMode(mode) == cfg.ModeFullAccess {
		summary["policy_token"] = redactSecret(managementToken)
	}
	return summary
}

func validateFullAccessSetupWritePath(ctx context.Context, setup cfg.SetupConfig, managementToken string, readClient *grafana.Client, sources []grafana.DataSource) (*traceSmokeResult, error) {
	traceSource := resolveTraceSourceForValidation(&cfg.Config{Setup: setup}, nil, sources)
	if traceSource == nil {
		return nil, fmt.Errorf("no trace datasource discovered; configure Tempo in Grafana first")
	}

	project, cleanup, err := createSetupValidationProject(ctx, setup, managementToken)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = cleanup()
	}()

	return runOTLPTraceSmokeTest(ctx, &cfg.Config{Setup: setup}, project.Name, project, traceSource, readClient, true)
}

func createSetupValidationProject(ctx context.Context, setup cfg.SetupConfig, managementToken string) (*cfg.Project, func() error, error) {
	client := cloudapi.New(managementToken)
	suffix, err := randomHex(4)
	if err != nil {
		return nil, nil, err
	}

	projectName := "setup-smoke-" + suffix
	serviceName := "wabsignal-setup-" + suffix
	policyName := managedResourceName(projectName, serviceName, "write")

	policy, err := client.CreateAccessPolicy(ctx, setup.Cloud.Region, cloudapi.CreateAccessPolicyRequest{
		Name:        policyName,
		DisplayName: fmt.Sprintf("wabsignal setup smoke %s", suffix),
		Scopes:      managedWriteScopes,
		Realms: []cloudapi.AccessPolicyRealm{
			{Type: "stack", Identifier: setup.Cloud.StackID},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create setup smoke access policy: %w", err)
	}

	token, err := client.CreateToken(ctx, policy.ID, cloudapi.CreateTokenRequest{
		Name:        managedResourceName(projectName, serviceName, "token"),
		DisplayName: fmt.Sprintf("wabsignal setup smoke token %s", suffix),
	})
	if err != nil {
		cleanupCtx, cancel := timeoutContext(20)
		defer cancel()
		_ = client.DeleteAccessPolicy(cleanupCtx, policy.ID)
		return nil, nil, fmt.Errorf("failed to create setup smoke token: %w", err)
	}

	project := &cfg.Project{
		Name:              projectName,
		PrimaryService:    serviceName,
		WriteToken:        strings.TrimSpace(token.Key),
		ManagedWriteToken: true,
		ManagedPolicyID:   policy.ID,
		ManagedPolicyName: policy.Name,
		ManagedTokenID:    token.ID,
		ManagedTokenName:  token.Name,
	}
	project.EnsureDefaults()

	cleanup := func() error {
		cleanupCtx, cancel := timeoutContext(20)
		defer cancel()
		return client.DeleteAccessPolicy(cleanupCtx, policy.ID)
	}
	return project, cleanup, nil
}
