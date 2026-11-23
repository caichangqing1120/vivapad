package windows_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"vivapad/tools/windows"
)

const clipboardReceiverSource = "111\r\n222\r\n333\r\n444\r\n555\r\n"
const clipboardReceiverPasted = "1c11\r\n2d22\r\n333\r\n4a44\r\n555\r\n"

func TestNotepadPlusPlusMultiSelectionClipboard(t *testing.T) {
	reference, app := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE"), os.Getenv("VIVAPAD_RTL_DESKTOP_EXE")
	if reference == "" && app == "" {
		t.Skip("requires VivaPad and pinned portable Notepad++ on an isolated Windows desktop")
	}
	if reference == "" || app == "" {
		t.Fatal("both desktop executables are required for the clipboard comparison")
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
	checkClipboardShortcut(t, editor, 'C')
	appData := filepath.Join(t.TempDir(), "appdata")
	if err := os.Mkdir(appData, 0700); err != nil {
		t.Fatal(err)
	}
	pasteIntoDesktopEditor(t, app, "VivaPad", append(os.Environ(), "APPDATA="+appData, "LOCALAPPDATA="+appData))
	checkClipboardShortcut(t, editor, 'X')
	editor.keys(t, keyLeftControl, 'Z')
	editor.close(t)
}

func TestVivaPadMultiSelectionClipboardRoundTrip(t *testing.T) {
	reference, fixture := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE"), os.Getenv("VIVAPAD_CLIPBOARD_FIXTURE_EXE")
	if fixture == "" && reference == "" {
		t.Skip("requires the built VivaPad fixture and pinned portable Notepad++")
	}
	if fixture == "" || reference == "" {
		t.Fatal("both the fixture and Notepad++ are required for the round trip")
	}
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte(windows.SelectionClipboardSource), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, fixture, "VivaPad clipboard fixture", append(os.Environ(), "VIVAPAD_MULTI_SELECTION_FIXTURE="+path), "-test.run=^TestServeWindowsMultiSelectionClipboardFixture$")
	awaitBytes(t, path+".ready", "ready")
	checkClipboardShortcut(t, editor, 'C')
	pasteIntoDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin")
	checkClipboardShortcut(t, editor, 'X')
	editor.keys(t, keyLeftControl, 'S')
	awaitBytes(t, path, "b\n")
	editor.keys(t, keyLeftControl, 'Z')
	editor.keys(t, keyLeftControl, 'S')
	awaitBytes(t, path, windows.SelectionClipboardSource)
	checkClipboardShortcut(t, editor, 'C') // Undo also restores the selected ranges.
	editor.close(t)
}

func checkClipboardShortcut(t *testing.T, editor *desktopEditor, key uintptr) {
	t.Helper()
	sequence := windows.ClipboardSequence()
	editor.keys(t, keyLeftControl, key)
	if sequence == windows.ClipboardSequence() {
		t.Fatalf("Ctrl+%c did not update the Windows clipboard", key)
	}
	if err := windows.CheckSelectionClipboard(); err != nil {
		t.Fatalf("Ctrl+%c: %v", key, err)
	}
}

func pasteIntoDesktopEditor(t *testing.T, executable, title string, env []string, args ...string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "receiver.txt")
	if err := os.WriteFile(path, []byte(clipboardReceiverSource), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, executable, title, env, append(args, path)...)
	editor.focusText(t)
	editor.keys(t, keyLeftControl, 0x24) // Home
	editor.keys(t, 0x27)                 // Right
	editor.keys(t, keyLeftControl, 'V')
	editor.keys(t, keyLeftControl, 'S')
	awaitBytes(t, path, clipboardReceiverPasted)
	editor.close(t)
}
