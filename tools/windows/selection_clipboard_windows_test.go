package windows_test

import (
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"unsafe"

	"vivapad/tools/windows"
)

const clipboardReceiverSource = "111\r\n222\r\n333\r\n444\r\n555\r\n"
const clipboardReceiverPasted = "1c11\r\n2d22\r\n333\r\n4a44\r\n555\r\n"

func TestNotepadPlusPlusSelectionHash(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	path := filepath.Join(t.TempDir(), "mixed-selection-hash.txt")
	if err := os.WriteFile(path, []byte(windows.SelectionHashSource), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
	editor.focusText(t)
	send := focusedScintilla(t, editor)
	const hashCommand = 48506 // IDM_TOOL_SHA256_GENERATEINTOCLIPBOARD in Notepad++ v8.9.8.1
	const bufferSize = uintptr(len(windows.SelectionHashSource) - len("\ufeff"))
	if length, codePage := send(2006, 0, 0), send(2137, 0, 0); length != bufferSize || codePage != 65001 {
		t.Fatalf("reference buffer length=%d code page=%d, want %d bytes of UTF-8 without BOM", length, codePage, bufferSize)
	}
	send(2572, bufferSize, 0) // SCI_SETSELECTION
	sequence := windows.ClipboardSequence()
	invokeNotepadCommand(t, editor, hashCommand)
	if windows.ClipboardSequence() == sequence {
		t.Fatal("Notepad++ did not publish a digest")
	}
	if err := windows.CheckSelectionHashClipboard(); err != nil {
		t.Fatal(err)
	}
	if send(2585, 0, 0) != 0 || send(2587, 0, 0) != bufferSize {
		t.Fatal("Notepad++ changed the single selection while hashing")
	}
	send(2563, 1, 0) // SCI_SETMULTIPLESELECTION
	send(2572, uintptr(len("caf\u00e9")), 0)
	send(2573, uintptr(len("caf\u00e9 caf\u00e9")), uintptr(len("caf\u00e9 ")))
	if got := send(2570, 0, 0); got != 2 { // SCI_GETSELECTIONS
		t.Fatalf("Notepad++ selections = %d, want 2", got)
	}
	sequence = windows.ClipboardSequence()
	invokeNotepadCommand(t, editor, hashCommand)
	if windows.ClipboardSequence() != sequence || send(2570, 0, 0) != 2 || send(2159, 0, 0) != 0 {
		t.Fatal("Notepad++ changed the clipboard, selections, or document on multiple ranges")
	}
	if err := windows.CheckSelectionHashClipboard(); err != nil {
		t.Fatal(err)
	}
	editor.close(t)
	awaitBytes(t, path, windows.SelectionHashSource)
}

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
	send := focusedScintilla(t, editor)
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

func TestNotepadPlusPlusMultiSelectionWordBoundaries(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	for _, tc := range []struct {
		name, value string
		initial     [2]uintptr
		want        [][2]uintptr
	}{
		{"combining mark at caret", "e\u0301 x", [2]uintptr{3, 3}, [][2]uintptr{{0, 3}}},
		{"number at caret", "x\u00b2 z", [2]uintptr{3, 3}, [][2]uintptr{{0, 3}}},
		{"combining mark excludes partial match", "e e\u0301 e", [2]uintptr{0, 1}, [][2]uintptr{{0, 1}, {6, 7}}},
		{"punctuation respects class edges", ": :: x:y :", [2]uintptr{0, 1}, [][2]uintptr{{0, 1}, {6, 7}, {9, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "words.txt")
			if err := os.WriteFile(path, []byte("\ufeff"+tc.value), 0600); err != nil {
				t.Fatal(err)
			}
			editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
			editor.focusText(t)
			send := focusedScintilla(t, editor)
			if got := send(2006, 0, 0); got != uintptr(len(tc.value)) || send(2137, 0, 0) != 65001 {
				t.Fatalf("Scintilla fixture is not UTF-8 at the expected length: %d", got)
			}
			send(2563, 1, 0)                                // SCI_SETMULTIPLESELECTION
			for _, command := range []uintptr{2688, 2689} { // SCI_MULTIPLESELECTADDNEXT / ADDEACH
				send(2572, tc.initial[1], tc.initial[0]) // SCI_SETSELECTION
				send(2690, 0, 0)                         // SCI_TARGETWHOLEDOCUMENT
				send(2198, 2, 0)                         // SCI_SETSEARCHFLAGS: SCFIND_WHOLEWORD
				send(command, 0, 0)
				var got [][2]uintptr
				for i := range send(2570, 0, 0) { // SCI_GETSELECTIONS
					got = append(got, [2]uintptr{send(2585, i, 0), send(2587, i, 0)}) // SCI_GETSELECTIONNSTART / END
				}
				want := tc.want
				if command == 2688 && len(want) > 2 {
					want = want[:2]
				}
				if !slices.Equal(got, want) {
					t.Fatalf("Scintilla %d selected byte ranges %v, want %v", command, got, want)
				}
			}
			editor.close(t)
		})
	}
}

func TestNotepadPlusPlusFirstMultiSelectFromCaret(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	const value = "cat CAT cat cats"
	path := filepath.Join(t.TempDir(), "first-selection.txt")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
	editor.focusText(t)
	send := focusedScintilla(t, editor)
	if got := send(2006, 0, 0); got != uintptr(len(value)) { // SCI_GETLENGTH
		t.Fatalf("Scintilla document length = %d, want %d", got, len(value))
	}
	seed, first, upper, after := [2]uintptr{8, 11}, [2]uintptr{0, 3}, [2]uintptr{4, 7}, [2]uintptr{12, 15}
	for i, tc := range []struct {
		name string
		want [][2]uintptr
	}{
		{"unrestricted", [][2]uintptr{seed, after, first, upper}},
		{"match case", [][2]uintptr{seed, after, first}},
		{"whole word", [][2]uintptr{seed, first, upper}},
		{"match case and whole word", [][2]uintptr{seed, first}},
	} {
		for _, all := range []bool{true, false} {
			mode, command, want := "Next", 42094+uintptr(i), [][2]uintptr{seed}
			if all {
				mode, command, want = "All", 42090+uintptr(i), tc.want
			}
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				send(2556, 9, 0) // SCI_SETEMPTYSELECTION
				var result uintptr
				if ok, _, err := sendMessageTimeout.Call(editor.hwnd, 0x0111, command, 0, 2, 1000, uintptr(unsafe.Pointer(&result))); ok == 0 {
					t.Fatalf("Notepad++ menu command %d: %v", command, err)
				}
				var got [][2]uintptr
				for n := range send(2570, 0, 0) { // SCI_GETSELECTIONS
					got = append(got, [2]uintptr{send(2585, n, 0), send(2587, n, 0)})
				}
				if main := send(2575, 0, 0); !slices.Equal(got, want) || main != uintptr(len(want)-1) {
					t.Fatalf("menu selected %v, main=%d; want %v", got, main, want)
				}
				if send(2159, 0, 0) != 0 { // SCI_GETMODIFY
					t.Fatal("menu changed the document")
				}
			})
		}
	}
	editor.close(t)
}

