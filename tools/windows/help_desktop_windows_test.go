package windows_test

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/app"
	"vivapad/tools/windows"
)

func TestVivaPadWindowsHelpDialogs(t *testing.T) {
	if os.Getenv("VIVAPAD_WINDOWS_HELP_TEST") != "1" {
		t.Skip("requires an isolated Windows desktop and clipboard")
	}
	binary := os.Getenv("VIVAPAD_RTL_DESKTOP_EXE")
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	var viva string
	for _, dep := range info.Deps {
		if dep.Path == "github.com/vivaui/viva" && dep.Replace != nil {
			viva = dep.Replace.Version
		}
	}
	if viva == "" {
		t.Fatal("desktop executable does not identify its published Viva dependency")
	}

	for _, viewport := range []struct {
		name          string
		width, height int32
		compact       bool
	}{{"compact", 640, 400, true}, {"desktop", 960, 600, false}} {
		t.Run(viewport.name, func(t *testing.T) {
			const disk = "\ufeffalpha beta\r\n"
			const edited = "\ufeffx beta\r\n"
			const sentinel = "alpha"
			profile := t.TempDir()
			path := filepath.Join(profile, "private-help-fixture.txt")
			if err := os.WriteFile(path, []byte(disk), 0600); err != nil {
				t.Fatal(err)
			}
			data := filepath.Join(profile, "appdata")
			if err := os.Mkdir(data, 0700); err != nil {
				t.Fatal(err)
			}
			editor := openDesktopEditor(t, binary, "VivaPad", append(os.Environ(),
				"APPDATA="+data, "LOCALAPPDATA="+data, "VIVA_ACCESSIBILITY=1"), path)
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			dpiContext := user32.NewProc("SetThreadDpiAwarenessContext")
			previous, _, err := dpiContext.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2
			if previous == 0 {
				t.Fatalf("use physical desktop coordinates: %v", err)
			}
			defer dpiContext.Call(previous)
			helpResize(t, editor, viewport.width, viewport.height)
			helpClick(t, editor, "Text editor")
			selectWord := func() {
				editor.keys(t, keyLeftControl, 0x24) // Ctrl+Home
				for range len(sentinel) {
					editor.keys(t, 0xa0, 0x27) // Left Shift+Right
				}
			}
			selectWord()
			editor.keys(t, keyLeftControl, 'C')
			helpClipboard(t, sentinel)
			sequence := windows.ClipboardSequence()

			open := func(command, dialog string, prefixes ...string) []string {
				t.Helper()
				selectWord()
				if viewport.compact {
					var overflow bool
					helpWait(t, editor, "compact Help entry", func(nodes []helpNode) bool {
						_, overflow = helpFind(nodes, "More menus")
						_, direct := helpFind(nodes, "?")
						return overflow || direct
					})
					if overflow {
						helpClick(t, editor, "More menus")
					}
				}
				helpClick(t, editor, "?")
				helpClick(t, editor, command)
				var rows []string
				helpWait(t, editor, dialog+" rows", func(nodes []helpNode) bool {
					if _, found := helpFind(nodes, dialog); !found {
						return false
					}
					rows = rows[:0]
					for _, prefix := range prefixes {
						row, found := helpFind(nodes, prefix)
						if !found {
							return false
						}
						rows = append(rows, row.Name)
					}
					return true
				})
				if got, err := app.ClipboardText(); err != nil || got != sentinel || windows.ClipboardSequence() != sequence {
					t.Fatalf("opening %s changed the clipboard: %q (%v)", dialog, got, err)
				}
				return rows
			}
			checkEdit := func(dialog string) {
				t.Helper()
				helpWait(t, editor, dialog+" dismissed", func(nodes []helpNode) bool {
					_, found := helpFind(nodes, dialog)
					return !found
				})
				awaitBytes(t, path, disk)
				editor.keys(t, 'X')
				editor.keys(t, keyLeftControl, 'S')
				awaitBytes(t, path, edited)
				editor.keys(t, keyLeftControl, 'Z')
				editor.keys(t, keyLeftControl, 'S')
				awaitBytes(t, path, disk)
			}

			about := open("About VivaPad", "About VivaPad", "Build: ", "Revision: ", "Platform: ")
			if about[2] != "Platform: windows/"+runtime.GOARCH {
				t.Fatalf("About platform = %q", about[2])
			}
			helpClick(t, editor, "Close", "About VivaPad")
			checkEdit("About VivaPad")

			debug := open("Debug Info...", "Debug Info", "Build: ", "Revision: ", "Platform: ", "Go: ", "Viva: ")
			if strings.Join(debug[:3], "\n") != strings.Join(about, "\n") || debug[3] != "Go: "+info.GoVersion || debug[4] != "Viva: "+viva {
				t.Fatalf("Debug Info does not report the executable's build: %v", debug)
			}
			helpClick(t, editor, "Copy", "Debug Info")
			want := "VivaPad\n" + strings.Join(debug, "\n")
			helpClipboard(t, want)
			if windows.ClipboardSequence() == sequence || strings.Contains(want, profile) || strings.Contains(want, "alpha beta") {
				t.Fatalf("diagnostic report was not published or contains fixture data: %q", want)
			}
			editor.keys(t, 0x1b) // Escape
			checkEdit("Debug Info")
			editor.close(t)
		})
	}
}

