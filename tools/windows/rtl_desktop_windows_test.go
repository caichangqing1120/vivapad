package windows_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	enumWindows         = user32.NewProc("EnumWindows")
	windowPID           = user32.NewProc("GetWindowThreadProcessId")
	isWindowVisible     = user32.NewProc("IsWindowVisible")
	getWindowText       = user32.NewProc("GetWindowTextW")
	getClassName        = user32.NewProc("GetClassNameW")
	getGUIThreadInfo    = user32.NewProc("GetGUIThreadInfo")
	getWindowLong       = user32.NewProc("GetWindowLongW")
	sendMessageTimeout  = user32.NewProc("SendMessageTimeoutW")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")
	getClientRect       = user32.NewProc("GetClientRect")
	clientToScreen      = user32.NewProc("ClientToScreen")
	setCursorPos        = user32.NewProc("SetCursorPos")
	mouseEvent          = user32.NewProc("mouse_event")
	keyboardEvent       = user32.NewProc("keybd_event")
	mapVirtualKey       = user32.NewProc("MapVirtualKeyW")
	postMessage         = user32.NewProc("PostMessageW")
)

const (
	keyLeftControl = 0xa2
	keyLeftAlt     = 0xa4
)

type desktopEditor struct {
	hwnd uintptr
	done chan struct{}
	err  error
}

func TestRTLDesktopMatchesNotepadPlusPlus(t *testing.T) {
	app, reference := os.Getenv("VIVAPAD_RTL_DESKTOP_EXE"), os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if app == "" || reference == "" {
		t.Skip("requires the built VivaPad and pinned portable Notepad++ on an isolated Windows desktop")
	}
	const original = "\u05d0\u05d1\u05d2\r\nabc\r\n"
	const afterRTL = "\u05d0x\u05d1\u05d2\r\nabc\r\n"
	const afterLTR = "\u05d0x\u05d1\u05d2\r\naybc\r\n"
	profile := t.TempDir()
	appData := filepath.Join(profile, "appdata")
	if err := os.Mkdir(appData, 0700); err != nil {
		t.Fatal(err)
	}
	appEnv := append(os.Environ(), "APPDATA="+appData, "LOCALAPPDATA="+appData)
	for _, target := range []struct {
		name, path, title string
		args, env         []string
	}{
		{"NotepadPlusPlus", reference, "Notepad++", []string{"-multiInst", "-nosession", "-noPlugin"}, nil},
		{"VivaPad", app, "VivaPad", nil, appEnv},
	} {
		t.Run(target.name, func(t *testing.T) {
			path := filepath.Join(profile, target.name+"-fixture.txt")
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string(nil), target.args...), path)
			editor := openDesktopEditor(t, target.path, target.title, target.env, args...)
			editor.focusText(t)
			editor.inspect(t, "focused", target.env == nil, false)
			editor.keys(t, keyLeftControl, 0x24) // Ctrl+Home
			editor.keys(t, keyLeftControl, keyLeftAlt, 'R')
			editor.inspect(t, "Ctrl+Alt+R", target.env == nil, true)
			editor.keys(t, 0x25) // Left
			editor.inspect(t, "Left", target.env == nil, true)
			editor.keys(t, 'X')
			editor.inspect(t, "type x", target.env == nil, true)
			editor.keys(t, keyLeftControl, 'S')
			awaitBytes(t, path, afterRTL)
			editor.close(t)

			if target.env != nil {
				checkSessionRTL(t, appData, path, true)
				args = nil // VivaPad must reopen the document from its session.
			}
			editor = openDesktopEditor(t, target.path, target.title, target.env, args...)
			editor.focusText(t)
			editor.keys(t, keyLeftControl, keyLeftAlt, 'L')
			editor.inspect(t, "Ctrl+Alt+L", target.env == nil, false)
			editor.keys(t, keyLeftControl, 0x23) // Ctrl+End (empty last line)
			editor.keys(t, 0x26)                 // Up
			editor.keys(t, 0x24)                 // Home (start of abc)
			editor.keys(t, 0x27)                 // Right
			editor.keys(t, 'Y')
			editor.keys(t, keyLeftControl, 'S')
			awaitBytes(t, path, afterLTR)
			editor.close(t)
			if target.env != nil {
				checkSessionRTL(t, appData, path, false)
			}
		})
	}
}

func checkSessionRTL(t *testing.T, appData, file string, want bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(appData, "VivaPad", "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	var session struct {
		RTL map[string]bool `json:"rtl"`
	}
	if err := json.Unmarshal(data, &session); err != nil {
		t.Fatal(err)
	}
	got := false
	file, err = filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	for path, rtl := range session.RTL {
		if strings.EqualFold(filepath.Clean(path), file) {
			got = rtl
		}
	}
	if got != want {
		t.Fatalf("session RTL = %v, want %v for %q; recorded flags: %v", got, want, file, session.RTL)
	}
}

func awaitBytes(t *testing.T, path, want string) {
	t.Helper()
	var value []byte
	var err error
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		value, err = os.ReadFile(path)
		if err == nil && string(value) == want {
			return
		}
	}
	t.Fatalf("saved %q, want %q (read error: %v)", value, want, err)
}

