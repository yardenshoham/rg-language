package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yardenshoham/rg-language/pkg/heb"
	"github.com/yardenshoham/rg-language/pkg/phonikud"
	"github.com/yardenshoham/rg-language/pkg/pipeline"
	"github.com/yardenshoham/rg-language/pkg/rg"
)

// row is one line of phonikud output. The field names match the differential
// corpus, so a JSON run can be diffed against testdata/corpus.jsonl directly.
type row struct {
	Vocalized string `json:"vocalized"`
	IPA       string `json:"ipa"`
	RG        string `json:"rg"`
	HebRG     string `json:"heb_rg"`
	Latin     string `json:"latin"`
	Doubled   bool   `json:"doubled_vowel,omitempty"`
}

// newPhonikudCmd exposes everything below the diacritizer. That half of the
// pipeline is pure string handling, so this needs no models and no ONNX Runtime
// and answers instantly — which makes it the quick way to check a rule, and the
// way to diff this port against the Python original a line at a time.
func newPhonikudCmd() *cobra.Command {
	var (
		asJSON      bool
		stressMode  string
		noNormalize bool
	)

	cmd := &cobra.Command{
		Use:   "phonikud [vocalized hebrew...]",
		Short: "Phonemize vocalized Hebrew and apply the RG rule, with no models",
		Long: "Takes Hebrew that already has niqqud — which is what the diacritizer would\n" +
			"have produced — and runs the rest of the pipeline on it. Reads one phrase\n" +
			"per line from stdin when given no arguments.",
		Example: `  rg-language phonikud "שָׁלוֹם"
  rg-language phonikud --json "מָה נִשְׁמַע"
  jq -r .vocalized pkg/phonikud/testdata/corpus.jsonl | rg-language phonikud --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := rg.StressMode(stressMode)
			switch mode {
			case rg.StressFirst, rg.StressSecond, rg.StressBoth:
			default:
				return fmt.Errorf("unknown stress mode %q, want first, second or both", stressMode)
			}

			run := func(vocalized string) error {
				if !noNormalize {
					vocalized = pipeline.ApplyLexicon(pipeline.NormalizeNiqqud(vocalized))
				}
				ipa := phonikud.Phonemize(vocalized)
				rgIPA := rg.Transform(ipa, mode)
				out := row{
					Vocalized: vocalized,
					IPA:       ipa,
					RG:        rgIPA,
					HebRG:     heb.RG(vocalized),
					Latin:     heb.Latin(rgIPA),
					Doubled:   pipeline.DoubledVowel(ipa),
				}
				return write(cmd.OutOrStdout(), out, asJSON)
			}

			if len(args) > 0 {
				return run(strings.Join(args, " "))
			}

			scanner := bufio.NewScanner(cmd.InOrStdin())
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if err := run(line); err != nil {
					return err
				}
			}
			return scanner.Err()
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "One JSON object per line, with the same field names as the corpus")
	cmd.Flags().StringVar(&stressMode, "stress", string(rg.StressFirst), "Which copy of the stressed vowel keeps the stress: first, second or both")
	cmd.Flags().BoolVar(&noNormalize, "raw", false, "Skip the niqqud repair and the override lexicon, phonemizing the input as given")
	return cmd
}

func write(w io.Writer, out row, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(out)
	}
	fmt.Fprintf(w, "niqqud     %s\n", out.Vocalized)
	fmt.Fprintf(w, "ipa        %s\n", out.IPA)
	fmt.Fprintf(w, "rg ipa     %s\n", out.RG)
	fmt.Fprintf(w, "rg         %s\n", heb.StripMarks(out.HebRG))
	fmt.Fprintf(w, "rg niqqud  %s\n", out.HebRG)
	fmt.Fprintf(w, "rg latin   %s\n", out.Latin)
	if out.Doubled {
		fmt.Fprintf(w, "warning    two identical adjacent vowels, which real Hebrew does not produce\n")
	}
	return nil
}
