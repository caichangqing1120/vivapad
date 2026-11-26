package windows_test

import (
	"os"
	"testing"
	"unsafe"

	"vivapad/tools/windows"
)

func TestNotepadPlusPlusDocumentPathClipboard(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	mainPath, subPath := windows.DocumentPathFixture(t)
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", mainPath, subPath)
	const (
		nppMessage          = 0x400 + 1000
		nppGetOpenFiles     = nppMessage + 7
		nppActivateDocument = nppMessage + 28
		nppGetCurrentView   = nppMessage + 88
		nppMoveToOtherView  = 10001
		nppCloneToOtherView = 10002
	)
	send := func(message, first, second uintptr) uintptr {
		t.Helper()
		var result uintptr
		if ok, _, err := sendMessageTimeout.Call(editor.hwnd, message, first, second, 2, 3000, uintptr(unsafe.Pointer(&result))); ok == 0 {
			t.Fatalf("Notepad++ message %#x: %v", message, err)
		}
		return result
	}
	if count := send(nppGetOpenFiles, 0, 1); count != 2 || send(nppActivateDocument, 0, 1) == 0 {
		t.Fatalf("expected two fixture documents in the main view, got %d", count)
	}
	if got := focusedScintilla(t, editor)(2006, 0, 0); got != uintptr(len(windows.DocumentPathSubText)) {
		t.Fatalf("second fixture buffer length = %d", got)
	}
	invokeNotepadCommand(t, editor, nppMoveToOtherView)
	if send(nppActivateDocument, 0, 0) == 0 {
		t.Fatal("could not activate the main document before cloning")
	}
	invokeNotepadCommand(t, editor, nppCloneToOtherView)
	if mainCount, subCount := send(nppGetOpenFiles, 0, 1), send(nppGetOpenFiles, 0, 2); mainCount != 1 || subCount != 2 {
		t.Fatalf("main and sub document counts = %d, %d; want one main and two sub tabs", mainCount, subCount)
	}
	if send(nppActivateDocument, 0, 0) == 0 || send(nppGetCurrentView, 0, 0) != 0 {
		t.Fatal("could not activate the main copy of the cloned document")
	}
	sci := focusedScintilla(t, editor)
	if got := sci(2006, 0, 0); got != uintptr(len(windows.DocumentPathMainText)) {
		t.Fatalf("main fixture buffer length = %d", got)
	}
	sci(2572, 5, 1) // SCI_SETSELECTION
	// Edit menu IDs in the pinned Notepad++ source, in submenu order.
	commands := [...]uintptr{42029, 42030, 42031, 42087, 42088}
	wants := windows.DocumentPathClipboardValues(mainPath, subPath)
	for i, command := range commands {
		sequence := windows.ClipboardSequence()
		invokeNotepadCommand(t, editor, command)
		if err := windows.CheckDocumentPathClipboard(sequence, wants[i]); err != nil {
			t.Fatalf("command %d: %v", command, err)
		}
		if send(nppGetCurrentView, 0, 0) != 0 || sci(2008, 0, 0) != 5 || sci(2009, 0, 0) != 1 || sci(2159, 0, 0) != 0 || sci(2174, 0, 0) != 0 {
			t.Fatalf("command %d changed the active view, selection, dirty state, or undo history", command)
		}
	}
	editor.close(t)
	awaitBytes(t, mainPath, windows.DocumentPathMainText)
	awaitBytes(t, subPath, windows.DocumentPathSubText)
}
