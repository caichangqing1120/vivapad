package windows_test

import (
	"os"
	"path/filepath"
	"testing"

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
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", "-openSession", xml)
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
