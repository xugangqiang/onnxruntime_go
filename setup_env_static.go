//go:build !windows && static

package onnxruntime_go

import (
	"fmt"
	"runtime"
	"unsafe"
)

/*
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include "onnxruntime_wrapper.h"

typedef OrtApiBase* (*GetOrtApiBaseFunction)(void);

// Since Go can't call C function pointers directly, we just use this helper
// when calling GetApiBase
OrtApiBase *CallGetAPIBaseFunction(void *fn) {
	OrtApiBase *to_return = ((GetOrtApiBaseFunction) fn)();
	return to_return;
}
*/
import "C"

// This file is compiled instead of setup_env.go when the "static" build tag is
// set. It is used when ONNX Runtime is linked statically into the main
// executable (for example via -Wl,--whole-archive -lonnxruntime together with
// -Wl,--export-dynamic) instead of being loaded from a shared object at
// runtime.
//
// In that configuration the ORT symbols (including OrtGetApiBase) already live
// in the process's own global symbol table, so we resolve them through
// dlopen(NULL) rather than loading an external shared library. There is
// intentionally no .so fallback path: a main executable cannot be dlopen'd by
// its own file path (glibc rejects it with "cannot dynamically load
// executable"), so dlopen(NULL) is the only supported mechanism.

// This will contain the handle returned by dlopen(NULL) if it succeeded.
var libraryHandle unsafe.Pointer

func platformCleanup() error {
	v, e := C.dlclose(libraryHandle)
	if v != 0 {
		return fmt.Errorf("Error closing the library: %w", e)
	}
	return nil
}

// Should only be called on Apple systems; looks up the CoreML provider
// function which should only be exported on apple onnxruntime dylib files.
func setAppendCoreMLFunctionPointer(libraryHandle unsafe.Pointer) error {
	// This function name must match the name in coreml_provider_factory.h,
	// which is provided in the onnxruntime release's include/ directory on for
	// Apple platforms.
	fnName := "OrtSessionOptionsAppendExecutionProvider_CoreML"
	cFunctionName := C.CString(fnName)
	defer C.free(unsafe.Pointer(cFunctionName))
	appendCoreMLProviderProc := C.dlsym(libraryHandle, cFunctionName)
	if appendCoreMLProviderProc == nil {
		msg := C.GoString(C.dlerror())
		return fmt.Errorf("Error looking up %s: %s", fnName, msg)
	}
	C.SetCoreMLProviderFunctionPointer(appendCoreMLProviderProc)
	return nil
}

// platformInitializeEnvironment resolves the ONNX Runtime entry points from the
// running process's own global symbol table. It is used when ORT is linked
// statically (see the "static" build tag). See the file-level comment above for
// why dlopen(NULL) is used instead of loading an external shared object.
func platformInitializeEnvironment() error {
	handle := C.dlopen(nil, C.RTLD_LAZY)
	if handle == nil {
		msg := C.GoString(C.dlerror())
		return fmt.Errorf("Error resolving statically-linked ONNX Runtime via dlopen(NULL): %s", msg)
	}
	cFunctionName := C.CString("OrtGetApiBase")
	defer C.free(unsafe.Pointer(cFunctionName))
	getAPIBaseProc := C.dlsym(handle, cFunctionName)
	if getAPIBaseProc == nil {
		C.dlclose(handle)
		msg := C.GoString(C.dlerror())
		return fmt.Errorf("Error looking up OrtGetApiBase in statically-linked ONNX Runtime: %s", msg)
	}
	ortAPIBase := C.CallGetAPIBaseFunction(getAPIBaseProc)
	tmp := C.SetAPIFromBase((*C.OrtApiBase)(unsafe.Pointer(ortAPIBase)))
	if tmp != 0 {
		C.dlclose(handle)
		return fmt.Errorf("Error setting ORT API base: %d", tmp)
	}
	if (runtime.GOOS == "darwin") || (runtime.GOOS == "ios") {
		setAppendCoreMLFunctionPointer(handle)
		// We'll silently ignore potential errors returned by
		// setAppendCoreMLFunctionPointer (for now at least). Even though we're
		// on Apple hardware, it's possible that the user will have compiled
		// the onnxruntime library from source without CoreML support.
		// A failure here will only leave the coreml function pointer as NULL
		// in our C code, which will be detected and result in an error at
		// runtime.
	}
	libraryHandle = handle
	return nil
}
