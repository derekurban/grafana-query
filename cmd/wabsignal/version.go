package wabsignal

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/derekurban/wabii-signal/internal/output"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version and build metadata",
		Long: strings.TrimSpace(`
Print the wabii-signal version.

For release builds, the version, commit, and build date are injected at build
time. For local builds, wabii-signal falls back to module build info when
available and otherwise reports a development build.
`),
		Example: strings.TrimSpace(`
  wabsignal version
  wabsignal version --short
  wabsignal version --output json
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			info := buildVersionInfo()
			outputMode := ""
			if rootGlobalOptions != nil {
				outputMode = strings.TrimSpace(rootGlobalOptions.Output)
			}
			if outputMode == "json" || outputMode == "raw" {
				return output.PrintJSON(info)
			}
			if short, _ := cmd.Flags().GetBool("short"); short {
				fmt.Println(info["version"])
				return nil
			}
			fmt.Printf("wabsignal %s\n", info["version"])
			fmt.Printf("commit: %s\n", info["commit"])
			fmt.Printf("date:   %s\n", info["date"])
			fmt.Printf("go:     %s\n", info["go_version"])
			fmt.Printf("os/arch:%s/%s\n", info["os"], info["arch"])
			return nil
		},
	}
	cmd.Flags().Bool("short", false, "Print only the version string")
	return cmd
}

func buildVersionInfo() map[string]any {
	info := map[string]any{
		"version":    version,
		"commit":     commit,
		"date":       date,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}

	if bi, ok := debug.ReadBuildInfo(); ok && bi != nil {
		info["module"] = bi.Main.Path
		if info["version"] == "dev" && strings.TrimSpace(bi.Main.Version) != "" && bi.Main.Version != "(devel)" {
			info["version"] = bi.Main.Version
		}
		for _, setting := range bi.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info["commit"] == "unknown" && strings.TrimSpace(setting.Value) != "" {
					info["commit"] = setting.Value
				}
			case "vcs.time":
				if info["date"] == "unknown" && strings.TrimSpace(setting.Value) != "" {
					info["date"] = setting.Value
				}
			case "vcs.modified":
				info["dirty"] = setting.Value
			}
		}
	}
	return info
}

var rootGlobalOptions *GlobalOptions
