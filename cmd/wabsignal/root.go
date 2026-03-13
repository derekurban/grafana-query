package wabsignal

import (
	"fmt"
	"os"
	"strings"

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
	Project    string
	Output     string
}

func NewRootCmd() *cobra.Command {
	opts := &GlobalOptions{}
	cmd := &cobra.Command{
		Use:           "wabsignal",
		Short:         "Hosted Grafana signal CLI for app debugging evidence",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if exemptFromSetupCheck(cmd) {
				return nil
			}
			return requireSetup(opts)
		},
	}

	cmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "Path to config file (default: ~/.config/wabsignal/config.yaml)")
	cmd.PersistentFlags().StringVar(&opts.Project, "project", "", "Override current project")
	cmd.PersistentFlags().StringVarP(&opts.Output, "output", "o", "auto", "Output mode: auto|json|table|raw|csv")

	cmd.AddCommand(newSetupCmd(opts))
	cmd.AddCommand(newProjectCmd(opts))
	cmd.AddCommand(newRunCmd(opts))
	cmd.AddCommand(newDoctorCmd(opts))
	cmd.AddCommand(newQueryCmd(opts))
	cmd.AddCommand(newSignalCmd(opts, "logs"))
	cmd.AddCommand(newSignalCmd(opts, "metrics"))
	cmd.AddCommand(newSignalCmd(opts, "traces"))
	cmd.AddCommand(newCorrelateCmd(opts))
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func exemptFromSetupCheck(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "setup", "version", "help", "init":
			return true
		}
	}
	return strings.TrimSpace(cmd.Name()) == ""
}