func helpClipboard(t *testing.T, want string) {
	t.Helper()
	var got string
	var issue error
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		got, issue = app.ClipboardText()
		if issue == nil && got == want {
			return
		}
	}
	t.Fatalf("clipboard = %q, want %q (%v)", got, want, issue)
}

type helpNode struct {
	Name                     string
	Left, Top, Right, Bottom float64
}

func helpFind(nodes []helpNode, name string) (helpNode, bool) {
	for _, node := range nodes {
		if node.Name == name || strings.HasSuffix(name, ": ") && strings.HasPrefix(node.Name, name) {
			return node, true
		}
	}
	return helpNode{}, false
}

func helpWait(t *testing.T, editor *desktopEditor, purpose string, accept func([]helpNode) bool) {
	t.Helper()
	var last []helpNode
	var issue error
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		output, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", "help_snapshot.ps1", strconv.FormatUint(uint64(editor.hwnd), 10)).CombinedOutput()
		cancel()
		if err != nil {
			issue = fmt.Errorf("UI Automation: %w: %s", err, output)
			continue
		}
		var nodes []helpNode
		if issue = json.Unmarshal(output, &nodes); issue != nil {
			continue
		}
		last = nodes
		if accept(nodes) {
			return
		}
	}
	var names []string
	for _, node := range last {
		names = append(names, node.Name)
	}
	t.Fatalf("waiting for %s: last observation %v; error %v", purpose, names, issue)
}

func helpClick(t *testing.T, editor *desktopEditor, name string, dialog ...string) {
	t.Helper()
	var x, y float64
	helpWait(t, editor, name, func(nodes []helpNode) bool {
		origin, end := helpClient(t, editor)
		area := helpNode{Left: float64(origin[0]), Top: float64(origin[1]), Right: float64(end[0]), Bottom: float64(end[1])}
		if len(dialog) != 0 {
			var found bool
			area, found = helpFind(nodes, dialog[0])
			if !found {
				return false
			}
		}
		for _, node := range nodes {
			cx, cy := (node.Left+node.Right)/2, (node.Top+node.Bottom)/2
			if node.Name == name && cx >= area.Left && cx < area.Right && cy >= area.Top && cy < area.Bottom {
				x, y = cx, cy
				return true
			}
		}
		return false
	})
	editor.activate(t)
	if ok, _, err := setCursorPos.Call(uintptr(int32(x)), uintptr(int32(y))); ok == 0 {
		t.Fatal(err)
	}
	mouseEvent.Call(2, 0, 0, 0, 0)
	mouseEvent.Call(4, 0, 0, 0, 0)
}

func helpClient(t *testing.T, editor *desktopEditor) (origin, end [2]int32) {
	t.Helper()
	var rect [4]int32
	if ok, _, err := getClientRect.Call(editor.hwnd, uintptr(unsafe.Pointer(&rect[0]))); ok == 0 {
		t.Fatal(err)
	}
	if ok, _, err := clientToScreen.Call(editor.hwnd, uintptr(unsafe.Pointer(&origin[0]))); ok == 0 {
		t.Fatal(err)
	}
	return origin, [2]int32{origin[0] + rect[2], origin[1] + rect[3]}
}

func helpResize(t *testing.T, editor *desktopEditor, width, height int32) {
	t.Helper()
	dpi, _, err := user32.NewProc("GetDpiForWindow").Call(editor.hwnd)
	if dpi == 0 {
		t.Fatal(err)
	}
	width, height = (width*int32(dpi)+48)/96, (height*int32(dpi)+48)/96
	var frame [4]int32
	if ok, _, err := user32.NewProc("GetWindowRect").Call(editor.hwnd, uintptr(unsafe.Pointer(&frame[0]))); ok == 0 {
		t.Fatal(err)
	}
	origin, end := helpClient(t, editor)
	outerWidth := frame[2] - frame[0] - (end[0] - origin[0]) + width
	outerHeight := frame[3] - frame[1] - (end[1] - origin[1]) + height
	if ok, _, err := user32.NewProc("SetWindowPos").Call(editor.hwnd, 0, 20, 20, uintptr(outerWidth), uintptr(outerHeight), 0x14); ok == 0 { // NOZORDER | NOACTIVATE
		t.Fatal(err)
	}
	origin, end = helpClient(t, editor)
	if end[0]-origin[0] != width || end[1]-origin[1] != height {
		t.Fatalf("client = %dx%d, want %dx%d", end[0]-origin[0], end[1]-origin[1], width, height)
	}
}
