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

func TestNotepadPlusPlusLegacySessionSelection(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	root := t.TempDir()
	path := filepath.Join(root, "japanese.txt")
	const disk = "\x93\xfa\x96\x7b\r\nabc"
	const buffer = "\u65e5\u672c\r\nabc"
	if err := os.WriteFile(path, []byte(disk), 0600); err != nil {
		t.Fatal(err)
	}
	const anchor, caret = len("\u65e5\u672c\r\n"), len("\u65e5")
	xml := filepath.Join(root, "selection.xml")
	stored := session.NPPState{Main: []session.NPPFile{{
		Path: path, Language: "normal", Encoding: 932, StartPos: int64(anchor), EndPos: int64(caret),
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
	sci := focusedScintilla(t, editor)
	if length, codePage := sci(2006, 0, 0), sci(2137, 0, 0); length != uintptr(len(buffer)) || codePage != 65001 {
		t.Fatalf("reference buffer length=%d code page=%d, want %d bytes of UTF-8", length, codePage, len(buffer))
	}
	if gotAnchor, gotCaret := sci(2009, 0, 0), sci(2008, 0, 0); gotAnchor != uintptr(anchor) || gotCaret != uintptr(caret) {
		t.Fatalf("reference restored anchor=%d caret=%d, want %d and %d", gotAnchor, gotCaret, anchor, caret)
	}
	editor.close(t)
	awaitBytes(t, path, disk)
}

func awaitSessionDocument(t *testing.T, editor *desktopEditor, name string) {
	t.Helper()
	var title string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		select {
		case <-editor.done:
			t.Fatalf("Notepad++ exited before activating session document %q: %v", name, editor.err)
		default:
		}
		var text [512]uint16
		getWindowText.Call(editor.hwnd, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
		title = syscall.UTF16ToString(text[:])
		if strings.Contains(title, name) {
			t.Logf("session document active: %q", title)
			return
		}
	}
	t.Fatalf("Notepad++ never activated session document %q; final window title %q", name, title)
}
