//go:build windows

package external

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsTrashRequiresRecycleOnDelete(t *testing.T) {
	want := trashGUID{0x947aab5f, 0x0a5c, 0x4c13, [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8}}
	if fileOperationInterface != want {
		t.Fatalf("IFileOperation IID = %+v, want %+v", fileOperationInterface, want)
	}
	if recycleOperationFlags&0x00080000 == 0 {
		t.Fatal("IFileOperation must request Recycle Bin rather than permanent deletion")
	}
	if recycleOperationFlags&0x00000400 == 0 {
		t.Fatal("IFileOperation must report errors to the application")
	}
}

func TestWindowsNativeTrashRecyclesHostedRunnerFixture(t *testing.T) {
	if os.Getenv("VIVAPAD_TEST_WINDOWS_RECYCLE_BIN") != "1" {
		t.Skip("native Recycle Bin check is enabled only in hosted Windows CI")
	}
	runnerTemp := os.Getenv("RUNNER_TEMP")
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || !filepath.IsAbs(runnerTemp) {
		t.Fatal("native Recycle Bin check requires a GitHub-hosted runner and its own temporary directory")
	}
	root, err := os.MkdirTemp(runnerTemp, "vivapad-recycle-")
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(root) // Extensionless so Shell's extension visibility setting cannot change it.
	path := filepath.Join(root, name)
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("clean fixture source: %v", err)
		}
		if err := os.Remove(root); err != nil {
			t.Errorf("clean fixture directory: %v", err)
		}
	})
	payload := []byte("VivaPad native recycle fixture: \x00\xff\r\n")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Trash(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source did not leave its original path: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	// The shell namespace is the receipt; a missing source alone also happens
	// with permanent deletion. Read only the uniquely named runner fixture.
	cmd := exec.CommandContext(ctx, "pwsh.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `
$ErrorActionPreference = 'Stop'
$shell = New-Object -ComObject Shell.Application
$bin = $shell.NameSpace(10)
if ($null -eq $bin) { throw 'Windows Shell could not open the Recycle Bin namespace' }
$clock = [System.Diagnostics.Stopwatch]::StartNew()
do {
  foreach ($item in $bin.Items()) {
    if ($item.Name -cne $env:VIVAPAD_RECYCLE_NAME) { continue }
    $original = $item.ExtendedProperty('System.Recycle.DeletedFrom')
    if ($original -ine $env:VIVAPAD_RECYCLE_PARENT) { continue }
    $bytes = [IO.File]::ReadAllBytes($item.Path)
    if ([Convert]::ToBase64String($bytes) -cne $env:VIVAPAD_RECYCLE_PAYLOAD) {
      throw 'Recycled fixture bytes differ from the original'
    }
    exit 0
  }
  Start-Sleep -Milliseconds 100
} while ($clock.Elapsed.TotalSeconds -lt 8)
throw 'The fixture was not found in the Recycle Bin with its original directory'
`)
	cmd.Env = append(os.Environ(), "VIVAPAD_RECYCLE_NAME="+name, "VIVAPAD_RECYCLE_PARENT="+root,
		"VIVAPAD_RECYCLE_PAYLOAD="+base64.StdEncoding.EncodeToString(payload))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("verify Windows Recycle Bin item: %v\n%s", err, output)
	}
}
