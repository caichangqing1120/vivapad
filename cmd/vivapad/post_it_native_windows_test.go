package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var postItUser32 = syscall.NewLazyDLL("user32.dll")

func prepareNativePostItDesktop() (func(), error) {
	getDesktop := postItUser32.NewProc("GetThreadDesktop")
	setDesktop := postItUser32.NewProc("SetThreadDesktop")
	closeDesktop := postItUser32.NewProc("CloseDesktop")
	thread, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId").Call()
	previous, _, _ := getDesktop.Call(thread)
	name, _ := syscall.UTF16PtrFromString(fmt.Sprintf("VivaPadPostIt-%d", os.Getpid()))
	desktop, _, err := postItUser32.NewProc("CreateDesktopW").Call(uintptr(unsafe.Pointer(name)), 0, 0, 0, 0x01ff, 0)
	if desktop == 0 {
		return nil, err
	}
	if ok, _, err := setDesktop.Call(desktop); ok == 0 {
		closeDesktop.Call(desktop)
		return nil, err
	}
	return func() {
		setDesktop.Call(previous)
		closeDesktop.Call(desktop)
	}, nil
}

func findNativePostItWindow() uintptr {
	title, _ := syscall.UTF16PtrFromString(postItNativeTitle)
	id, _, _ := postItUser32.NewProc("FindWindowW").Call(0, uintptr(unsafe.Pointer(title)))
	return id
}

func focusNativePostItWindow() {
	if id := findNativePostItWindow(); id != 0 {
		postItUser32.NewProc("SetFocus").Call(id)
	}
}

func inspectNativePostItWindow() postItNativeWindow {
	id := findNativePostItWindow()
	if id == 0 {
		return postItNativeWindow{}
	}
	getStyle := postItUser32.NewProc("GetWindowLongW")
	style, _, _ := getStyle.Call(id, ^uintptr(15))
	level, _, _ := getStyle.Call(id, ^uintptr(19))
	focus, _, _ := postItUser32.NewProc("GetFocus").Call()
	return postItNativeWindow{id: id, decorated: style&0x00c00000 != 0, topmost: level&8 != 0, focused: focus == id}
}

func sendNativePostItKey(id uintptr, f12 bool) {
	post := postItUser32.NewProc("PostMessageW")
	if f12 {
		post.Call(id, 0x100, 0x7b, 1)          // WM_KEYDOWN, VK_F12
		post.Call(id, 0x101, 0x7b, 0xc0000001) // WM_KEYUP
	} else {
		post.Call(id, 0x102, 'x', 1) // WM_CHAR
	}
}
