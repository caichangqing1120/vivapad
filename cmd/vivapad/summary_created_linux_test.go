package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vivaui/viva/text"
	"golang.org/x/sys/unix"
)

func TestLinuxSummaryShowsFileCreationTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.txt")
	const contents = "example\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	modified := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
	var stat unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, path, 0, unix.STATX_BTIME, &stat)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EOPNOTSUPP) || (err == nil && stat.Mask&unix.STATX_BTIME == 0) {
		t.Skip("fixture filesystem does not report creation time")
	}
	if err != nil {
		t.Fatal(err)
	}
	const format = "2006-01-02 15:04:05"
	createdText := time.Unix(stat.Btime.Sec, int64(stat.Btime.Nsec)).Local().Format(format)
	modifiedText := modified.Local().Format(format)
	if createdText == modifiedText {
		t.Fatal("fixture creation and modified times are identical")
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	selection := text.Range{Start: 1, End: 4}
	doc.Editor.SetSelection(selection)
	state.command("Summary...")
	if !state.summaryOpen || state.summary.created != createdText || state.summary.modified != modifiedText {
		t.Fatalf("Summary created=%q, modified=%q; want %q, %q", state.summary.created, state.summary.modified, createdText, modifiedText)
	}
	if doc.Editor.String() != contents || doc.Editor.Selection() != selection || doc.Dirty() {
		t.Fatal("opening Summary changed the document")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := summaryFor(doc); got.created != "" || got.modified != "" {
		t.Fatalf("missing file unexpectedly reports created=%q, modified=%q", got.created, got.modified)
	}
}

func TestLinuxCreationTimeRejectsReplacedPath(t *testing.T) {
	dir := t.TempDir()
	path, replacement := filepath.Join(dir, "source.txt"), filepath.Join(dir, "replacement.txt")
	for _, name := range []string{path, replacement} {
		if err := os.WriteFile(name, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	old, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(old, current) {
		t.Fatal("fixture did not replace the file")
	}
	if _, ok := creationTime(path, current); !ok {
		t.Skip("fixture filesystem does not report creation time")
	}
	if got, ok := creationTime(path, old); ok {
		t.Fatalf("old file's metadata picked up replacement creation time %v", got)
	}
}
