//go:build darwin || linux

package recovery

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestLoadRejectsPipeWithoutWaitingForWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Load(path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("loading named pipe = %v, want non-regular file error", err)
		}
	case <-time.After(2 * time.Second):
		// Release a blocked reader without writing any payload.
		if fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0); err == nil {
			unix.Close(fd)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}
		t.Fatal("loading recovery waited for a named pipe writer")
	}
}
