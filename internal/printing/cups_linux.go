//go:build linux

package printing

import (
	"fmt"
	"os/exec"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

func chooseCUPSDefault() (DeviceMetrics, error) {
	if _, err := exec.LookPath("lp"); err != nil {
		return DeviceMetrics{}, fmt.Errorf("%w: CUPS lp command is unavailable", ErrUnsupported)
	}
	handle, err := purego.Dlopen("libcups.so.2", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return DeviceMetrics{}, fmt.Errorf("%w: CUPS library is unavailable: %v", ErrUnsupported, err)
	}
	defer purego.Dlclose(handle)
	var namedDest func(uintptr, uintptr, uintptr) uintptr
	var copyInfo func(uintptr, uintptr) uintptr
	var defaultMedia func(uintptr, uintptr, uintptr, uint32, uintptr) int32
	var freeInfo func(uintptr)
	var freeDests func(int32, uintptr)
	for _, entry := range []struct {
		name   string
		target any
	}{
		{"cupsGetNamedDest", &namedDest},
		{"cupsCopyDestInfo", &copyInfo},
		{"cupsGetDestMediaDefault", &defaultMedia},
		{"cupsFreeDestInfo", &freeInfo},
		{"cupsFreeDests", &freeDests},
	} {
		symbol, err := purego.Dlsym(handle, entry.name)
		if err != nil {
			return DeviceMetrics{}, fmt.Errorf("%w: CUPS %s: %v", ErrUnsupported, entry.name, err)
		}
		purego.RegisterFunc(entry.target, symbol)
	}
	destination := namedDest(0, 0, 0)
	if destination == 0 {
		return DeviceMetrics{}, ErrNoPrinter
	}
	defer freeDests(1, destination)
	info := copyInfo(0, destination)
	if info == 0 {
		return DeviceMetrics{}, fmt.Errorf("printing: CUPS default printer has no media information")
	}
	defer freeInfo(info)
	var media cupsSize
	found := defaultMedia(0, destination, info, 0, uintptr(unsafe.Pointer(&media)))
	runtime.KeepAlive(&media)
	if found == 0 {
		return DeviceMetrics{}, fmt.Errorf("printing: CUPS default printer did not report its paper and margins")
	}
	return cupsDeviceMetrics(media)
}