func TestNotepadPlusPlusMultiSelectSearchOrigin(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	for _, tc := range []struct {
		name, value string
		option      uintptr
		seed        [2]uintptr
		want        [][2]uintptr
	}{
		{"adjacent match", "aaaaaa", 1, [2]uintptr{1, 3}, [][2]uintptr{{1, 3}, {3, 5}}},
		{"forward then wrap", "aaaaaaaaaaaa", 1, [2]uintptr{3, 5}, [][2]uintptr{{3, 5}, {5, 7}, {7, 9}, {9, 11}, {0, 2}}},
		{"UTF-8 offsets", "猫猫猫猫猫猫", 1, [2]uintptr{3, 9}, [][2]uintptr{{3, 9}, {9, 15}}},
		{"whole word", "aa aaaa aa", 2, [2]uintptr{3, 5}, [][2]uintptr{{3, 5}, {8, 10}, {0, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "search-origin.txt")
			if err := os.WriteFile(path, []byte("\ufeff"+tc.value), 0600); err != nil {
				t.Fatal(err)
			}
			editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
			editor.focusText(t)
			send := focusedScintilla(t, editor)
			if got := send(2006, 0, 0); got != uintptr(len(tc.value)) || send(2137, 0, 0) != 65001 {
				t.Fatalf("Scintilla fixture is not UTF-8 at the expected length: %d", got)
			}
			for _, all := range []bool{false, true} {
				command, want := 42094+tc.option, tc.want[:2]
				if all {
					command, want = 42090+tc.option, tc.want
				}
				send(2572, tc.seed[1], tc.seed[0]) // SCI_SETSELECTION
				var result uintptr
				if ok, _, err := sendMessageTimeout.Call(editor.hwnd, 0x0111, command, 0, 2, 1000, uintptr(unsafe.Pointer(&result))); ok == 0 {
					t.Fatalf("Notepad++ menu command %d: %v", command, err)
				}
				var got [][2]uintptr
				for n := range send(2570, 0, 0) { // SCI_GETSELECTIONS
					got = append(got, [2]uintptr{send(2585, n, 0), send(2587, n, 0)})
				}
				if main := send(2575, 0, 0); !slices.Equal(got, want) || main != uintptr(len(want)-1) {
					t.Fatalf("menu %d selected %v, main=%d; want %v", command, got, main, want)
				}
			}
			editor.close(t)
		})
	}
}

