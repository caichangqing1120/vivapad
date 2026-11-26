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
	hwnd := awaitSessionBuffer(t, editor, legacySessionBuffer)
	gotAnchor, anchorOK := sessionScintillaValue(hwnd, 2009, 0) // SCI_GETANCHOR
	gotCaret, caretOK := sessionScintillaValue(hwnd, 2008, 0)   // SCI_GETCURRENTPOS
	if !anchorOK || !caretOK || gotAnchor != uintptr(legacySessionAnchor) || gotCaret != uintptr(legacySessionCaret) {
		t.Fatalf("reference restored anchor=%d caret=%d, want %d and %d", gotAnchor, gotCaret, legacySessionAnchor, legacySessionCaret)
	}
	editor.close(t)
	awaitBytes(t, path, legacySessionDisk)
}

// The active file can already be in the window title while keyboard focus
// still belongs to an empty Scintilla. Identify the loaded view by its bytes.
func awaitSessionBuffer(t *testing.T, editor *desktopEditor, want string) uintptr {
	t.Helper()
	type observedView struct {
		Length, CodePage uintptr
	}
	var views []observedView
	var matches []uintptr
	enumChildren := user32.NewProc("EnumChildWindows")
	find := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if visible, _, _ := isWindowVisible.Call(hwnd); visible == 0 {
			return 1
		}
		var class [32]uint16
		getClassName.Call(hwnd, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		if syscall.UTF16ToString(class[:]) != "Scintilla" {
			return 1
		}
		length, lengthOK := sessionScintillaValue(hwnd, 2006, 0) // SCI_GETLENGTH
		codePage, pageOK := sessionScintillaValue(hwnd, 2137, 0) // SCI_GETCODEPAGE
		if !lengthOK || !pageOK {
			return 1
		}
		views = append(views, observedView{length, codePage})
		if length != uintptr(len(want)) || codePage != 65001 {
			return 1
		}
		for index := 0; index < len(want); index++ {
			value, ok := sessionScintillaValue(hwnd, 2007, uintptr(index)) // SCI_GETCHARAT
			if !ok || byte(value) != want[index] {
				return 1
			}
		}
		matches = append(matches, hwnd)
		return 1
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		select {
		case <-editor.done:
			t.Fatalf("Notepad++ exited before exposing its session buffer: %v", editor.err)
		default:
		}
		views, matches = views[:0], matches[:0]
		enumChildren.Call(editor.hwnd, find, 0)
		if len(matches) == 1 {
			t.Logf("reference UTF-8 session buffer matched in one visible Scintilla: %+v", views)
			return matches[0]
		}
	}
	t.Fatalf("wanted one visible Scintilla with the exact UTF-8 fixture, found %d; observed views: %+v", len(matches), views)
	return 0
}

func sessionScintillaValue(hwnd, message, index uintptr) (uintptr, bool) {
	var value uintptr
	ok, _, _ := sendMessageTimeout.Call(hwnd, message, index, 0, 2, 250, uintptr(unsafe.Pointer(&value)))
	return value, ok != 0
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
