// Package onnx loads models under the process-wide ONNX Runtime environment. It is
// a C library loaded from disk at run time and may only be initialized once per
// process, so the first session opened wins.
package onnx

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// The default is where the image puts the library; the variable overrides it for a local copy.
var initOnce = sync.OnceValue(func() error {
	path := cmp.Or(os.Getenv("ONNXRUNTIME_LIB"), "/usr/local/lib/libonnxruntime.so")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("ONNX Runtime shared library not found, set ONNXRUNTIME_LIB: %w", err)
	}
	ort.SetSharedLibraryPath(path)
	return ort.InitializeEnvironment()
})

// Session loads a model, initializing the runtime on the first call. The options are
// only read while the session is created, so destroying them here is safe.
//
// The one setting is the intra-op thread count: ONNX Runtime sizes that pool from the
// host's core count, not the container's quota, and the surplus threads spin before they
// block — under a 2-CPU quota that measured 5-7x slower. GOMAXPROCS reads the cgroup.
func Session(ctx context.Context, modelPath string, inputs, outputs []string) (*ort.DynamicAdvancedSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := initOnce(); err != nil {
		return nil, err
	}
	options, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("creating session options: %w", err)
	}
	defer func() { _ = options.Destroy() }()
	if err := options.SetIntraOpNumThreads(runtime.GOMAXPROCS(0)); err != nil {
		return nil, fmt.Errorf("setting intra-op threads: %w", err)
	}
	return ort.NewDynamicAdvancedSession(modelPath, inputs, outputs, options)
}

// runOptions asks the CPU arena to give back what a run grew it by. Without this the
// arena keeps its high-water mark forever: one 500-rune synthesis takes the process from
// about 530 MB resident to about 2 GB and it stays there, which on Railway's per-minute
// RAM billing was most of the bill. Measured over the same run, 626 MB and no slower.
var runOptions = sync.OnceValues(func() (*ort.RunOptions, error) {
	options, err := ort.NewRunOptions()
	if err != nil {
		return nil, fmt.Errorf("creating run options: %w", err)
	}
	if err := options.AddRunConfigEntry("memory.enable_memory_arena_shrinkage", "cpu:0"); err != nil {
		_ = options.Destroy()
		return nil, fmt.Errorf("enabling arena shrinkage: %w", err)
	}
	return options, nil
})

// Run is session.Run with the shared run options; outputs are allocated by the runtime.
func Run(session *ort.DynamicAdvancedSession, inputs, outputs []ort.Value) error {
	options, err := runOptions()
	if err != nil {
		return err
	}
	return session.RunWithOptions(inputs, outputs, options)
}

// Destroy frees tensors' C memory; said once here, a failed free is not actionable.
func Destroy(values ...ort.Value) {
	for _, value := range values {
		_ = value.Destroy()
	}
}
