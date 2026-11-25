package windows_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestNotepadPlusPlusSummaryWords(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	// Also used in TestDocumentSummaryMatchesWindowsReferenceWords.
	const source = "alpha,beta\r\nx\u0301y _\n\U0001f642 \U0001f642\r\u732b\uff0f\u72ac"
	const fixture = "\ufeff" + source
	path := filepath.Join(t.TempDir(), "summary-words.txt")
	if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
	editor.focusText(t)
	send := focusedScintilla(t, editor)
	if got := send(2006, 0, 0); got != uintptr(len(source)) || send(2137, 0, 0) != 65001 {
		t.Fatalf("reference fixture is not UTF-8 at the expected length: %d", got)
	}
	for _, tc := range []struct {
		name string
		end  uintptr
	}{
		{"empty selection", 0},
		{"selected first word", 5},
	} {
		if !t.Run(tc.name, func(t *testing.T) {
			send(2572, tc.end, 0)        // SCI_SETSELECTION
			const summaryCommand = 44049 // IDM_VIEW_SUMMARY in Notepad++ v8.9.8.1
			if ok, _, err := postMessage.Call(editor.hwnd, 0x0111, summaryCommand, 0); ok == 0 {
				t.Fatalf("post Notepad++ Summary command: %v", err)
			}
			dialog, words := readNotepadSummary(t, editor.hwnd)
			if words != "7" {
				t.Errorf("Notepad++ Summary Words = %q, want 7", words)
			}
			button, _, err := user32.NewProc("GetDlgItem").Call(dialog, 1) // IDOK
			if button == 0 {
				t.Fatalf("find Notepad++ Summary OK button: %v", err)
			}
			if ok, _, err := postMessage.Call(button, 0x00f5, 0, 0); ok == 0 { // BM_CLICK
				t.Fatalf("click Notepad++ Summary OK button: %v", err)
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
			if send(2585, 0, 0) != 0 || send(2587, 0, 0) != tc.end || send(2159, 0, 0) != 0 {
				t.Fatal("Notepad++ Summary changed the selection or document")
			}
		}) {
			return
		}
	}
	editor.close(t)
	awaitBytes(t, path, fixture)
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
