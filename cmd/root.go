// Package cmd is the command line interface.
package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

type loggerKey struct{}

func newRootCmd() *cobra.Command {
	var debug bool

	rootCmd := &cobra.Command{
		Use:          "rg-language",
		Short:        "שפת הריש גימל — Hebrew text and speech in RG",
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			level := slog.LevelInfo
			if debug {
				level = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(cmd.OutOrStdout(), &slog.HandlerOptions{
				Level: level,
			}))
			cmd.SetContext(context.WithValue(cmd.Context(), loggerKey{}, logger))
		},
	}

	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newWebCmd())
	rootCmd.AddCommand(newSayCmd())
	rootCmd.AddCommand(newPhonikudCmd())
	return rootCmd
}

// Execute runs the CLI.
func Execute() {
	// cobra already prints the error itself — SilenceErrors is not set.
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
