package external

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	result := filepath.Join(dir, "probe-cwd.txt")
	runConsoleProbe(t, exe, dir, result, "-test.run=^TestWindowsConsoleProbeChild$")
	got, err := os.ReadFile(result)
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
		name string
		kind ConsoleKind
		file string
		args []string
	}{
		{"cmd", CommandPrompt, "cmd-cwd.txt", []string{"/d", "/c", "cd>cmd-cwd.txt"}},
		{"PowerShell", WindowsPowerShell, "ps-cwd.txt", []string{"-NoProfile", "-NonInteractive", "-Command", "[IO.File]::WriteAllText('ps-cwd.txt',[Environment]::CurrentDirectory)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := windowsConsoleProgram(tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			result := filepath.Join(dir, tc.file)
			runConsoleProbe(t, program, dir, result, tc.args...)
			got, err := os.ReadFile(result)
			if err != nil {
				t.Fatal(err)
			}
			requireSameConsoleDirectory(t, strings.TrimSpace(string(got)), dir)
		})
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

func runConsoleProbe(t *testing.T, program, dir, result string, args ...string) {
	t.Helper()
	started := time.Now()
	process, err := startWindowsConsole(program, dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process)
	t.Logf("%s started in %s", filepath.Base(program), time.Since(started).Round(time.Millisecond))
	status, err := windows.WaitForSingleObject(process, 15000)
	elapsed := time.Since(started).Round(time.Millisecond)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		info, resultErr := os.Stat(result)
		if resultErr == nil {
			t.Logf("console result file exists (%d bytes)", info.Size())
		} else {
			t.Logf("console result file unavailable: %v", resultErr)
		}
		if terminateErr := windows.TerminateProcess(process, 1); terminateErr != nil {
			t.Logf("terminate console probe: %v", terminateErr)
		} else if cleanupStatus, cleanupErr := windows.WaitForSingleObject(process, 5000); cleanupErr != nil || cleanupStatus != windows.WAIT_OBJECT_0 {
			t.Logf("wait after ending console probe = %#x, %v", cleanupStatus, cleanupErr)
		}
		t.Fatalf("console process wait after %s = %#x, %v", elapsed, status, err)
	}
	t.Logf("%s exited after %s", filepath.Base(program), elapsed)
	var code uint32
	if err := windows.GetExitCodeProcess(process, &code); err != nil || code != 0 {
		t.Fatalf("console process exit = %d, %v", code, err)
	}
}
