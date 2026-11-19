//go:build darwin || linux

package document

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestSearchSourceRejectsPipeWithoutWaitingForWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search.txt")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ReadSearchSource(path, UTF8, 1024)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("reading named pipe = %v, want non-regular file error", err)
		}
	case <-time.After(2 * time.Second):
		// Unblock the reader on a regression, without delivering any input.
		fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if err == nil {
			unix.Close(fd)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}
		t.Fatal("search source waited for a named pipe writer")
	}
}
