package windows_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

func TestVivaPadWindowsDesktopReadOnlyModes(t *testing.T) {
	if os.Getenv("VIVAPAD_TEST_WINDOWS_FILE_ATTRIBUTE") != "1" {
		t.Skip("requires the hosted Windows desktop")
	}
	runnerTemp := os.Getenv("RUNNER_TEMP")
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || !filepath.IsAbs(runnerTemp) {
		t.Fatal("requires the GitHub-hosted Windows temporary directory")
	}
	binary := os.Getenv("VIVAPAD_RTL_DESKTOP_EXE")
	if !filepath.IsAbs(binary) {
		t.Fatal("requires an absolute path to the built VivaPad executable")
	}
	root, err := os.MkdirTemp(runnerTemp, "vivapad-readonly-")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "fixture.txt")
	t.Cleanup(func() {
		_ = os.Chmod(path, 0600)
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove owned fixture: %v", err)
		}
	})
	const original = "alpha\r\nbeta\r\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	nativePath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attributes := func() uint32 {
		t.Helper()
		value, err := windows.GetFileAttributes(nativePath)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	initial := attributes()
	const readOnly = windows.FILE_ATTRIBUTE_READONLY
	if initial&readOnly != 0 {
		t.Fatal("fixture unexpectedly starts read-only")
	}
	profile := t.TempDir()
	editor := openDesktopEditor(t, binary, "VivaPad", append(os.Environ(),
		"APPDATA="+profile, "LOCALAPPDATA="+profile, "VIVA_ACCESSIBILITY=1"), path)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dpiContext := user32.NewProc("SetThreadDpiAwarenessContext")
	previous, _, err := dpiContext.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2
	if previous == 0 {
		t.Fatalf("use physical desktop coordinates: %v", err)
	}
	defer dpiContext.Call(previous)
	helpResize(t, editor, 960, 640)
	const attributeCommand = "Read-Only Attribute in Windows"
	const editorModeCommand = "Read-Only on Current Document"
	toggle := func(command string) {
		t.Helper()
		helpClick(t, editor, "Edit")
		if command == editorModeCommand {
			helpClick(t, editor, "Read-Only in Notepad++")
		}
		helpClick(t, editor, command)
		helpWait(t, editor, "read-only menu dismissed", func(nodes []helpNode) bool {
			_, stillOpen := helpFind(nodes, command)
			return !stillOpen
		})
	}
	typeAtStart := func(key uintptr) {
		t.Helper()
		helpClick(t, editor, "Text editor")
		editor.keys(t, keyLeftControl, 0x24) // Ctrl+Home
		editor.keys(t, key)
	}
	toggle(attributeCommand)
	if got := attributes(); got != initial|readOnly {
		t.Fatalf("setting Windows read-only: attributes %#x, want %#x", got, initial|readOnly)
	}
	typeAtStart('X')
	toggle(editorModeCommand)
	if got := attributes(); got != initial|readOnly {
		t.Fatalf("setting editor read-only changed attributes: %#x", got)
	}
	toggle(attributeCommand)
	if got := attributes(); got != initial {
		t.Fatalf("clearing Windows read-only: attributes %#x, want %#x", got, initial)
	}
	typeAtStart('Z')
	toggle(editorModeCommand)
	if got := attributes(); got != initial {
		t.Fatalf("clearing editor read-only changed attributes: %#x", got)
	}
	editor.keys(t, keyLeftControl, 'S')
	awaitBytes(t, path, original)
	typeAtStart('Y')
	editor.keys(t, keyLeftControl, 'S')
	awaitBytes(t, path, "y"+original)
	editor.close(t)
}
