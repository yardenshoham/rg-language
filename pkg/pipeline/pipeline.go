// Package pipeline turns Hebrew text into RG text and RG speech.
//
//	Hebrew text
//	  -> diacritizer.AddDiacritics   add niqqud, stress and vocal-shva marks
//	  -> NormalizeNiqqud             repair the doubled-vowel artifact
//	  -> ApplyLexicon                pin hand-corrected words
//	  -> phonikud.Phonemize          -> IPA with stress
//	  -> rg.Transform                -> RG IPA
//	  -> voice.Synth                 -> WAV
//
// Everything below the diacritizer is deterministic, so only that first step can
// surprise you.
package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/yardenshoham/rg-language/pkg/diacritizer"
	"github.com/yardenshoham/rg-language/pkg/heb"
	"github.com/yardenshoham/rg-language/pkg/phonikud"
	"github.com/yardenshoham/rg-language/pkg/rg"
	"github.com/yardenshoham/rg-language/pkg/voice"
)

// The model filenames inside the models directory. They are pinned by name so a
// checksum mismatch at build time is the only way a different checkpoint can get
// in: the voice was chosen by human listening, and swapping it silently would
// invalidate every verdict behind this project.
const (
	DiacritizerModel = "phonikud-1.0.onnx"
	VoiceModel       = "shaul_whisper_heb_ipa1.onnx"
)

// Cache sizes, in megabytes. All three are bounded by bytes rather than by entry
// count, because entry size follows the length of whatever someone typed: a
// 500-rune request retains tens of kilobytes, so a count-based bound would let
// unauthenticated traffic pin an unbounded amount of memory.
//
// The id cache is what makes a /audio/{hash} URL resolvable, and its entries are
// far smaller than the audio itself, so an equal byte budget still holds many
// times more of them — running out of ids would mean serving 404s for links this
// very process handed out.
const (
	transformCacheMB = 64
	audioIDCacheMB   = 64
	defaultAudioMB   = 256
)

// modelTimeout bounds how long one piece of work may wait for the model slot and
// then hold it. A sentence takes about a quarter of a second, so anything near
// this is a queue that is never going to drain.
const modelTimeout = 60 * time.Second

// ErrUnknownAudio means the hash was never handed out, or has been evicted. It is
// the one audio error that is the caller's fault rather than the server's.
var ErrUnknownAudio = errors.New("no audio for this hash")

// resultCost is roughly what one cached Result retains, which is dominated by
// the strings and the per-run segments rather than by the struct itself.
func resultCost(r Result) int64 {
	cost := int64(len(r.Input) + len(r.Vocalized) + len(r.IPA) + len(r.RGIPA) + len(r.AudioHash))
	for _, s := range r.Hebrew {
		cost += int64(len(s.Text)) + 32
	}
	for _, word := range r.Syllables {
		for _, s := range word {
			cost += int64(len(s.Text)) + 32
		}
	}
	return cost + 128
}

// Result is everything the site shows for one piece of Hebrew.
type Result struct {
	Input     string
	Vocalized string
	IPA       string
	RGIPA     string
	// Hebrew is the vocalized RG rendering, split so the UI can highlight the
	// inserted רג.
	Hebrew    []rg.Segment
	Syllables [][]heb.Syllable
	// AudioHash addresses the audio for this result, which is synthesized lazily
	// on first request.
	AudioHash string
}

// Unvocalized is the Hebrew rendering with the niqqud stripped, which is how
// people actually write RG.
func (r Result) Unvocalized() []rg.Segment {
	plain := make([]rg.Segment, 0, len(r.Hebrew))
	for _, s := range r.Hebrew {
		plain = append(plain, rg.Segment{Text: heb.StripMarks(s.Text), Inserted: s.Inserted})
	}
	return plain
}

// Pipeline holds the two models and the caches. It is safe for concurrent use.
type Pipeline struct {
	diacritizer *diacritizer.Diacritizer
	voice       *voice.Voice

	transforms *lru[Result]
	audioIDs   *lru[string]
	audio      *lru[[]byte]

	// transforming and synthesizing collapse concurrent requests for the same
	// input into one run of the model. modelSlot then serializes what is left:
	// with both models resident there is no memory to spare for parallel
	// sessions, ONNX Runtime already spreads one request across the cores, and a
	// sentence takes about a quarter of a second anyway.
	transforming singleflight.Group
	synthesizing singleflight.Group
	modelSlot    chan struct{}
}

func megabytes(mb int) int64 { return int64(mb) * 1024 * 1024 }

// modelWork runs fn under the single model slot, on a context detached from any
// one caller's. Shared work must not die because the request that happened to
// start it went away — with singleflight that would hand the cancellation to
// every other request waiting on the same phrase.
func (p *Pipeline) modelWork(fn func() (any, error)) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), modelTimeout)
	defer cancel()

	select {
	case p.modelSlot <- struct{}{}:
		defer func() { <-p.modelSlot }()
	case <-ctx.Done():
		return nil, fmt.Errorf("waiting for the model: %w", ctx.Err())
	}
	return fn()
}

