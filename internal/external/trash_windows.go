//go:build windows

package external

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type trashGUID struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

// FOFX_RECYCLEONDELETE requires the Recycle Bin on Windows 8 and later.
const recycleOperationFlags = 0x00080000 | 0x00000400 | 0x00000010 | 0x00000004

var (
	fileOperationClass     = trashGUID{0x3ad05575, 0x8857, 0x4850, [8]byte{0x92, 0x77, 0x11, 0xb8, 0x5b, 0xdb, 0x8e, 0x09}}
	fileOperationInterface = trashGUID{0x947aab5f, 0x0a5c, 0x4c13, [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8}}
	shellItemInterface     = trashGUID{0x43826d1e, 0xe718, 0x42ee, [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
	ole32                  = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx         = ole32.NewProc("CoInitializeEx")
	coUninitialize         = ole32.NewProc("CoUninitialize")
	coCreateInstance       = ole32.NewProc("CoCreateInstance")
	shCreateItem           = syscall.NewLazyDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
)

func trashNative(path string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	status, _, _ := coInitializeEx.Call(0, 2)
	if err := windowsTrashStatus("initialize COM", status); err != nil {
		return err
	}
	defer coUninitialize.Call()
	var operation unsafe.Pointer
	status, _, _ = coCreateInstance.Call(
		uintptr(unsafe.Pointer(&fileOperationClass)), 0, 1,
		uintptr(unsafe.Pointer(&fileOperationInterface)), uintptr(unsafe.Pointer(&operation)),
	)
	if err := windowsTrashStatus("create file operation", status); err != nil {
		return err
	}
	if operation == nil {
		return fmt.Errorf("Windows Recycle Bin did not create a file operation")
	}
	defer comTrashCall(operation, 2)
	source, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var item unsafe.Pointer
	status, _, _ = shCreateItem.Call(
		uintptr(unsafe.Pointer(source)), 0,
		uintptr(unsafe.Pointer(&shellItemInterface)), uintptr(unsafe.Pointer(&item)),
	)
	if err := windowsTrashStatus("open shell item", status); err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("Windows Recycle Bin did not open the shell item")
	}
	defer comTrashCall(item, 2)
	runtime.KeepAlive(source)
	for _, call := range []struct {
		name   string
		method uintptr
		args   []uintptr
	}{
		{"set recycle flags", 5, []uintptr{recycleOperationFlags}},
		{"queue recycle", 18, []uintptr{uintptr(item), 0}},
		{"perform recycle", 21, nil},
	} {
		if err := windowsTrashStatus(call.name, comTrashCall(operation, call.method, call.args...)); err != nil {
			return err
		}
	}
	var aborted int32
	if err := windowsTrashStatus("check recycle result", comTrashCall(operation, 22, uintptr(unsafe.Pointer(&aborted)))); err != nil {
		return err
	}
	if aborted != 0 {
		return fmt.Errorf("Windows Recycle Bin operation canceled")
	}
	runtime.KeepAlive(item)
	return nil
}

func comTrashCall(instance unsafe.Pointer, method uintptr, args ...uintptr) uintptr {
	vtable := *(*unsafe.Pointer)(instance)
	function := *(*uintptr)(unsafe.Add(vtable, method*unsafe.Sizeof(uintptr(0))))
	result, _, _ := syscall.SyscallN(function, append([]uintptr{uintptr(instance)}, args...)...)
	return result
}

func windowsTrashStatus(action string, status uintptr) error {
	if uint32(status)&0x80000000 != 0 {
		return fmt.Errorf("Windows Recycle Bin %s failed: HRESULT 0x%08x", action, uint32(status))
	}
	return nil
}
