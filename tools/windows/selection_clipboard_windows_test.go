package windows_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"vivapad/tools/windows"
)

func TestNotepadPlusPlusMultiSelectionClipboard(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	path := filepath.Join(t.TempDir(), "selection.txt")
	if err := os.WriteFile(path, []byte(windows.SelectionClipboardSource), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
	editor.focusText(t)
	info := struct {
		size, flags                                   uint32
		active, focus, capture, menu, moveSize, caret uintptr
		caretRect                                     [4]int32
	}{}
	info.size = uint32(unsafe.Sizeof(info))
	thread, _, _ := windowPID.Call(editor.hwnd, 0)
	if ok, _, _ := getGUIThreadInfo.Call(thread, uintptr(unsafe.Pointer(&info))); ok == 0 || info.focus == 0 {
		t.Fatal("could not read focused control")
	}
	var name [128]uint16
	getClassName.Call(info.focus, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if class := syscall.UTF16ToString(name[:]); class != "Scintilla" {
		t.Fatalf("focus = %q, want Scintilla", class)
	}
	send := func(message, first, second uintptr) uintptr {
		t.Helper()
		var result uintptr
		if ok, _, err := sendMessageTimeout.Call(info.focus, message, first, second, 2, 1000, uintptr(unsafe.Pointer(&result))); ok == 0 {
			t.Fatalf("Scintilla message %#x: %v", message, err)
		}
		return result
	}
	if got := send(2006, 0, 0); got != uintptr(len(windows.SelectionClipboardSource)) {
		t.Fatalf("Scintilla file length = %d", got)
	}
	send(2031, 0, 0) // SCI_SETEOLMODE: CRLF
	send(2563, 1, 0) // SCI_SETMULTIPLESELECTION
	send(2572, 8, 5) // SCI_SETSELECTION: c\rd
	send(2573, 3, 3) // SCI_ADDSELECTION: empty before b
	send(2573, 3, 0) // SCI_ADDSELECTION: a\r\n
	if got := send(2570, 0, 0); got != 3 {
		t.Fatalf("Scintilla selection count = %d, want 3", got)
	}
	for _, message := range []uintptr{0x0301, 0x0300} { // WM_COPY, WM_CUT
		sequence := windows.ClipboardSequence()
		send(message, 0, 0)
		if sequence == windows.ClipboardSequence() {
			t.Fatalf("message %#x did not write to the system clipboard", message)
		}
		if err := windows.CheckSelectionClipboard(); err != nil {
			t.Fatalf("clipboard after %#x: %v", message, err)
		}
	}
	send(2176, 0, 0) // SCI_UNDO before closing the reference document
	editor.close(t)
}
