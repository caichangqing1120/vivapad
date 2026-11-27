package windows_test

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestNotepadPlusPlusFinderFocusRoundTrip(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	const (
		source       = "finder_token\r\nseparate line\r\nfinder_token\r\n"
		query        = "finder_token"
		keyF7        = 0x76
		wmSetText    = 0x000c
		bmClick      = 0x00f5
		findWhat     = 1601 // IDFINDWHAT in Notepad++ v8.9.8.1
		findAll      = 1641 // IDC_FINDALL_CURRENTFILE
		gaRoot       = 2
		getLength    = 2006 // SCI_GETLENGTH
		getAnchor    = 2009 // SCI_GETANCHOR
		getCaret     = 2008 // SCI_GETCURRENTPOS
		getReadOnly  = 2140 // SCI_GETREADONLY
		getModified  = 2159 // SCI_GETMODIFY
		setSelection = 2572 // SCI_SETSELECTION
		anchor       = uintptr(len("finder_token\r\nseparate"))
		caret        = uintptr(len("finder_token\r\n"))
	)
	path := filepath.Join(t.TempDir(), "finder-focus.txt")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
	editor.focusText(t)
	document := focusedDesktopControl(t, editor)
	readDocument := scintillaAt(t, document)
	if got := readDocument(getLength, 0, 0); got != uintptr(len(source)) {
		t.Fatalf("document length = %d, want %d", got, len(source))
	}
	readDocument(setSelection, caret, anchor)
	checkDocument := func(stage string) {
		t.Helper()
		gotAnchor, gotCaret := readDocument(getAnchor, 0, 0), readDocument(getCaret, 0, 0)
		if gotAnchor != anchor || gotCaret != caret || readDocument(getModified, 0, 0) != 0 {
			t.Fatalf("%s: document anchor/caret = %d/%d, want %d/%d with an unmodified buffer", stage, gotAnchor, gotCaret, anchor, caret)
		}
	}
	checkDocument("before search")
	editor.keys(t, keyLeftControl, 'F')
	getAncestor := user32.NewProc("GetAncestor")
	getDlgItem := user32.NewProc("GetDlgItem")
	dialog, _, _ := getAncestor.Call(focusedDesktopControl(t, editor), gaRoot)
	field, _, _ := getDlgItem.Call(dialog, findWhat)
	button, _, _ := getDlgItem.Call(dialog, findAll)
	if dialog == 0 || dialog == editor.hwnd || field == 0 || button == 0 {
		t.Fatalf("Find dialog did not expose Find What and Find All controls: dialog=%#x field=%#x button=%#x", dialog, field, button)
	}
	value, err := syscall.UTF16PtrFromString(query)
	if err != nil {
		t.Fatal(err)
	}
	var result uintptr
	ok, _, err := sendMessageTimeout.Call(field, wmSetText, 0, uintptr(unsafe.Pointer(value)), 2, 1000, uintptr(unsafe.Pointer(&result)))
	runtime.KeepAlive(value)
	if ok == 0 || result == 0 {
		t.Fatalf("set Find What: %v", err)
	}
	if ok, _, err := sendMessageTimeout.Call(button, bmClick, 0, 0, 2, 5000, uintptr(unsafe.Pointer(&result))); ok == 0 {
		t.Fatalf("click Find All in Current Document: %v", err)
	}
	finder := focusedDesktopControl(t, editor)
	if finder == document {
		t.Fatal("Find All left focus in the editor instead of the result pane")
	}
	readFinder := scintillaAt(t, finder)
	if readFinder(getReadOnly, 0, 0) == 0 || readFinder(getLength, 0, 0) <= uintptr(len(source)) {
		t.Fatal("focused control does not contain the read-only search results")
	}
	checkDocument("after search")
	for _, step := range []struct {
		name string
		want uintptr
	}{{"editor", document}, {"Finder", finder}, {"editor again", document}} {
		editor.keys(t, keyF7)
		if got := focusedDesktopControl(t, editor); got != step.want {
			t.Fatalf("F7 to %s: focus = %#x, want %#x", step.name, got, step.want)
		}
		checkDocument(step.name)
		t.Logf("F7 focused %s without changing the document selection", step.name)
	}
	editor.close(t)
	awaitBytes(t, path, source)
}
