//go:build !windows

package onnxruntime_go

/*
#include <stdlib.h>
*/
import "C"

// Converts the given path to an ORTCHAR_T string, pointed to by a *C.char. The
// returned string must be freed using C.free when no longer needed. This
// wrapper is used for source compatibility with onnxruntime API functions
// requiring paths, which must be UTF-16 on Windows but UTF-8 elsewhere.
//
// It is defined here (rather than in setup_env.go) so that it is available for
// both the default build and the "static" build tag.
func createOrtCharString(str string) (*C.char, error) {
	return C.CString(str), nil
}
