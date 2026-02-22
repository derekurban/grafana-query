package grafquery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/derekurban/grafana-query/internal/cfg"
	"github.com/derekurban/grafana-query/internal/grafana"
	"github.com/spf13/cobra"
)

func newInitCmd(opts *GlobalOptions) *cobra.Command {
	var urlFlag, tokenFlag, ctxName string
	var nonInteractive bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize config and auto-discover Grafana data sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := resolveConfigPath(opts.ConfigPath)
			if err != nil {
				return err
			}
			c, err := cfg.Load(cfgPath)
			if err != nil {
				return err
			}
			if strings.TrimSpace(ctxName) == "" {
				ctxName = "production"
			}

			if strings.TrimSpace(urlFlag) == "" {
				if nonInteractive {
					return fmt.Errorf("--url required in non-interactive mode")
				}
				fmt.Print("Grafana URL: ")
				fmt.Scanln(&urlFlag)
			}
			if strings.TrimSpace(tokenFlag) == "" {
				if nonInteractive {
					return fmt.Errorf("--token required in non-interactive mode")
				}
				fmt.Print("Service account token: ")
				fmt.Scanln(&tokenFlag)
			}

			client := grafana.New(urlFlag, tokenFlag)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			ds, err := client.GetDataSources(ctx)
			if err != nil {
				return fmt.Errorf("discovery failed: %w", err)
			}

			sources := map[string]string{}
			for _, d := range ds {
				switch strings.ToLower(d.Type) {
				case "loki":
					if sources["logs"] == "" {
						sources["logs"] = d.UID
					}
				case "prometheus":
					if sources["metrics"] == "" {
						sources["metrics"] = d.UID
					}
				case "tempo":
					if sources["traces"] == "" {
						sources["traces"] = d.UID
					}
				}
			}

			c.Contexts[ctxName] = &cfg.Context{
				Grafana: cfg.GrafanaConfig{
					URL:   urlFlag,
					Token: tokenFlag,
				},
				Sources: sources,
				Defaults: cfg.DefaultsConfig{
					Since:  "1h",
					Limit:  100,
					Output: "auto",
					Labels: map[string]string{},
				},
			}
			c.CurrentContext = ctxName
			if err := cfg.Save(cfgPath, c); err != nil {
				return err
			}

			fmt.Printf("Config written to %s\n", cfgPath)
			fmt.Printf("Current context: %s\n", ctxName)
			fmt.Printf("Discovered %d data sources\n", len(ds))
			for sig, uid := range sources {
				fmt.Printf("  %s -> %s\n", sig, uid)
			}
			fmt.Println("Try:")
			fmt.Println("  grafquery logs '{job=\"myapp\"}' --since 15m")
			fmt.Println("  grafquery metrics 'up{job=\"myapp\"}'")
			fmt.Println("  grafquery traces '{ resource.service.name = \"myapp\" }'")
			return nil
		},
	}
	cmd.Flags().StringVar(&urlFlag, "url", "", "Grafana URL")
	cmd.Flags().StringVar(&tokenFlag, "token", "", "Service account token")
	cmd.Flags().StringVar(&ctxName, "context-name", "production", "Context name")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Fail instead of prompting")
	return cmd
}
