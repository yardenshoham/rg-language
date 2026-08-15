// Package cmd is the command line interface.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "rg-language",
		Short:        "שפת הריש גימל — Hebrew text and speech in RG",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.AddCommand(newVersionCmd(), newWebCmd(), newSayCmd(), newPhonikudCmd())
	return rootCmd
}

func Execute() {
	if newRootCmd().Execute() != nil {
		os.Exit(1) // cobra already printed it — SilenceErrors is not set
	}
}

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
