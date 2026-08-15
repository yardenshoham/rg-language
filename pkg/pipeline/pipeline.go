// Package pipeline turns Hebrew text into RG text and RG speech: AddDiacritics ->
// NormalizeNiqqud -> ApplyLexicon -> Phonemize -> rg.Transform -> voice.Synth.
// Only the first step is a model, so only it can surprise you.
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

// Pinned by name so a build-time checksum mismatch is the only way a different
// checkpoint gets in: the voice was chosen by human listening, and swapping it
// silently would invalidate every verdict behind this project.
const (
	DiacritizerModel = "phonikud-1.0.onnx"
	VoiceModel       = "matcha-he-en.onnx"
)

// Cache sizes, in megabytes. All three are bounded by bytes, not entries: entry
// size follows what someone typed — a 500-rune request retains tens of kilobytes —
// so a count-based bound would let unauthenticated traffic pin unbounded memory.
// Ids are far smaller than audio, so an equal budget holds many more; running out
// of them means 404s for links this process handed out.
const (
	transformCacheMB = 64
	audioIDCacheMB   = 64
	defaultAudioMB   = 256
)

// modelTimeout bounds waiting for and then holding the model slot. A sentence
// takes about a quarter of a second, so anything near this will never drain.
const modelTimeout = 60 * time.Second

// ErrUnknownAudio means the hash was never handed out, or has been evicted.
var ErrUnknownAudio = errors.New("no audio for this hash")

// resultCost is roughly what one cached Result retains.
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
	// Hebrew is the vocalized rendering, split so the UI can highlight the רג.
	Hebrew    []rg.Segment
	Syllables [][]heb.Syllable
	// AudioHash addresses the audio, which is synthesized lazily on first request.
	AudioHash string
}

// Unvocalized is the Hebrew rendering with the niqqud stripped.
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

	// singleflight collapses concurrent requests for the same input; modelSlot then
	// serializes what is left, since two resident models leave no memory to spare
	// for parallel sessions and ONNX Runtime already uses all the cores.
	transforming singleflight.Group
	synthesizing singleflight.Group
	modelSlot    chan struct{}
}

func megabytes(mb int) int64 { return int64(mb) * 1024 * 1024 }

// modelWork runs fn under the single model slot, on its own deadline: shared work
// must not die because the request that started it went away — with singleflight
// that would hand the cancellation to every request waiting on the same phrase.
func modelWork[V any](p *Pipeline, fn func() (V, error)) (V, error) {
	select {
	case p.modelSlot <- struct{}{}:
		defer func() { <-p.modelSlot }()
	case <-time.After(modelTimeout):
		var zero V
		return zero, fmt.Errorf("waiting for the model: %w", context.DeadlineExceeded)
	}
	return fn()
}

// cachedOnce answers from cache, or runs fn once however many callers ask at once,
// caching the result and waiting only as long as this caller is interested.
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
		var zero V
		return zero, ctx.Err()
	}
}

// Options configures a Pipeline.
type Options struct {
	// ModelsDir holds the two .onnx files.
	ModelsDir string
	// AudioCacheMB bounds the audio held in memory. Zero picks a default.
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
		// Only reachable if a unit of work is wedged; better than hanging.
	}

	closeErr := p.diacritizer.Close()
	if err := p.voice.Close(); err != nil {
		return err
	}
	return closeErr
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

		out := Result{
			Input:     text,
			Vocalized: vocalized,
			IPA:       ipa,
			RGIPA:     rgIPA,
			Hebrew:    heb.RGSegments(vocalized),
			Syllables: heb.Syllables(rgIPA),
			AudioHash: audioHash(rgIPA),
		}
		p.audioIDs.put(out.AudioHash, rgIPA)
		return out, nil
	})
}

// audioHash addresses audio by content — phonemes plus synthesis parameters —
// which is what lets the route be served immutable.
func audioHash(rgIPA string) string {
	sum := sha256.Sum256([]byte(voice.Fingerprint + "\x00" + rgIPA))
	return hex.EncodeToString(sum[:16])
}

// Audio returns the WAV for a hash handed out by Transform, synthesizing it on
// first request. The cache is a correctness feature, not just a speed one: the voice
// samples a noise prior, so re-synthesis of the same phonemes runs the same length but
// sounds audibly different.
func (p *Pipeline) Audio(ctx context.Context, hash string) ([]byte, error) {
	// Ahead of the id lookup: a hot WAV's id is never refreshed, so it can age out
	// of the id cache while the audio is still here.
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
