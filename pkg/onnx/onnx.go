// Package onnx initializes the process-wide ONNX Runtime environment. It is a C
// library loaded from disk at run time and may only be initialized once per
// process, so both model packages call Init and the first one wins.
package onnx

import (
	"cmp"
	"fmt"
	"os"
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

// Destroy frees a tensor's C memory, saying once here rather than at every
// deferred call site that a failed free is not actionable.
func Destroy(value ort.Value) { _ = value.Destroy() }
