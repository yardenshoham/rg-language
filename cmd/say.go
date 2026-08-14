package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yardenshoham/rg-language/pkg/heb"
	"github.com/yardenshoham/rg-language/pkg/pipeline"
)

// newSayCmd transforms one phrase and optionally writes the audio, so the whole
// pipeline can be checked from a terminal — including by ear, which is the only
// instrument that has ever worked on this project.
func newSayCmd() *cobra.Command {
	var out string

	cmd := &cobra.Command{
		Use:     "say <hebrew>",
		Short:   "Transform Hebrew to RG and print all three renderings",
		Example: `rg-language say "מה נשמע" --out hello.wav`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := pipeline.New(cmd.Context(), pipeline.Options{ModelsDir: modelsDirDefault()})
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
			fmt.Fprintf(w, "niqqud     %s\n", result.Vocalized)
			fmt.Fprintf(w, "ipa        %s\n", result.IPA)
			fmt.Fprintf(w, "rg ipa     %s\n", result.RGIPA)
			fmt.Fprintf(w, "rg         %s\n", heb.StripMarks(heb.RG(result.Vocalized)))
			fmt.Fprintf(w, "rg niqqud  %s\n", heb.RG(result.Vocalized))
			fmt.Fprintf(w, "rg latin   %s\n", heb.Latin(result.RGIPA))
			if pipeline.DoubledVowel(result.IPA) {
				fmt.Fprintf(w, "warning    two identical adjacent vowels, which real Hebrew does not produce\n")
			}
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