// share runs fn once for a given key, and waits for it only as long as the
// caller is still interested.
func (p *Pipeline) share(ctx context.Context, group *singleflight.Group, key string, fn func() (any, error)) (any, error) {
	result := group.DoChan(key, fn)
	select {
	case r := <-result:
		return r.Val, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Options configures a Pipeline.
type Options struct {
	// ModelsDir holds the two .onnx files.
	ModelsDir string
	// AudioCacheMB bounds the synthesized audio held in memory. Zero picks a
	// sensible default.
	AudioCacheMB int
}

// New loads both models. It takes a few seconds — most of the process's startup.
func New(ctx context.Context, opts Options) (*Pipeline, error) {
	audioMB := opts.AudioCacheMB
	if audioMB <= 0 {
		audioMB = defaultAudioMB
	}

	d, err := diacritizer.New(ctx, filepath.Join(opts.ModelsDir, DiacritizerModel))
	if err != nil {
		return nil, err
	}
	v, err := voice.New(ctx, filepath.Join(opts.ModelsDir, VoiceModel))
	if err != nil {
		_ = d.Close()
		return nil, err
	}

	return &Pipeline{
		diacritizer: d,
		voice:       v,
		transforms:  newLRU(megabytes(transformCacheMB), resultCost),
		audioIDs: newLRU(megabytes(audioIDCacheMB),
			func(rgIPA string) int64 { return int64(len(rgIPA)) + 64 }),
		audio:     newLRU(megabytes(audioMB), func(b []byte) int64 { return int64(len(b)) }),
		modelSlot: make(chan struct{}, 1),
	}, nil
}

// Close releases both models. It waits for any inference already running, since
// destroying a session out from under a call into it is a use-after-free.
func (p *Pipeline) Close() error {
	select {
	case p.modelSlot <- struct{}{}:
		defer func() { <-p.modelSlot }()
	case <-time.After(modelTimeout):
		// Every unit of work is itself bounded by modelTimeout, so this only
		// happens if one is wedged. Shutting down is still better than hanging.
	}

	closeErr := p.diacritizer.Close()
	if err := p.voice.Close(); err != nil {
		return err
	}
	return closeErr
}

// Transform renders Hebrew text as RG. The result is cached, which saves the
// diacritizer call — the expensive half of the text path.
func (p *Pipeline) Transform(ctx context.Context, text string) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, nil
	}
	if cached, ok := p.transforms.get(text); ok {
		return cached, nil
	}

	result, err := p.share(ctx, &p.transforming, text, func() (any, error) {
		if cached, ok := p.transforms.get(text); ok {
			return cached, nil
		}
		raw, err := p.modelWork(func() (any, error) {
			return p.diacritizer.AddDiacritics(text)
		})
		if err != nil {
			return Result{}, err
		}

		vocalized := ApplyLexicon(NormalizeNiqqud(raw.(string)))
		ipa := phonikud.Phonemize(vocalized)
		rgIPA := rg.Transform(ipa, rg.StressFirst)

		out := Result{
			Input:     text,
			Vocalized: vocalized,
			IPA:       ipa,
			RGIPA:     rgIPA,
			Hebrew:    heb.RGSegments(vocalized),
			Syllables: heb.Syllables(rgIPA),
			AudioHash: audioHash(rgIPA),
		}
		p.transforms.put(text, out)
		p.audioIDs.put(out.AudioHash, rgIPA)
		return out, nil
	})
	if err != nil {
		return Result{}, err
	}
	return result.(Result), nil
}

// audioHash addresses audio by its content: the phonemes plus the synthesis
// parameters that produced them. That is what lets the route be served
// immutable and cached forever by the browser.
func audioHash(rgIPA string) string {
	sum := sha256.Sum256([]byte(voice.Fingerprint + "\x00" + rgIPA))
	return hex.EncodeToString(sum[:16])
}

// Audio returns the WAV for a hash handed out by Transform, synthesizing it on
// first request. Concurrent requests for the same hash synthesize once.
//
// Caching is a correctness feature here, not only a speed one: Piper's graph
// contains a random node, so re-synthesizing the same phonemes gives audibly
// different audio. The cache is what makes a phrase sound the same twice.
func (p *Pipeline) Audio(ctx context.Context, hash string) ([]byte, error) {
	if cached, ok := p.audio.get(hash); ok {
		return cached, nil
	}
	rgIPA, ok := p.audioIDs.get(hash)
	if !ok {
		return nil, fmt.Errorf("%q: %w", hash, ErrUnknownAudio)
	}

	wav, err := p.share(ctx, &p.synthesizing, hash, func() (any, error) {
		if cached, ok := p.audio.get(hash); ok {
			return cached, nil
		}
		return p.modelWork(func() (any, error) {
			wav, err := p.voice.Synth(rgIPA)
			if err != nil {
				return nil, err
			}
			p.audio.put(hash, wav)
			return wav, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return wav.([]byte), nil
}
