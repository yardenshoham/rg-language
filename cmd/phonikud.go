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

type row struct {
	Vocalized string `json:"vocalized"`
	IPA       string `json:"ipa"`
	RG        string `json:"rg"`
	HebRG     string `json:"heb_rg"`
	Latin     string `json:"latin"`
	Doubled   bool   `json:"doubled_vowel,omitempty"`
}

func newRow(vocalized, ipa, rgIPA string) row {
	return row{Vocalized: vocalized, IPA: ipa, RG: rgIPA, HebRG: heb.RG(vocalized), Latin: heb.Latin(rgIPA), Doubled: pipeline.DoubledVowel(ipa)}
}

func (r row) write(w io.Writer) {
	fmt.Fprintf(w, "niqqud     %s\nipa        %s\nrg ipa     %s\nrg         %s\nrg niqqud  %s\nrg latin   %s\n",
		r.Vocalized, r.IPA, r.RG, heb.StripMarks(r.HebRG), r.HebRG, r.Latin)
	if r.Doubled {
		fmt.Fprintf(w, "warning    two identical adjacent vowels, which real Hebrew does not produce\n")
	}
}

// newPhonikudCmd needs no model and no ONNX Runtime: the quick way to check a rule.
func newPhonikudCmd() *cobra.Command {
	var (
		asJSON, noNormalize bool
		stressMode          string
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
			if mode != rg.StressFirst && mode != rg.StressSecond && mode != rg.StressBoth {
				return fmt.Errorf("unknown stress mode %q, want first, second or both", stressMode)
			}

			run := func(vocalized string) error {
				if !noNormalize {
					vocalized = pipeline.ApplyLexicon(pipeline.NormalizeNiqqud(vocalized))
				}
				ipa := phonikud.Phonemize(vocalized)
				r := newRow(vocalized, ipa, rg.Transform(ipa, mode))
				if asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(r)
				}
				r.write(cmd.OutOrStdout())
				return nil
			}

			if len(args) > 0 {
				return run(strings.Join(args, " "))
			}

			scanner := bufio.NewScanner(cmd.InOrStdin())
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				if line := strings.TrimSpace(scanner.Text()); line != "" {
					if err := run(line); err != nil {
						return err
					}
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
