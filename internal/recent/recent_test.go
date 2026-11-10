package recent

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func TestAddMovesExistingPathToFrontAndCapsHistory(t *testing.T) {
	root := t.TempDir()
	var entries []Entry
	for index := 0; index < 13; index++ {
		entries = Add(entries, Entry{Path: filepath.Join(root, string(rune('a'+index))+".txt")})
	}
	if len(entries) != 10 || filepath.Base(entries[0].Path) != "m.txt" || filepath.Base(entries[9].Path) != "d.txt" {
		t.Fatalf("bounded history = %+v", entries)
	}
	updated := Add(entries, Entry{Path: filepath.Join(root, "f.txt"), Encoding: document.GB18030})
	if len(updated) != 10 || filepath.Base(updated[0].Path) != "f.txt" || updated[0].Encoding != document.GB18030 {
		t.Fatalf("deduplicated history = %+v", updated)
	}
	count := 0
	for _, entry := range updated {
		if entry.Path == updated[0].Path {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("duplicate path count = %d", count)
	}
}

func TestPrivateRecentHistoryRoundTripAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "recent.json")
	want := []Entry{{Path: filepath.Join(t.TempDir(), "chinese.txt"), Encoding: document.GB18030}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("history = %+v, %v", got, err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil || fileInfo.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %+v, %v", fileInfo, err)
	}
	directoryInfo, err := os.Stat(filepath.Dir(path))
	if err != nil || directoryInfo.Mode().Perm() != 0700 {
		t.Fatalf("directory mode = %+v, %v", directoryInfo, err)
	}
	if err := Clear(path); err != nil {
		t.Fatal(err)
	}
	if entries, err := Load(path); err != nil || len(entries) != 0 {
		t.Fatalf("cleared history = %+v, %v", entries, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("history file survived clear: %v", err)
	}
}

func TestRecentHistoryPersistsRegisteredCharacterSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent.json")
	want := []Entry{{Path: filepath.Join(t.TempDir(), "japanese.txt"), Encoding: document.ShiftJIS}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("registered history = %+v, %v", got, err)
	}
}

func TestRecentHistoryRejectsInvalidAndOversizedData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent.json")
	for _, content := range []string{
		"{",
		`{"version":2,"entries":[]}`,
		`{"version":1,"entries":[{"path":"relative.txt"}]}`,
		`{"version":1,"entries":[{"path":"/tmp/a.txt","encoding":99}]}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted bad history: %q", content)
		}
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"entries":[]}`+strings.Repeat(" ", 1<<20)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("oversized history accepted")
	}
}
