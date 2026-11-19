package recovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"vivapad/internal/document"
)

func TestSaveLoadPrivateDirtyBuffers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "recovery.json")
	// A saved-path tab keeps a platform-absolute path: "/tmp/example.go" is not
	// absolute on Windows, where Save rejects it.
	saved := filepath.Join(t.TempDir(), "example.go")
	want := State{Active: 0, Tabs: []Tab{{
		Path: saved, Name: "example.go", Content: "猫\ntext",
		Encoding: document.UTF8, LineEnding: document.LF,
		DiskDigest: strings.Repeat("a", 64), SelectionStart: 1, SelectionEnd: 1, ReadOnly: true, Pinned: true,
	}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded=%+v err=%v, want %+v", got, err, want)
	}
	info, err := os.Stat(path)
	if err != nil || !privateMode(info.Mode()) {
		t.Fatalf("backup file permissions = %+v, %v", info, err)
	}
	directory, err := os.Stat(filepath.Dir(path))
	if err != nil || !privateDirMode(directory.Mode()) {
		t.Fatalf("backup directory permissions = %+v, %v", directory, err)
	}
	if err := Clear(path); err != nil {
		t.Fatal(err)
	}
	if state, err := Load(path); err != nil || len(state.Tabs) != 0 {
		t.Fatalf("cleared backup = %+v, %v", state, err)
	}
}

func TestRecoveryV9TabColorRoundTripAndV8Fallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	want := State{Active: 0, Tabs: []Tab{{Name: "new 1", Content: "draft", TabColor: 4}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("recovery color = %+v, %v", got, err)
	}
	want.Tabs[0].TabColor = 6
	if err := Save(path, want); err == nil {
		t.Fatal("accepted invalid recovery tab color")
	}
	if err := os.WriteFile(path, []byte(`{"version":8,"active":0,"tabs":[{"name":"new 1","content":"draft","tab_color":4}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got.Tabs[0].TabColor != 0 {
		t.Fatalf("v8 recovery tab color = %+v, %v", got, err)
	}
}

func TestMacintoshCRRecoveryRoundTripsWithoutChangingEnumValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	want := State{Active: 0, Tabs: []Tab{{Name: "new 1", Content: "a\nb", Encoding: document.UTF8, LineEnding: document.CR}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("CR recovery = %+v, %v", got, err)
	}
}

func TestMixedEOLRecoveryRoundTripAndInvalidSequence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	want := State{Active: 0, Tabs: []Tab{{Name: "mixed.txt", Content: "a\nb\nc\n", Encoding: document.UTF8, LineEnding: document.CRLF, LineEndings: "WLM"}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed EOL backup = %+v, %v", got, err)
	}
	for _, invalid := range []string{"W", "W?M", "WLMM"} {
		bad := want
		bad.Tabs = []Tab{want.Tabs[0]}
		bad.Tabs[0].LineEndings = invalid
		if err := Save(path, bad); err == nil {
			t.Fatalf("accepted invalid EOL sequence %q", invalid)
		}
	}
}

func TestRecoveryRejectsCorruptAndOversizedData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	for _, content := range []string{
		"{",
		`{"version":10,"active":-1,"tabs":[]}`,
		`{"version":1,"active":0,"tabs":[{"path":"relative.txt","name":"x","content":"x","disk_digest":""}]}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted corrupt backup %q", content)
		}
	}
	if err := Save(path, State{Active: 0, Tabs: []Tab{{Name: "huge", Content: strings.Repeat("x", 33<<20)}}}); err == nil {
		t.Fatal("wrote backup above the size limit")
	}
}

func TestEscapedOversizeRecoveryKeepsPreviousBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	if err := Save(path, State{Active: 0, Tabs: []Tab{{Name: "draft", Content: "keep me"}}}); err != nil {
		t.Fatal(err)
	}
	state := State{Active: 0, Tabs: []Tab{{Name: "draft", Content: strings.Repeat("\x01", 6<<20)}}}
	if err := Save(path, state); err == nil || !strings.Contains(err.Error(), "exceeds 32 MiB") {
		t.Fatalf("oversized encoded backup = %v", err)
	}
	loaded, err := Load(path)
	if err != nil || len(loaded.Tabs) != 1 || loaded.Tabs[0].Content != "keep me" {
		t.Fatalf("refused backup changed the last valid draft: %+v, %v", loaded, err)
	}
}

func TestEncodedContentsFitAtJSONSizeBoundary(t *testing.T) {
	tabs := []Tab{{Content: "\x00\x01\b\f\n\r\t"}, {Content: "<>&\"\\\u2028\u2029\u732b\U0001f4dd"}, {}}
	size := 0
	for _, tab := range tabs {
		encoded, err := json.Marshal(tab.Content)
		if err != nil {
			t.Fatal(err)
		}
		size += len(encoded)
	}
	if !encodedContentsFit(tabs, size) || encodedContentsFit(tabs, size-1) {
		t.Fatalf("content size check disagrees with encoding/json at %d bytes", size)
	}
}

