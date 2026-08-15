// Package onnx initializes the process-wide ONNX Runtime environment. It is a C
// library loaded from disk at run time and may only be initialized once per
// process, so both model packages call Init and the first one wins.
package onnx

import (
	"cmp"
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

func Init() error { return initOnce() }

// SessionOptions returns the options both models load under. The caller destroys
// them; ONNX Runtime copies what it needs when the session is created.
//
// The only setting here is the thread count, and it exists because ONNX Runtime
// sizes its intra-op pool from the machine's core count, which in a container is
// the host's, not the quota's. On an 8-CPU Railway instance of a much larger host
// that means dozens of threads contending for 8 CPUs, and they spin before they
// block: measured under a 2-CPU quota, synthesis took 5-7x longer than the same
// work with the pool sized to the quota. GOMAXPROCS is the quota — Go reads the
// cgroup, so this stays correct wherever it runs, and on a bare machine it is just
// the core count, which is what the default would have picked anyway.
func SessionOptions() (*ort.SessionOptions, error) {
	options, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("creating session options: %w", err)
	}
	if err := options.SetIntraOpNumThreads(runtime.GOMAXPROCS(0)); err != nil {
		_ = options.Destroy()
		return nil, fmt.Errorf("setting intra-op threads: %w", err)
	}
	return options, nil
}

// Destroy frees a tensor's C memory, saying once here rather than at every
// deferred call site that a failed free is not actionable.
func Destroy(value ort.Value) { _ = value.Destroy() }
