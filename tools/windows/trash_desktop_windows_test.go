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
	helpClick(t, editor, "Move to Trash", command)
	helpWait(t, editor, "recycled tab closed", func(nodes []helpNode) bool {
		_, stillOpen := helpFind(nodes, tab)
		return !stillOpen
	})
	windows.CheckRecycledFixture(t, path, []byte(saved))
	editor.close(t)
}
