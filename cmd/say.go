package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yardenshoham/rg-language/pkg/pipeline"
)

// newSayCmd checks the pipeline by ear — the only instrument that has ever worked here.
func newSayCmd() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:     "say <hebrew>",
		Short:   "Transform Hebrew to RG and print all three renderings",
		Example: `rg-language say "מה נשמע" --out hello.wav`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := pipeline.New(cmd.Context(), defaultModelsDir(), 0)
			if err != nil {
				return fmt.Errorf("loading models: %w", err)
			}
			defer p.Close() //nolint:errcheck // nothing useful to do about it here

			result, err := p.Transform(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "input      %s\n", result.Input)
			newRow(result.Vocalized, result.IPA, result.RGIPA).write(w)
			if out == "" {
				return nil
			}

			wav, err := p.Audio(cmd.Context(), result.AudioHash)
			if err != nil {
				return err
			}
			if err := os.WriteFile(out, wav, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(w, "wrote      %s (%d bytes)\n", out, len(wav))
			return nil
		},
	}

	cmd.Flags().StringVar(&out, "out", "", "Write the synthesized audio to this WAV file")
	return cmd
}
