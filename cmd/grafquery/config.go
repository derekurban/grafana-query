package grafquery

import (
	"fmt"
	"sort"

	"github.com/derekurban/grafana-query/internal/cfg"
	"github.com/derekurban/grafana-query/internal/output"
	"github.com/spf13/cobra"
)

func newConfigCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage grafquery config and contexts"}

	cmd.AddCommand(&cobra.Command{
		Use:   "current",
		Short: "Show current context",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			ctx, name, err := c.ResolveContext(opts.Context)
			if err != nil {
				return err
			}
			return output.PrintJSON(map[string]any{
				"current-context": name,
				"url":             ctx.Grafana.URL,
				"sources":         ctx.Sources,
				"defaults":        ctx.Defaults,
			})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			rows := []map[string]any{}
			names := make([]string, 0, len(c.Contexts))
			for n := range c.Contexts {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				ctx := c.Contexts[n]
				rows = append(rows, map[string]any{
					"name":    n,
					"current": n == c.CurrentContext,
					"url":     ctx.Grafana.URL,
				})
			}
			output.PrintTable(rows, []string{"name", "current", "url"})
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "use <context>",
		Short: "Switch current context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, p, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			if _, ok := c.Contexts[args[0]]; !ok {
				return fmt.Errorf("context %q not found", args[0])
			}
			c.CurrentContext = args[0]
			if err := cfg.Save(p, c); err != nil {
				return err
			}
			fmt.Printf("Switched current context to %s\n", args[0])
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set-source <context> <signal> <uid>",
		Short: "Set source UID mapping (logs|metrics|traces) for a context",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, p, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			ctx, ok := c.Contexts[args[0]]
			if !ok {
				return fmt.Errorf("context %q not found", args[0])
			}
			if ctx.Sources == nil {
				ctx.Sources = map[string]string{}
			}
			ctx.Sources[args[1]] = args[2]
			if err := cfg.Save(p, c); err != nil {
				return err
			}
			fmt.Printf("Set %s source for %s -> %s\n", args[1], args[0], args[2])
			return nil
		},
	})

	return cmd
}
