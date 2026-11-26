package windows

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

func HostedRecycleFixture(t *testing.T, payload []byte) string {
	t.Helper()
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
	// An extensionless name is independent of the Shell's extension visibility setting.
	path := filepath.Join(root, filepath.Base(root))
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("clean fixture source: %v", err)
		}
		if err := os.Remove(root); err != nil {
			t.Errorf("clean fixture directory: %v", err)
		}
	})
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func CheckRecycledFixture(t *testing.T, path string, payload []byte) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source did not leave its original path: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	// A missing source alone cannot distinguish recycling from permanent deletion.
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
	cmd.Env = append(os.Environ(), "VIVAPAD_RECYCLE_NAME="+filepath.Base(path), "VIVAPAD_RECYCLE_PARENT="+filepath.Dir(path),
		"VIVAPAD_RECYCLE_PAYLOAD="+base64.StdEncoding.EncodeToString(payload))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("verify Windows Recycle Bin item: %v\n%s", err, output)
	}
}
