package grafquery

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

type GlobalOptions struct {
	ConfigPath string
	Context    string
	Output     string
}

func NewRootCmd() *cobra.Command {
	opts := &GlobalOptions{}
	cmd := &cobra.Command{
		Use:   "grafquery",
		Short: "Unified Grafana query CLI (logs, metrics, traces, correlate, dashboards)",
	}

	cmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "Path to config file (default: ~/.config/grafquery/config.yaml)")
	cmd.PersistentFlags().StringVar(&opts.Context, "context", "", "Override config context")
	cmd.PersistentFlags().StringVarP(&opts.Output, "output", "o", "auto", "Output mode: auto|json|table|raw|csv")

	cmd.AddCommand(newInitCmd(opts))
	cmd.AddCommand(newConfigCmd(opts))
	cmd.AddCommand(newQueryCmd(opts))
	cmd.AddCommand(newSignalCmd(opts, "logs"))
	cmd.AddCommand(newSignalCmd(opts, "metrics"))
	cmd.AddCommand(newSignalCmd(opts, "traces"))
	cmd.AddCommand(newCorrelateCmd(opts))
	cmd.AddCommand(newDashCmd(opts))
	cmd.AddCommand(newLocalCmd(opts))
	cmd.AddCommand(newVersionCmd())

	return cmd
}
