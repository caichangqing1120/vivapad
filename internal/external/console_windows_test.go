package external

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsConsoleProcessUsesFreshConsoleAndLiteralDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir & 100% (\u6d4b\u8bd5)")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIVAPAD_TEST_CONSOLE_PROBE", "1")
	runConsoleProbe(t, exe, dir, "-test.run=^TestWindowsConsoleProbeChild$")
	got, err := os.ReadFile(filepath.Join(dir, "probe-cwd.txt"))
	if err != nil {
		t.Fatal(err)
	}
	requireSameConsoleDirectory(t, string(got), dir)
}

func TestWindowsConsoleProbeChild(t *testing.T) {
	if os.Getenv("VIVAPAD_TEST_CONSOLE_PROBE") != "1" {
		t.Skip("helper process")
	}
	for _, handle := range []uint32{windows.STD_INPUT_HANDLE, windows.STD_OUTPUT_HANDLE, windows.STD_ERROR_HANDLE} {
		value, err := windows.GetStdHandle(handle)
		if err != nil {
			t.Fatal(err)
		}
		var mode uint32
		if err := windows.GetConsoleMode(value, &mode); err != nil {
			t.Fatalf("standard handle %#x is not a console: %v", handle, err)
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("probe-cwd.txt", []byte(dir), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsConfiguredShellsStartInContainingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir & 100% (probe)")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		kind ConsoleKind
		file string
		args []string
	}{
		{CommandPrompt, "cmd-cwd.txt", []string{"/d", "/c", "cd>cmd-cwd.txt"}},
		{WindowsPowerShell, "ps-cwd.txt", []string{"-NoProfile", "-NonInteractive", "-Command", "[IO.File]::WriteAllText('ps-cwd.txt',[Environment]::CurrentDirectory)"}},
	} {
		program, err := windowsConsoleProgram(tc.kind)
		if err != nil {
			t.Fatal(err)
		}
		runConsoleProbe(t, program, dir, tc.args...)
		got, err := os.ReadFile(filepath.Join(dir, tc.file))
		if err != nil {
			t.Fatal(err)
		}
		requireSameConsoleDirectory(t, strings.TrimSpace(string(got)), dir)
	}
}

func requireSameConsoleDirectory(t *testing.T, reported, expected string) {
	t.Helper()
	got, err := os.Stat(reported)
	if err != nil {
		t.Fatalf("console reported directory %q: %v", reported, err)
	}
	want, err := os.Stat(expected)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsDir() || !os.SameFile(got, want) {
		t.Fatalf("console working directory = %q, want the directory at %q", reported, expected)
	}
}

func runConsoleProbe(t *testing.T, program, dir string, args ...string) {
	t.Helper()
	process, err := startWindowsConsole(program, dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process)
	status, err := windows.WaitForSingleObject(process, 15000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		_ = windows.TerminateProcess(process, 1)
		t.Fatalf("console process wait = %#x, %v", status, err)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(process, &code); err != nil || code != 0 {
		t.Fatalf("console process exit = %d, %v", code, err)
	}
}
