package windows_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"vivapad/internal/session"
)

func TestNotepadPlusPlusSummaryWords(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	// Also used in TestDocumentSummaryMatchesWindowsReferenceWords.
	const source = "alpha,beta\r\nx\u0301y _\n\U0001f642 \U0001f642\r\u732b\uff0f\u72ac"
	const fixture = "\ufeff" + source
	for _, tc := range []struct {
		name string
		end  uintptr
	}{
		{"empty selection", 0},
		{"selected first word", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "summary-words.txt")
			if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
				t.Fatal(err)
			}
			xml := filepath.Join(root, "summary.xml")
			stored := session.NPPState{Main: []session.NPPFile{{
				Path: path, Language: "normal", Encoding: -1, EndPos: int64(tc.end),
			}}}
			if err := session.SaveNPP(xml, stored); err != nil {
				t.Fatal(err)
			}
			settings := filepath.Join(root, "settings")
			if err := os.Mkdir(settings, 0700); err != nil {
				t.Fatal(err)
			}
			const config = `<NotepadPlus><GUIConfigs><GUIConfig name="NewDocDefaultSettings" addNewDocumentOnStartup="no" /></GUIConfigs></NotepadPlus>`
			if err := os.WriteFile(filepath.Join(settings, "config.xml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", "-settingsDir="+settings, "-openSession", xml)
			awaitSessionDocument(t, editor, filepath.Base(path))
			view := awaitSessionBuffer(t, editor, source)
			read := func(message uintptr) uintptr {
				t.Helper()
				value, ok := sessionScintillaValue(view, message, 0)
				if !ok {
					t.Fatalf("read Scintilla message %#x from view %#x", message, view)
				}
				return value
			}
			type viewState struct{ anchor, caret, length, modified uintptr }
			readView := func() viewState {
				return viewState{read(2009), read(2008), read(2006), read(2159)}
			}
			before := readView()
			want := viewState{caret: tc.end, length: uintptr(len(source))}
			if before != want {
				t.Fatalf("selection fixture before Summary = %+v, want %+v", before, want)
			}
			const summaryCommand = 44049 // IDM_VIEW_SUMMARY in Notepad++ v8.9.8.1
			if ok, _, err := postMessage.Call(editor.hwnd, 0x0111, summaryCommand, 0); ok == 0 {
				t.Fatalf("post Notepad++ Summary command: %v", err)
			}
			dialog, words := readNotepadSummary(t, editor.hwnd)
			if words != "7" {
				t.Errorf("Notepad++ Summary Words = %q, want 7", words)
			}
			// Both Notepad++ Summary dialog styles support closing the dialog itself.
			if ok, _, err := postMessage.Call(dialog, 0x0010, 0, 0); ok == 0 { // WM_CLOSE
				t.Fatalf("close Notepad++ Summary: %v", err)
			}
			isWindow := user32.NewProc("IsWindow")
			for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(25 * time.Millisecond) {
				if open, _, _ := isWindow.Call(dialog); open == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("Notepad++ Summary did not close")
				}
			}
			if after := readView(); after != before {
				t.Fatalf("Notepad++ Summary changed the selection or document: before %+v; after %+v", before, after)
			}
			editor.close(t)
			awaitBytes(t, path, fixture)
		})
	}
}

func readNotepadSummary(t *testing.T, editor uintptr) (uintptr, string) {
	t.Helper()
	getWindow := user32.NewProc("GetWindow")
	var dialog uintptr
	findDialog := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		owner, _, _ := getWindow.Call(hwnd, 4) // GW_OWNER
		if owner == editor {
			var caption [32]uint16
			getWindowText.Call(hwnd, uintptr(unsafe.Pointer(&caption[0])), uintptr(len(caption)))
			if syscall.UTF16ToString(caption[:]) == "Summary" {
				dialog = hwnd
				return 0
			}
		}
		return 1
	})
	enumChildren := user32.NewProc("EnumChildWindows")
	words := ""
	readChild := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		// WM_GETTEXT marshals the label across the process boundary. GetWindowText
		// only returns another process's top-level caption.
		var label [4096]uint16
		var copied uintptr
		ok, _, _ := sendMessageTimeout.Call(hwnd, 0x000d, uintptr(len(label)), uintptr(unsafe.Pointer(&label[0])), 2, 500, uintptr(unsafe.Pointer(&copied)))
		if ok != 0 {
			for _, line := range strings.FieldsFunc(syscall.UTF16ToString(label[:]), func(r rune) bool { return r == '\r' || r == '\n' }) {
				if value, found := strings.CutPrefix(line, "Words:"); found {
					words = strings.TrimSpace(value)
					return 0
				}
			}
		}
		return 1
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		if dialog == 0 {
			enumWindows.Call(findDialog, 0)
		}
		if dialog != 0 {
			enumChildren.Call(dialog, readChild, 0)
			if words != "" {
				return dialog, words
			}
		}
	}
	t.Fatalf("did not find the Words row in the Notepad++ Summary dialog (window %#x)", dialog)
	return 0, ""
}