func BenchmarkRejectEscapedOversizeRecovery(b *testing.B) {
	path := filepath.Join(b.TempDir(), "recovery.json")
	state := State{Active: 0, Tabs: []Tab{{Name: "draft", Content: strings.Repeat("\x01", 6<<20)}}}
	b.ReportAllocs()
	for b.Loop() {
		if err := Save(path, state); err == nil || !strings.Contains(err.Error(), "exceeds 32 MiB") {
			b.Fatalf("oversized encoded backup = %v", err)
		}
	}
}

func TestV3RecoveryDefaultsToEditableBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	value := `{"version":3,"active":0,"tabs":[{"name":"new 1","content":"draft"}]}`
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || len(stored.Tabs) != 1 || stored.Tabs[0].ReadOnly {
		t.Fatalf("v3 read-only recovery migration = %+v, %v", stored, err)
	}
}

func TestV4RecoveryDefaultsToUnpinnedBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	value := `{"version":4,"active":0,"tabs":[{"name":"new 1","content":"draft","pinned":true}]}`
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || len(stored.Tabs) != 1 || stored.Tabs[0].Pinned {
		t.Fatalf("v4 pin migration = %+v, %v", stored, err)
	}
}

func TestWriterCoalescesLatestAndClearWaits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "recovery.json")
	writer := NewWriter(path, nil)
	first := State{Active: 0, Tabs: []Tab{{Name: "new 1", Content: "first"}}}
	latest := State{Active: 0, Tabs: []Tab{{Name: "new 1", Content: "latest"}}}
	writer.Submit(first)
	writer.Submit(latest)
	if err := writer.Wait(); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || !reflect.DeepEqual(stored, latest) {
		t.Fatalf("coalesced backup = %+v, %v", stored, err)
	}
	if err := writer.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("backup reappeared after clear: %v", err)
	}
}

func TestRecoveryPersistsSplitPaneOfDirtyTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	want := State{Active: 0, Split: true, ActivePane: 1, Divider: 0.42,
		Tabs: []Tab{{Name: "new 2", Content: "right draft", Pane: 1}},
	}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("split backup = %+v, %v, want %+v", got, err, want)
	}
}

func TestLegacyRecoveryV1StillLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"tabs":[{"name":"new 1","content":"draft"}],"active":0}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || len(got.Tabs) != 1 || got.Split || got.Tabs[0].Pane != 0 {
		t.Fatalf("legacy recovery = %+v, %v", got, err)
	}
}

func TestLegacyRecoveryV2UsesCurrentEncodingAsDiskFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	content := `{"version":2,"active":0,"tabs":[{"name":"new 1","content":"中文","encoding":4}]}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || len(got.Tabs) != 1 || got.Tabs[0].DiskEncoding != document.GB18030 {
		t.Fatalf("v2 recovery = %+v, %v", got, err)
	}
}

func TestRecoveryPersistsRegisteredCurrentAndDiskEncodings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	want := State{Active: 0, Tabs: []Tab{{Name: "draft", Content: "日本", Encoding: document.GBK, DiskEncoding: document.ShiftJIS}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("registered recovery = %+v, %v", got, err)
	}
}

func TestRecoveryPreservesExplicitNormalTextAndRustModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	want := State{Active: 0, Tabs: []Tab{
		{Name: "source.go", Content: "draft", LanguageSelected: true},
		{Name: "notes.txt", Content: "fn main() {}", Language: ".rs", LanguageSelected: true},
	}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("language recovery = %+v, %v", got, err)
	}
	want.Tabs[1].Language = "not-a-language"
	if err := Save(path, want); err == nil {
		t.Fatal("unknown recovery language accepted")
	}
}

func TestRecoveryFoldHeadersRoundTripAndRejectUnsafeLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	want := State{Active: 0, Tabs: []Tab{{Name: "draft.json", Content: "{\n  value\n}\n", FoldedHeaders: []int{0}}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("recovery folds = %+v, %v", got, err)
	}
	want.Tabs[0].FoldedHeaders = []int{0, 0}
	if err := Save(path, want); err == nil {
		t.Fatal("repeated recovery fold accepted")
	}
	want.Tabs[0].FoldedHeaders = []int{4}
	if err := Save(path, want); err == nil {
		t.Fatal("out-of-range recovery fold accepted")
	}
}

// privateMode reports whether mode carries the private-file metadata this
// platform supports: POSIX 0600 on macOS and Linux, or a writable regular file
// on Windows, where Go reports synthetic permission bits and VivaPad keeps the
// privacy of its stores in the profile directory ACL (a separate gate).
func privateMode(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return mode.IsRegular() && mode.Perm()&0200 != 0
	}
	return mode.Perm() == 0600
}

// privateDirMode reports whether mode carries a private directory's metadata:
// POSIX 0700 on macOS and Linux, or a writable directory on Windows.
func privateDirMode(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return mode.IsDir() && mode.Perm()&0200 != 0
	}
	return mode.Perm() == 0700
}
