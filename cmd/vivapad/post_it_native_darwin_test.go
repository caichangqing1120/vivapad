package main

import "github.com/ebitengine/purego/objc"

func prepareNativePostItDesktop() (func(), error) { return func() {}, nil }

func focusNativePostItWindow() {} // AppKit activates the window when it opens.

func inspectNativePostItWindow() postItNativeWindow {
	nativeApp := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	window := nativeApp.Send(objc.RegisterName("keyWindow"))
	if window == 0 || window.Send(objc.RegisterName("isVisible")) == 0 {
		return postItNativeWindow{}
	}
	style := window.Send(objc.RegisterName("styleMask"))
	return postItNativeWindow{id: uintptr(window), decorated: style&1 != 0,
		topmost: window.Send(objc.RegisterName("level")) == 3,
		focused: window.Send(objc.RegisterName("firstResponder")) == window.Send(objc.RegisterName("contentView"))}
}

func sendNativePostItKey(id uintptr, f12 bool) {
	window := objc.ID(id)
	characters, keyCode := "x", uint16(7)
	if f12 {
		characters, keyCode = "\uF70F", 111 // NSF12FunctionKey and kVK_F12
	}
	value := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), characters)
	keyEvent := objc.ID(objc.GetClass("NSEvent")).Send(objc.RegisterName("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
		uintptr(10), struct{ X, Y float64 }{}, uintptr(0), float64(0), window.Send(objc.RegisterName("windowNumber")), objc.ID(0), value, value, false, keyCode)
	window.Send(objc.RegisterName("sendEvent:"), keyEvent)
}
