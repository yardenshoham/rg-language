package cmd

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print the version of rg-language",
		Example: "rg-language version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, ok := debug.ReadBuildInfo()
			if !ok {
				return fmt.Errorf("failed to read build info")
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
				Version   string
				GoVersion string
			}{Version: info.Main.Version, GoVersion: info.GoVersion})
		},
	}
}
