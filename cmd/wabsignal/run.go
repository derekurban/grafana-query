package wabsignal

import (
	"fmt"
	"time"

	"github.com/derekurban/wabii-signal/internal/cfg"
	"github.com/derekurban/wabii-signal/internal/output"
	"github.com/spf13/cobra"
)

func newRunCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Manage the current project run/session scope",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "start [run-id]",
		Short: "Start or replace the current run scope for the active project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config, path, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			project, projectName, err := config.ResolveProject(opts.Project)
			if err != nil {
				return err
			}

			runID := firstArg(args)
			if runID == "" {
				runID, err = generateRunID()
				if err != nil {
					return err
				}
			}

			project.CurrentRun = &cfg.RunState{
				ID:        runID,
				StartedAt: time.Now().UTC().Format(time.RFC3339),
			}
			if err := cfg.Save(path, config); err != nil {
				return err
			}

			payload := map[string]any{
				"project":    projectName,
				"run_id":     project.CurrentRun.ID,
				"started_at": project.CurrentRun.StartedAt,
			}
			if isJSONOutput(opts.Output) {
				return output.PrintJSON(payload)
			}
			fmt.Printf("Current run for %s: %s\n", projectName, project.CurrentRun.ID)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show the current run scope for the active project",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, _, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			project, projectName, err := config.ResolveProject(opts.Project)
			if err != nil {
				return err
			}

			payload := map[string]any{
				"project":     projectName,
				"current_run": project.CurrentRun,
			}
			if isJSONOutput(opts.Output) {
				return output.PrintJSON(payload)
			}
			if project.CurrentRun == nil {
				fmt.Printf("No current run set for %s.\n", projectName)
				return nil
			}
			output.PrintTable([]map[string]any{
				{
					"project":    projectName,
					"run_id":     project.CurrentRun.ID,
					"started_at": project.CurrentRun.StartedAt,
				},
			}, []string{"project", "run_id", "started_at"})
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Clear the current run scope for the active project",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, path, err := loadConfigFromFlags(opts)
			if err != nil {
				return err
			}
			project, projectName, err := config.ResolveProject(opts.Project)
			if err != nil {
				return err
			}

			var previous *cfg.RunState
			if project.CurrentRun != nil {
				copy := *project.CurrentRun
				previous = &copy
			}
			project.CurrentRun = nil
			if err := cfg.Save(path, config); err != nil {
				return err
			}

			payload := map[string]any{
				"project":      projectName,
				"cleared_run":  previous,
				"current_run":  nil,
				"had_run_scope": previous != nil,
			}
			if isJSONOutput(opts.Output) {
				return output.PrintJSON(payload)
			}
			if previous == nil {
				fmt.Printf("No current run was set for %s.\n", projectName)
				return nil
			}
			fmt.Printf("Cleared run %s for %s.\n", previous.ID, projectName)
			return nil
		},
	})

	return cmd
}
