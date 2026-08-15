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

const (
	defaultLibraryPath = "/usr/local/lib/libonnxruntime.so" // where the image puts it
	libraryPathEnv     = "ONNXRUNTIME_LIB"                  // override, for a local copy
)

var initOnce = sync.OnceValue(func() error {
	path := cmp.Or(os.Getenv(libraryPathEnv), defaultLibraryPath)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("ONNX Runtime shared library not found, set %s: %w", libraryPathEnv, err)
	}
	ort.SetSharedLibraryPath(path)
	return ort.InitializeEnvironment()
})

// Session loads a model, initializing the runtime on the first call. The options are
// only read while the session is created, so destroying them here is safe.
//
// The only setting is the thread count, and it exists because ONNX Runtime sizes its
// intra-op pool from the machine's core count, which in a container is the host's,
// not the quota's. On an 8-CPU Railway instance of a much larger host that means
// dozens of threads contending for 8 CPUs, and they spin before they block: measured
// under a 2-CPU quota, synthesis took 5-7x longer than the same work with the pool
// sized to the quota. GOMAXPROCS is the quota — Go reads the cgroup, so this stays
// correct wherever it runs, and on a bare machine it is just the core count, which is
// what the default would have picked anyway.
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

// Destroy frees tensors' C memory; said once here, a failed free is not actionable.
func Destroy(values ...ort.Value) {
	for _, value := range values {
		_ = value.Destroy()
	}
}
