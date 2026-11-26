package windows_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/app"
	"vivapad/internal/session"
)

const legacySessionDisk = "\x93\xfa\x96\x7b\r\nabc"
const legacySessionBuffer = "\u65e5\u672c\r\nabc"
const legacySessionAnchor, legacySessionCaret = len("\u65e5\u672c\r\n"), len("\u65e5")

func writeLegacySelectionSession(t *testing.T, root string) (path, xml string) {
	t.Helper()
	path = filepath.Join(root, "japanese.txt")
	if err := os.WriteFile(path, []byte(legacySessionDisk), 0600); err != nil {
		t.Fatal(err)
	}
	xml = filepath.Join(root, "selection.xml")
	stored := session.NPPState{Main: []session.NPPFile{{
		Path: path, Language: "normal", Encoding: 932, StartPos: int64(legacySessionAnchor), EndPos: int64(legacySessionCaret),
	}}}
	if err := session.SaveNPP(xml, stored); err != nil {
		t.Fatal(err)
	}
	return path, xml
}

func TestNotepadPlusPlusLegacySessionSelection(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	root := t.TempDir()
	path, xml := writeLegacySelectionSession(t, root)
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
	if length, codePage := sci(2006, 0, 0), sci(2137, 0, 0); length != uintptr(len(legacySessionBuffer)) || codePage != 65001 {
		t.Fatalf("reference buffer length=%d code page=%d, want %d bytes of UTF-8", length, codePage, len(legacySessionBuffer))
	}
	if gotAnchor, gotCaret := sci(2009, 0, 0), sci(2008, 0, 0); gotAnchor != uintptr(legacySessionAnchor) || gotCaret != uintptr(legacySessionCaret) {
		t.Fatalf("reference restored anchor=%d caret=%d, want %d and %d", gotAnchor, gotCaret, legacySessionAnchor, legacySessionCaret)
	}
	editor.close(t)
	awaitBytes(t, path, legacySessionDisk)
}

func TestVivaPadLegacySessionSelection(t *testing.T) {
	binary := os.Getenv("VIVAPAD_RTL_DESKTOP_EXE")
	if binary == "" {
		t.Skip("requires built VivaPad on an isolated Windows desktop")
	}
	root := t.TempDir()
	path, xml := writeLegacySelectionSession(t, root)
	config := filepath.Join(root, "appdata")
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "APPDATA="+config, "LOCALAPPDATA="+config, "VIVA_ACCESSIBILITY=1")
	editor := openDesktopEditor(t, binary, "VivaPad", env)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dpiContext := user32.NewProc("SetThreadDpiAwarenessContext")
	prior, _, err := dpiContext.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2
	if prior == 0 {
		t.Fatalf("use physical desktop coordinates: %v", err)
	}
	defer dpiContext.Call(prior)
	helpResize(t, editor, 960, 600)
	helpClick(t, editor, "File")
	helpClick(t, editor, "Sessions")
	helpClick(t, editor, "Load Session...")
	picker := waitForSessionPicker(t, editor)
	if err := app.SetClipboardText(xml); err != nil {
		t.Fatal(err)
	}
	picker.keys(t, keyLeftAlt, 'N') // File name field in the English Windows picker.
	picker.keys(t, keyLeftControl, 'V')
	picker.keys(t, 0x0d) // Enter
	helpWait(t, editor, "imported session tab", func(nodes []helpNode) bool {
		_, found := helpFind(nodes, filepath.Base(path))
		return found
	})
	awaitBytes(t, path, legacySessionDisk)

	// Do not click the text or move the caret after the picker. The first key
	// must replace the import's restored selection, including its CRLF.
	editor.keys(t, 'X')
	editor.keys(t, keyLeftControl, 'S')
	awaitBytes(t, path, "\x93\xfaxabc")
	editor.keys(t, keyLeftControl, 'Z')
	editor.keys(t, keyLeftControl, 'S')
	awaitBytes(t, path, legacySessionDisk)
	editor.close(t)
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
		title = sessionWindowTitle(editor.hwnd)
		if strings.Contains(title, name) {
			t.Logf("session document active: %q", title)
			return
		}
	}
	t.Fatalf("Notepad++ never activated session document %q; final window title %q", name, title)
}

func waitForSessionPicker(t *testing.T, editor *desktopEditor) *desktopEditor {
	t.Helper()
	var owner uint32
	windowPID.Call(editor.hwnd, uintptr(unsafe.Pointer(&owner)))
	var title string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(40 * time.Millisecond) {
		active, _, _ := getForegroundWindow.Call()
		var pid uint32
		if active != 0 && active != editor.hwnd {
			windowPID.Call(active, uintptr(unsafe.Pointer(&pid)))
			title = sessionWindowTitle(active)
			if owner != 0 && pid == owner && title == "Load Session" {
				return &desktopEditor{hwnd: active}
			}
		}
	}
	t.Fatalf("VivaPad's Load Session picker did not become active; last dialog title %q", title)
	return nil
}

func sessionWindowTitle(hwnd uintptr) string {
	var text [512]uint16
	getWindowText.Call(hwnd, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	return syscall.UTF16ToString(text[:])
}
