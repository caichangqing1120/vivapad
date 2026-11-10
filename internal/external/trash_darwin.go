//go:build darwin

package external

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
int vivapad_trash_file(const char *path, char **location, char **message);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func trashNative(path string) error {
	_, err := trashDarwin(path)
	return err
}

func trashDarwin(path string) (string, error) {
	source := C.CString(path)
	defer C.free(unsafe.Pointer(source))
	var location, message *C.char
	if C.vivapad_trash_file(source, &location, &message) == 0 {
		defer C.free(unsafe.Pointer(location))
		return C.GoString(location), nil
	}
	if message == nil {
		return "", fmt.Errorf("macOS Trash rejected %s", path)
	}
	defer C.free(unsafe.Pointer(message))
	return "", fmt.Errorf("macOS Trash: %s", C.GoString(message))
}