func TestNotepadPlusPlusMultiSelectFromCarets(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	const value = "new new new."
	path := filepath.Join(t.TempDir(), "collapsed-selection.txt")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
	editor.focusText(t)
	send := focusedScintilla(t, editor)
	if got := send(2006, 0, 0); got != uintptr(len(value)) {
		t.Fatalf("length = %d, want %d", got, len(value))
	}
	send(2563, 1, 0) // SCI_SETMULTIPLESELECTION
	for _, tc := range []struct {
		name    string
		command uintptr
		want    [][2]uintptr
	}{
		{"Next", 42096, [][2]uintptr{{3, 3}, {12, 12}, {4, 7}}},
		{"All", 42092, [][2]uintptr{{12, 12}, {4, 7}, {8, 11}, {0, 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			send(2556, 3, 0)   // SCI_SETEMPTYSELECTION
			send(2573, 12, 12) // SCI_ADDSELECTION, unrelated caret
			send(2573, 7, 7)   // SCI_ADDSELECTION, main caret
			var result uintptr
			if ok, _, err := sendMessageTimeout.Call(editor.hwnd, 0x0111, tc.command, 0, 2, 1000, uintptr(unsafe.Pointer(&result))); ok == 0 {
				t.Fatalf("Notepad++ menu command %d: %v", tc.command, err)
			}
			var got [][2]uintptr
			for n := range send(2570, 0, 0) { // SCI_GETSELECTIONS
				got = append(got, [2]uintptr{send(2585, n, 0), send(2587, n, 0)})
			}
			if main := send(2575, 0, 0); !slices.Equal(got, tc.want) || main != uintptr(len(tc.want)-1) {
				t.Fatalf("menu selected %v, main=%d; want %v", got, main, tc.want)
			}
			if send(2159, 0, 0) != 0 { // SCI_GETMODIFY
				t.Fatal("menu changed the document")
			}
		})
	}
	editor.close(t)
}

func TestNotepadPlusPlusMultiSelectAllFromMixedSelection(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	const value = "old new old new!"
	path := filepath.Join(t.TempDir(), "mixed-selection.txt")
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
	editor.focusText(t)
	send := focusedScintilla(t, editor)
	if got := send(2006, 0, 0); got != uintptr(len(value)) {
		t.Fatalf("length = %d, want %d", got, len(value))
	}
	send(2563, 1, 0)   // SCI_SETMULTIPLESELECTION
	send(2572, 3, 0)   // SCI_SETSELECTION
	send(2573, 16, 16) // SCI_ADDSELECTION, unrelated caret
	send(2573, 6, 6)   // SCI_ADDSELECTION, main caret inside the word
	const allWholeWord = 42092
	var result uintptr
	if ok, _, err := sendMessageTimeout.Call(editor.hwnd, 0x0111, allWholeWord, 0, 2, 1000, uintptr(unsafe.Pointer(&result))); ok == 0 {
		t.Fatalf("Notepad++ All menu command: %v", err)
	}
	var got [][2]uintptr
	for n := range send(2570, 0, 0) { // SCI_GETSELECTIONS
		got = append(got, [2]uintptr{send(2585, n, 0), send(2587, n, 0)})
	}
	want := [][2]uintptr{{0, 3}, {16, 16}, {4, 7}, {12, 15}}
	if main := send(2575, 0, 0); !slices.Equal(got, want) || main != uintptr(len(want)-1) {
		t.Fatalf("menu selected %v, main=%d; want %v", got, main, want)
	}
	if send(2159, 0, 0) != 0 { // SCI_GETMODIFY
		t.Fatal("menu changed the document")
	}
	editor.close(t)
}

func focusedScintilla(t *testing.T, editor *desktopEditor) func(uintptr, uintptr, uintptr) uintptr {
	t.Helper()
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
	return func(message, first, second uintptr) uintptr {
		t.Helper()
		var result uintptr
		if ok, _, err := sendMessageTimeout.Call(info.focus, message, first, second, 2, 1000, uintptr(unsafe.Pointer(&result))); ok == 0 {
			t.Fatalf("Scintilla message %#x: %v", message, err)
		}
		return result
	}
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
