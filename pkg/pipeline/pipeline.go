// Package pipeline turns Hebrew text into RG text and RG speech: AddDiacritics ->
// NormalizeNiqqud -> ApplyLexicon -> Phonemize -> rg.Transform -> voice.Synth.
package pipeline

import (
	"cmp"
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

// Pinned by name: the voice was chosen by human listening, so a build-time checksum
// mismatch should be the only way another checkpoint gets in.
const (
	DiacritizerModel = "phonikud-1.0.onnx"
	VoiceModel       = "matcha-he-en.onnx"
)

// modelTimeout bounds waiting for and then holding the model slot. A sentence
// takes about a quarter of a second, so anything near this will never drain.
const modelTimeout = 60 * time.Second

// ErrUnknownAudio means the hash was never handed out, or has been evicted.
var ErrUnknownAudio = errors.New("no audio for this hash")

// resultCost bounds what one cached Result retains. Every field is derived from the
// input, and over the whole corpus the total stays under 128 bytes per input byte.
func resultCost(r Result) int64 { return int64(len(r.Input))*128 + 256 }

// Result is everything the site shows for one piece of Hebrew.
type Result struct {
	Input     string
	Vocalized string
	IPA       string
	RGIPA     string
	// Hebrew is the vocalized rendering, split so the UI can highlight the רג.
	Hebrew    []rg.Segment
	Syllables [][]heb.Syllable
	AudioHash string
}

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

	// singleflight collapses duplicate requests; modelSlot serializes the rest, since two
	// resident models leave no memory for parallel sessions and ONNX Runtime uses all cores.
	transforming singleflight.Group
	synthesizing singleflight.Group
	modelSlot    chan struct{}
}

// modelWork takes its own deadline: with singleflight, cancelling the request that
// started shared work would cancel it for every waiter.
func modelWork[V any](p *Pipeline, fn func() (V, error)) (V, error) {
	select {
	case p.modelSlot <- struct{}{}:
		defer func() { <-p.modelSlot }()
	case <-time.After(modelTimeout):
		return *new(V), fmt.Errorf("waiting for the model: %w", context.DeadlineExceeded)
	}
	return fn()
}

// cachedOnce answers from cache, or runs fn once however many callers ask at once.
func cachedOnce[V any](ctx context.Context, group *singleflight.Group, cache *lru[V], key string, fn func() (V, error)) (V, error) {
	if cached, ok := cache.get(key); ok {
		return cached, nil
	}
	result := group.DoChan(key, func() (any, error) {
		// The winner of a race fills the cache before the losers are woken.
		if cached, ok := cache.get(key); ok {
			return cached, nil
		}
		value, err := fn()
		if err == nil {
			cache.put(key, value)
		}
		return value, err
	})
	select {
	case r := <-result:
		value, _ := r.Val.(V)
		return value, r.Err
	case <-ctx.Done():
		return *new(V), ctx.Err()
	}
}

// New takes a few seconds — most of the process's startup; audioCacheMB zero picks the
// default. The cache budgets are bytes, not entries: an entry is as big as what someone
// typed, so a count bound would let traffic pin unbounded memory; ids are far smaller.
func New(ctx context.Context, modelsDir string, audioCacheMB int) (*Pipeline, error) {
	d, err := diacritizer.New(ctx, filepath.Join(modelsDir, DiacritizerModel))
	if err != nil {
		return nil, err
	}
	v, err := voice.New(ctx, filepath.Join(modelsDir, VoiceModel))
	if err != nil {
		_ = d.Close()
		return nil, err
	}

	return &Pipeline{
		diacritizer: d,
		voice:       v,
		transforms:  newLRU(64<<20, resultCost),
		audioIDs:    newLRU(64<<20, func(rgIPA string) int64 { return int64(len(rgIPA)) + 64 }),
		audio:       newLRU(cmp.Or(max(int64(audioCacheMB)<<20, 0), int64(256<<20)), func(b []byte) int64 { return int64(len(b)) }),
		modelSlot:   make(chan struct{}, 1),
	}, nil
}

// Close waits for any inference already running: destroying a live session is a
// use-after-free. The slot is never handed back, so nothing can start after this.
func (p *Pipeline) Close() error {
	select {
	case p.modelSlot <- struct{}{}:
	case <-time.After(modelTimeout): // only if a unit of work is wedged; better than hanging
	}
	return errors.Join(p.diacritizer.Close(), p.voice.Close())
}

// Transform renders Hebrew text as RG. Caching it saves the diacritizer call.
func (p *Pipeline) Transform(ctx context.Context, text string) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, nil
	}
	return cachedOnce(ctx, &p.transforming, p.transforms, text, func() (Result, error) {
		raw, err := modelWork(p, func() (string, error) { return p.diacritizer.AddDiacritics(text) })
		if err != nil {
			return Result{}, err
		}

		vocalized := ApplyLexicon(NormalizeNiqqud(raw))
		ipa := phonikud.Phonemize(vocalized)
		rgIPA := rg.Transform(ipa, rg.StressFirst)
		sum := sha256.Sum256([]byte(voice.Fingerprint + "\x00" + rgIPA))

		out := Result{
			Input:     text,
			Vocalized: vocalized,
			IPA:       ipa,
			RGIPA:     rgIPA,
			Hebrew:    heb.RGSegments(vocalized),
			Syllables: heb.Syllables(rgIPA),
			AudioHash: hex.EncodeToString(sum[:16]),
		}
		p.audioIDs.put(out.AudioHash, rgIPA)
		return out, nil
	})
}

// Audio returns the WAV for a hash handed out by Transform, synthesizing it on first
// request. The cache is a correctness feature, not just a speed one: the voice samples
// a noise prior, so re-synthesis of the same phonemes sounds audibly different.
func (p *Pipeline) Audio(ctx context.Context, hash string) ([]byte, error) {
	// Ahead of the id lookup: a hot WAV's id can age out while the audio is still here.
	if cached, ok := p.audio.get(hash); ok {
		return cached, nil
	}
	rgIPA, ok := p.audioIDs.get(hash)
	if !ok {
		return nil, fmt.Errorf("%q: %w", hash, ErrUnknownAudio)
	}

	return cachedOnce(ctx, &p.synthesizing, p.audio, hash, func() ([]byte, error) {
		return modelWork(p, func() ([]byte, error) { return p.voice.Synth(rgIPA) })
	})
}
