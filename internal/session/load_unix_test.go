//go:build darwin || linux

package session

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestSessionLoadersRejectNamedPipeBeforeReading(t *testing.T) {
	for _, tc := range []struct {
		name string
		load func(string) error
	}{
		{"private JSON", func(path string) error { _, err := Load(path); return err }},
		{"NPP XML", func(path string) error { _, err := LoadNPP(path); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session")
			if err := unix.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- tc.load(path) }()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "not a regular file") {
					t.Fatalf("loading FIFO = %v, want a nonregular-file error", err)
				}
			case <-time.After(2 * time.Second):
				if fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0); err == nil {
					unix.Close(fd)
					select {
					case <-done:
					case <-time.After(time.Second):
					}
				}
				t.Fatal("session loader waited for a FIFO writer")
			}
		})
	}
}
