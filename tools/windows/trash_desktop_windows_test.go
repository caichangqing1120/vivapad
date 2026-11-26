package windows_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"vivapad/tools/windows"
)

func TestVivaPadWindowsDesktopTrashCancelAndConfirm(t *testing.T) {
	const original = "alpha\r\nbeta\r\n"
	const saved = "x" + original
	path := windows.HostedRecycleFixture(t, []byte(original))
	binary := os.Getenv("VIVAPAD_RTL_DESKTOP_EXE")
	if binary == "" {
		t.Fatal("requires the built VivaPad Windows executable")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
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
	const command = "Move to Recycle Bin"
	const tabPrefix = "Close "
	tab := tabPrefix + filepath.Base(path)
	helpWait(t, editor, "fixture tab", func(nodes []helpNode) bool {
		_, found := helpFind(nodes, tab)
		return found
	})
	open := func() {
		t.Helper()
		helpClick(t, editor, "File")
		helpClick(t, editor, command)
		helpWait(t, editor, "Recycle Bin confirmation", func(nodes []helpNode) bool {
			_, cancel := helpFind(nodes, "Cancel")
			_, confirm := helpFind(nodes, "Move to Trash")
			return cancel && confirm
		})
	}
	helpClick(t, editor, "Text editor")
	editor.keys(t, keyLeftControl, 0x24) // Ctrl+Home
	editor.keys(t, 'X')
	open()
	helpClick(t, editor, "Cancel", command)
	helpWait(t, editor, "cancelled confirmation", func(nodes []helpNode) bool {
		_, confirm := helpFind(nodes, "Move to Trash")
		_, stillOpen := helpFind(nodes, tab)
		return !confirm && stillOpen
	})
	awaitBytes(t, path, original)
	editor.keys(t, keyLeftControl, 'S')
	awaitBytes(t, path, saved)

	helpClick(t, editor, "Text editor")
	editor.keys(t, 'Y')
	open()
	backup := filepath.Join(filepath.Dir(path), "original-before-replacement")
	// Keep the original file alive while another file takes its path.
	originalInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, backup); err != nil {
		t.Fatalf("rename the task-owned source while confirmation is open: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(backup) })
	renamed, err := os.Stat(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(saved), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, originalInfo.ModTime(), originalInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(renamed, replacement) || renamed.Size() != replacement.Size() || !renamed.ModTime().Equal(replacement.ModTime()) {
		t.Fatal("replacement must be a different file with matching size and timestamp")
	}
	helpClick(t, editor, "Move to Trash", command)
	helpWait(t, editor, "replacement refused", func(nodes []helpNode) bool {
		_, confirm := helpFind(nodes, "Move to Trash")
		_, stillOpen := helpFind(nodes, tab)
		return !confirm && stillOpen
	})
	for _, fixture := range []struct {
		path string
		info os.FileInfo
	}{{backup, renamed}, {path, replacement}} {
		current, err := os.Stat(fixture.path)
		if err != nil || !os.SameFile(fixture.info, current) {
			t.Fatalf("refusal moved or replaced its fixture: %v", err)
		}
		awaitBytes(t, fixture.path, saved)
	}
	// Restore the original object and start a fresh confirmation for the valid move.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, path); err != nil {
		t.Fatal(err)
	}
	t.Log("confirmation refused a same-content replacement; both file identities survived")
	open()
	helpClick(t, editor, "Move to Trash", command)
	helpWait(t, editor, "recycled tab closed", func(nodes []helpNode) bool {
		_, stillOpen := helpFind(nodes, tab)
		return !stillOpen
	})
	windows.CheckRecycledFixture(t, path, []byte(saved))
	editor.close(t)
}
