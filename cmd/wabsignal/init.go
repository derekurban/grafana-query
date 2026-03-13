package wabsignal

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "init",
		Short:  "Deprecated setup shim",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("`wabsignal init` has been replaced.")
			fmt.Println("Run:")
			fmt.Println("  wabsignal setup --mode restrictive|full-access ...")
			fmt.Println("  wabsignal project create <project-name> <primary-service> [extra-services...]")
		},
	}
}
