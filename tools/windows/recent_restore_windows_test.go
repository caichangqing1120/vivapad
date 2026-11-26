package windows_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"vivapad/internal/recent"
)

func TestWindowsRecentClosedFileRestore(t *testing.T) {
	reference, binary := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE"), os.Getenv("VIVAPAD_RTL_DESKTOP_EXE")
	if reference == "" && binary == "" {
		t.Skip("requires VivaPad and pinned portable Notepad++ on an isolated Windows desktop")
	}
	if reference == "" || binary == "" {
		t.Fatal("both Windows desktop executables are required")
	}
	const original = "alpha\r\nbeta\r\n"
	const edited = "x" + original
	for _, tc := range []struct {
		name, executable, title string
		width, height           int32
	}{
		{"NotepadPlusPlus", reference, "Notepad++", 0, 0},
		{"VivaPadCompact", binary, "VivaPad", 640, 400},
		{"VivaPadDesktop", binary, "VivaPad", 960, 600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "recent-source.txt")
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			var args, env []string
			var history string
			if tc.width == 0 {
				args = []string{"-multiInst", "-nosession", "-noPlugin"}
			} else {
				profile := filepath.Join(root, "appdata")
				if err := os.Mkdir(profile, 0700); err != nil {
					t.Fatal(err)
				}
				env = append(os.Environ(), "APPDATA="+profile, "LOCALAPPDATA="+profile)
				history = filepath.Join(profile, "VivaPad", "closed.json")
			}
			editor := openDesktopEditor(t, tc.executable, tc.title, env, append(args, path)...)
			if tc.width != 0 {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				dpiContext := user32.NewProc("SetThreadDpiAwarenessContext")
				prior, _, err := dpiContext.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2
				if prior == 0 {
					t.Fatalf("use physical desktop coordinates: %v", err)
				}
				defer dpiContext.Call(prior)
				helpResize(t, editor, tc.width, tc.height)
			}
			editor.focusText(t)
			editor.keys(t, keyLeftControl, 0x24) // Ctrl+Home
			if history == "" && focusedScintilla(t, editor)(2006, 0, 0) != uintptr(len(original)) {
				t.Fatal("the saved fixture is not active")
			}
			editor.keys(t, keyLeftControl, 'W')
			if history == "" {
				if focusedScintilla(t, editor)(2006, 0, 0) != 0 {
					t.Fatal("closing the last saved file did not leave an empty tab")
				}
			} else {
				awaitWindowsClosedHistory(t, history, path)
			}
			editor.keys(t, keyLeftControl, 0xa0, 'T') // Ctrl+Left Shift+T
			if history == "" {
				if focusedScintilla(t, editor)(2006, 0, 0) != uintptr(len(original)) {
					t.Fatal("Ctrl+Shift+T did not restore the fixture")
				}
			} else {
				awaitWindowsClosedHistory(t, history, "")
			}
			awaitBytes(t, path, original)
			editor.keys(t, 'X') // No click or focus key after Restore.
			editor.keys(t, keyLeftControl, 'S')
			awaitBytes(t, path, edited)
			editor.keys(t, keyLeftControl, 'Z')
			editor.keys(t, keyLeftControl, 'S')
			awaitBytes(t, path, original)
			editor.close(t)
		})
	}
}

func awaitWindowsClosedHistory(t *testing.T, history, want string) {
	t.Helper()
	var entries []recent.Entry
	var err error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		entries, err = recent.Load(history)
		if err == nil && (want == "" && len(entries) == 0 || want != "" && len(entries) == 1 && entries[0].Path == want) {
			return
		}
	}
	t.Fatalf("closed history = %+v (%v), want %q", entries, err, want)
}
