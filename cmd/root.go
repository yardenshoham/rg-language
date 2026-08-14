// Package cmd is the command line interface.
package cmd

import (
	"os"

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

// Execute runs the CLI.
func Execute() {
	// cobra already prints the error itself — SilenceErrors is not set.
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
