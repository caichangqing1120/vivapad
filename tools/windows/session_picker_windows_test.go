package windows_test

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func openSessionPickerFile(t *testing.T, dialog uintptr, path string) {
	t.Helper()
	enumChildren := user32.NewProc("EnumChildWindows")
	getControlID := user32.NewProc("GetDlgCtrlID")
	getParent := user32.NewProc("GetParent")
	isEnabled := user32.NewProc("IsWindowEnabled")
	var edits, buttons []uintptr
	var inventory []string
	find := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		visible, _, _ := isWindowVisible.Call(hwnd)
		enabled, _, _ := isEnabled.Call(hwnd)
		if visible == 0 || enabled == 0 {
			return 1
		}
		var name [128]uint16
		getClassName.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		class := syscall.UTF16ToString(name[:])
		id, _, _ := getControlID.Call(hwnd)
		inventory = append(inventory, fmt.Sprintf("%s:%d", class, int32(id)))
		if strings.EqualFold(class, "Button") && id == 1 { // IDOK
			buttons = append(buttons, hwnd)
		}
		if strings.EqualFold(class, "Edit") {
			for control := hwnd; control != 0 && control != dialog; {
				id, _, _ = getControlID.Call(control)
				if id == 1148 { // cmb13, the filename control in the shell dialog.
					edits = append(edits, hwnd)
					break
				}
				control, _, _ = getParent.Call(control)
			}
		}
		return 1
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(40 * time.Millisecond) {
		edits, buttons, inventory = edits[:0], buttons[:0], inventory[:0]
		enumChildren.Call(dialog, find, 0)
		if len(edits) == 1 && len(buttons) == 1 {
			break
		}
	}
	if len(edits) != 1 || len(buttons) != 1 {
		t.Fatalf("Load Session: found %d filename edits and %d Open buttons; visible enabled controls: %v", len(edits), len(buttons), inventory)
	}

	value, err := syscall.UTF16FromString(path)
	if err != nil {
		t.Fatal(err)
	}
	var result uintptr
	ok, _, callErr := sendMessageTimeout.Call(edits[0], 0x000c, 0, uintptr(unsafe.Pointer(&value[0])), 2, 1000, uintptr(unsafe.Pointer(&result))) // WM_SETTEXT
	runtime.KeepAlive(value)
	if ok == 0 || result == 0 {
		t.Fatalf("set the Load Session filename: %v", callErr)
	}
	readback := make([]uint16, len(value)+1)
	ok, _, callErr = sendMessageTimeout.Call(edits[0], 0x000d, uintptr(len(readback)), uintptr(unsafe.Pointer(&readback[0])), 2, 1000, uintptr(unsafe.Pointer(&result))) // WM_GETTEXT
	if got := syscall.UTF16ToString(readback); ok == 0 || got != path {
		t.Fatalf("Load Session filename = %q, want %q (%v)", got, path, callErr)
	}
	t.Log("Verified the full filename in the Windows file picker")
	if ok, _, callErr := postMessage.Call(buttons[0], 0x00f5, 0, 0); ok == 0 { // BM_CLICK
		t.Fatalf("click Open in the Load Session picker: %v", callErr)
	}
}
