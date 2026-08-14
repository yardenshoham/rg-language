// Package onnx initializes the process-wide ONNX Runtime environment. It is a C
// library loaded from disk at run time and may only be initialized once per
// process, so both model packages call Init and the first one wins.
package onnx

import (
	"fmt"
	"os"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// DefaultLibraryPath is where the container image puts the shared library.
const DefaultLibraryPath = "/usr/local/lib/libonnxruntime.so"

// LibraryPathEnv overrides it, for a locally unpacked copy.
const LibraryPathEnv = "ONNXRUNTIME_LIB"

var initOnce = sync.OnceValue(func() error {
	path := os.Getenv(LibraryPathEnv)
	if path == "" {
		path = DefaultLibraryPath
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("ONNX Runtime shared library not found, set %s: %w", LibraryPathEnv, err)
	}
	ort.SetSharedLibraryPath(path)
	return ort.InitializeEnvironment()
})

// Init loads the shared library. Safe to call from every constructor; only the
// first call does any work.
func Init() error { return initOnce() }

// Destroy frees a tensor's C memory, saying once here rather than at every
// deferred call site that a failed free is not actionable.
func Destroy(value ort.Value) { _ = value.Destroy() }
