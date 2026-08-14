// Package onnx initializes the process-wide ONNX Runtime environment.
//
// The runtime is a C library loaded at run time from a path on disk, and it may
// only be initialized once per process. Both model packages call Init, so
// whichever loads first wins and the other is a no-op.
package onnx

import (
	"fmt"
	"os"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// DefaultLibraryPath is where the container image puts the shared library.
const DefaultLibraryPath = "/usr/local/lib/libonnxruntime.so"

// LibraryPathEnv overrides it, which is how local development finds its own
// unpacked copy.
const LibraryPathEnv = "ONNXRUNTIME_LIB"

var (
	once    sync.Once
	initErr error
)

// Init loads the ONNX Runtime shared library. It is safe to call from every
// constructor that needs it; only the first call does any work.
func Init() error {
	once.Do(func() {
		path := os.Getenv(LibraryPathEnv)
		if path == "" {
			path = DefaultLibraryPath
		}
		if _, err := os.Stat(path); err != nil {
			initErr = fmt.Errorf("ONNX Runtime shared library not found, set %s: %w", LibraryPathEnv, err)
			return
		}
		ort.SetSharedLibraryPath(path)
		initErr = ort.InitializeEnvironment()
	})
	return initErr
}

// Destroy frees a tensor's C memory. It exists so that the callers, which all
// free in a defer, can say once here rather than at every call site that there
// is nothing useful to do when a free fails.
func Destroy(value ort.Value) { _ = value.Destroy() }