func openDesktopEditor(t *testing.T, path, title string, env []string, args ...string) *desktopEditor {
	t.Helper()
	command := exec.Command(path, args...)
	command.Dir, command.Env = filepath.Dir(path), env
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	editor := &desktopEditor{done: make(chan struct{})}
	go func() {
		editor.err = command.Wait()
		close(editor.done)
	}()
	t.Cleanup(func() {
		select {
		case <-editor.done:
		default:
			_ = command.Process.Kill()
			<-editor.done
		}
	})
	var hwnd uintptr
	find := syscall.NewCallback(func(candidate, _ uintptr) uintptr {
		var pid uint32
		windowPID.Call(candidate, uintptr(unsafe.Pointer(&pid)))
		if int(pid) != command.Process.Pid {
			return 1
		}
		visible, _, _ := isWindowVisible.Call(candidate)
		if visible != 0 {
			var text [256]uint16
			getWindowText.Call(candidate, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
			if strings.Contains(syscall.UTF16ToString(text[:]), title) {
				hwnd = candidate
				return 0
			}
		}
		return 1
	})
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		select {
		case <-editor.done:
			t.Fatalf("%s exited before opening its window: %v", title, editor.err)
		default:
		}
		enumWindows.Call(find, 0)
		if hwnd != 0 {
			editor.hwnd = hwnd
			editor.activate(t)
			return editor
		}
	}
	t.Fatalf("%s did not open an interactive window", title)
	return nil
}

func (e *desktopEditor) activate(t *testing.T) {
	t.Helper()
	for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); time.Sleep(40 * time.Millisecond) {
		if current, _, _ := getForegroundWindow.Call(); current == e.hwnd {
			return
		}
		setForegroundWindow.Call(e.hwnd)
	}
	t.Fatal("the tested editor did not own the foreground window")
}

func (e *desktopEditor) focusText(t *testing.T) {
	t.Helper()
	e.activate(t)
	var rect [4]int32
	if ok, _, _ := getClientRect.Call(e.hwnd, uintptr(unsafe.Pointer(&rect[0]))); ok == 0 {
		t.Fatal("editor client area unavailable")
	}
	point := [2]int32{rect[2] / 3, rect[3] / 2}
	if ok, _, _ := clientToScreen.Call(e.hwnd, uintptr(unsafe.Pointer(&point[0]))); ok == 0 {
		t.Fatal("editor client area is not on the Windows desktop")
	}
	if ok, _, _ := setCursorPos.Call(uintptr(point[0]), uintptr(point[1])); ok == 0 {
		t.Fatal("could not place the pointer in the editor")
	}
	mouseEvent.Call(2, 0, 0, 0, 0) // MOUSEEVENTF_LEFTDOWN
	mouseEvent.Call(4, 0, 0, 0, 0) // MOUSEEVENTF_LEFTUP
	time.Sleep(150 * time.Millisecond)
}

func (e *desktopEditor) keys(t *testing.T, chord ...uintptr) {
	t.Helper()
	e.activate(t)
	press := func(key, flags uintptr) {
		scan, _, _ := mapVirtualKey.Call(key, 0)
		if key >= 0x23 && key <= 0x28 { // navigation keys use extended scan codes
			flags |= 1
		}
		keyboardEvent.Call(key, scan, flags, 0)
		time.Sleep(35 * time.Millisecond)
	}
	for _, key := range chord {
		press(key, 0)
	}
	for i := len(chord) - 1; i >= 0; i-- {
		press(chord[i], 2) // KEYEVENTF_KEYUP
	}
	time.Sleep(100 * time.Millisecond)
}

func (e *desktopEditor) inspect(t *testing.T, stage string, reference, rtl bool) {
	t.Helper()
	info := struct {
		size, flags                                   uint32
		active, focus, capture, menu, moveSize, caret uintptr
		caretRect                                     [4]int32
	}{}
	info.size = uint32(unsafe.Sizeof(info))
	thread, _, _ := windowPID.Call(e.hwnd, 0)
	if ok, _, _ := getGUIThreadInfo.Call(thread, uintptr(unsafe.Pointer(&info))); ok == 0 || info.focus == 0 {
		t.Fatalf("%s: could not read the editor's focused control", stage)
	}
	var name [128]uint16
	getClassName.Call(info.focus, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	class := syscall.UTF16ToString(name[:])
	if !reference {
		t.Logf("%s: focused control is %q", stage, class)
		return
	}
	if class != "Scintilla" {
		t.Fatalf("%s: Notepad++ focus is %q, want Scintilla", stage, class)
	}
	read := func(message uintptr) uintptr {
		var result uintptr
		if ok, _, _ := sendMessageTimeout.Call(info.focus, message, 0, 0, 2, 500, uintptr(unsafe.Pointer(&result))); ok == 0 {
			t.Fatalf("%s: Scintilla message %d timed out", stage, message)
		}
		return result
	}
	style, _, _ := getWindowLong.Call(info.focus, uintptr(^uint32(19))) // GWL_EXSTYLE (-20)
	actual := style&0x00400000 != 0                                     // WS_EX_LAYOUTRTL
	t.Logf("%s: Scintilla length=%d caret=%d read-only=%t RTL=%t", stage, read(2006), read(2008), read(2140) != 0, actual)
	if actual != rtl {
		t.Fatalf("%s: Scintilla RTL=%t, want %t", stage, actual, rtl)
	}
}

func (e *desktopEditor) close(t *testing.T) {
	t.Helper()
	postMessage.Call(e.hwnd, 0x10, 0, 0) // WM_CLOSE
	select {
	case <-e.done:
		if e.err != nil {
			t.Fatal(e.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("editor did not close cleanly")
	}
}
