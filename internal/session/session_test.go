package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSaveLoadVersionedPrivateSession(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "VivaPad", "session.json")
	first, second := filepath.Join(root, "one.txt"), filepath.Join(root, "two.txt")
	want := State{Paths: []string{first, second}, Active: second, ReadOnly: map[string]bool{first: true}, Pinned: map[string]bool{second: true}, FolderRoots: []string{filepath.Join(root, "project")}, FolderVisible: true}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded=%+v err=%v, want %+v", got, err, want)
	}
	info, err := os.Stat(path)
	if err != nil || !privateMode(info.Mode()) {
		t.Fatalf("session permissions = %+v, err=%v", info, err)
	}
	directory, err := os.Stat(filepath.Dir(path))
	if err != nil || !privateDirMode(directory.Mode()) {
		t.Fatalf("session directory permissions = %+v, err=%v", directory, err)
	}
}

func TestMissingAndDamagedSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if state, err := Load(path); err != nil || len(state.Paths) != 0 {
		t.Fatalf("missing session = %+v, %v", state, err)
	}
	for name, content := range map[string]string{
		"malformed":              "{",
		"unsupported":            `{"version":13,"paths":[],"active":""}`,
		"unknown read-only path": `{"version":4,"paths":[],"active":"","read_only":{"/missing.txt":true}}`,
		"unknown pinned path":    `{"version":5,"paths":[],"active":"","pinned":{"/missing.txt":true}}`,
		"relative folder":        `{"version":6,"paths":[],"active":"","folder_roots":["relative"]}`,
		"relative":               `{"version":1,"paths":["relative.txt"],"active":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("damaged session was accepted")
			}
		})
	}
}

func TestV3SessionDefaultsToEditableDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	// A fixture path must be absolute on the build platform, and its JSON
	// escaping must survive Windows separators.
	saved := filepath.Join(t.TempDir(), "saved.txt")
	value := `{"version":3,"paths":["` + jsonString(t, saved) + `"],"active":"` + jsonString(t, saved) + `","view":{"split":false}}`
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || stored.ReadOnly != nil || len(stored.Paths) != 1 {
		t.Fatalf("v3 read-only migration = %+v, %v", stored, err)
	}
}

func TestV4SessionDefaultsToUnpinnedDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	saved := filepath.Join(t.TempDir(), "saved.txt")
	value := `{"version":4,"paths":["` + jsonString(t, saved) + `"],"active":"` + jsonString(t, saved) + `","view":{"split":false}}`
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || stored.Pinned != nil {
		t.Fatalf("v4 pin migration = %+v, %v", stored, err)
	}
}

func TestV5SessionDefaultsToNoFolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	value := `{"version":5,"paths":[],"active":"","folder_roots":["/future"],"folder_visible":true}`
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || len(stored.FolderRoots) != 0 || stored.FolderVisible {
		t.Fatalf("v5 folder migration = %+v, %v", stored, err)
	}
}

func TestExistingSessionDirectoryIsMadePrivate(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "VivaPad")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(directory, "session.json"), State{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(directory)
	if err != nil || !privateDirMode(info.Mode()) {
		t.Fatalf("directory permissions = %+v, %v", info, err)
	}
}

func TestLoadRejectsOversizedSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	data := `{"version":1,"paths":[],"active":""}` + strings.Repeat(" ", 1<<20)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("oversized session was accepted")
	}
}

func TestSaveRejectsSessionThatCannotBeLoaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	tooLong := "/" + strings.Repeat("a", 1<<20)
	if err := Save(path, State{Paths: []string{tooLong}}); err == nil {
		t.Fatal("wrote a session exceeding the loader limit")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("oversized session left a file: %v", err)
	}
}

func TestSessionRejectsFalseReadOnlyEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	entry := "/saved.txt"
	if err := Save(path, State{Paths: []string{entry}, ReadOnly: map[string]bool{entry: false}}); err == nil {
		t.Fatal("false read-only entry was saved")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid state wrote a session: %v", err)
	}
}

func TestSessionPersistsPerViewTabsAndDivider(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	path := filepath.Join(root, "settings", "session.json")
	want := State{Paths: []string{first, second}, Active: second, View: ViewState{
		Split: true, Left: first, Right: second, RightPaths: []string{second}, ActivePane: 1, Divider: 0.37,
	}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("view session = %+v, %v, want %+v", got, err, want)
	}
}

func TestLegacyV1SessionStillLoads(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.json")
	first := filepath.Join(root, "first.txt")
	content := `{"version":1,"paths":["` + jsonString(t, first) + `"],"active":"` + jsonString(t, first) + `"}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || len(got.Paths) != 1 || got.View.Split {
		t.Fatalf("legacy session = %+v, %v", got, err)
	}
}

func TestInvalidViewOwnershipIsRejected(t *testing.T) {
	root := t.TempDir()
	first, other := filepath.Join(root, "first.txt"), filepath.Join(root, "other.txt")
	if err := Save(filepath.Join(root, "session.json"), State{Paths: []string{first}, Active: first, View: ViewState{
		Split: true, Left: first, Right: other, RightPaths: []string{other}, ActivePane: 2, Divider: 1.5,
	}}); err == nil {
		t.Fatal("invalid view state was saved")
	}
}

func TestSessionRemembersOnlyKnownLegacyEncodings(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "chinese.txt")
	path := filepath.Join(root, "session.json")
	want := State{Paths: []string{first}, Active: first, Encodings: map[string]string{first: "GB18030"}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy session = %+v, %v, want %+v", got, err, want)
	}
	want.Encodings[first] = "Unknown"
	if err := Save(path, want); err == nil {
		t.Fatal("unknown encoding persisted")
	}
	want.Encodings = map[string]string{filepath.Join(root, "not-open.txt"): "GB18030"}
	if err := Save(path, want); err == nil {
		t.Fatal("encoding for closed path persisted")
	}
}

func TestSessionPersistsRegisteredCharacterSet(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "cyrillic.txt"), filepath.Join(root, "japanese.txt")
	path := filepath.Join(root, "session.json")
	want := State{Paths: []string{first, second}, Active: second, Encodings: map[string]string{
		first: "Windows-1251", second: "Shift-JIS",
	}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("registered session = %+v, %v", got, err)
	}
}

func TestSessionPersistsExplicitLanguageAndNormalTextOverride(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "source.go"), filepath.Join(root, "notes.txt")
	path := filepath.Join(root, "session.json")
	want := State{Paths: []string{first, second}, Active: second, Languages: map[string]string{first: "", second: ".rs"}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("manual language session = %+v, %v", got, err)
	}
	want.Languages[second] = "unknown"
	if err := Save(path, want); err == nil {
		t.Fatal("unknown language persisted")
	}
	want.Languages = map[string]string{filepath.Join(root, "closed.txt"): ".go"}
	if err := Save(path, want); err == nil {
		t.Fatal("language for closed path persisted")
	}
}

func TestLegacyV6SessionHasNoManualLanguageOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(`{"version":6,"paths":[],"active":"","view":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got.Languages != nil {
		t.Fatalf("v6 language migration = %+v, %v", got, err)
	}
}

func TestSessionFoldHeadersRequireMatchingDiskIdentityAndKnownPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	file := filepath.Join(t.TempDir(), "fold.go")
	want := State{Paths: []string{file}, Active: file, Folds: map[string]FoldState{
		file: {Digest: strings.Repeat("a", 64), Headers: []int{0, 3}},
	}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("private folds = %+v, %v", got, err)
	}
	want.Folds[file] = FoldState{Digest: "short", Headers: []int{0}}
	if err := Save(path, want); err == nil {
		t.Fatal("invalid fold digest accepted")
	}
	want.Folds[file] = FoldState{Digest: strings.Repeat("a", 64), Headers: []int{3, 3}}
	if err := Save(path, want); err == nil {
		t.Fatal("repeated fold header accepted")
	}
	want.Folds = map[string]FoldState{filepath.Join(t.TempDir(), "closed.go"): {Digest: strings.Repeat("a", 64), Headers: []int{0}}}
	if err := Save(path, want); err == nil {
		t.Fatal("fold state for a closed path accepted")
	}
}

func TestLegacyV7SessionHasNoFoldState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(`{"version":7,"paths":[],"active":"","view":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got.Folds != nil {
		t.Fatalf("v7 fold migration = %+v, %v", got, err)
	}
}

func TestDocumentMapVisiblePersistsAndV8DefaultsOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	want := State{DocumentMapVisible: true}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("document map session = %+v, %v", got, err)
	}
	if err := os.WriteFile(path, []byte(`{"version":8,"paths":[],"active":"","view":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got.DocumentMapVisible {
		t.Fatalf("v8 map migration = %+v, %v", got, err)
	}
}

func TestFunctionListDockPersistsAndOlderSessionDefaultsOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	want := State{FolderVisible: true, FunctionListVisible: true, ActiveLeftDock: "functions"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("function dock session = %+v, %v", got, err)
	}
	want.FunctionListVisible = false
	if err := Save(path, want); err == nil {
		t.Fatal("hidden Function List stayed selected as dock tab")
	}
	if err := os.WriteFile(path, []byte(`{"version":9,"paths":[],"active":"","view":{},"folder_visible":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got.FunctionListVisible || got.ActiveLeftDock != "" || !got.FolderVisible {
		t.Fatalf("v9 Function List migration = %+v, %v", got, err)
	}
}

func TestDocumentListDockV11AndV10Migration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	want := State{FolderVisible: true, FunctionListVisible: true, DocumentListVisible: true, ActiveLeftDock: "documents"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("document dock session = %+v, %v", got, err)
	}
	want.DocumentListVisible = false
	if err := Save(path, want); err == nil {
		t.Fatal("closed Document List remained selected")
	}
	if err := os.WriteFile(path, []byte(`{"version":10,"paths":[],"active":"","view":{},"folder_visible":true,"function_list_visible":true,"active_left_dock":"functions"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got.DocumentListVisible || !got.FolderVisible || !got.FunctionListVisible || got.ActiveLeftDock != "functions" {
		t.Fatalf("v10 document dock migration = %+v, %v", got, err)
	}
}

func TestTabColorsV12RoundTripAndV11Migration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	doc := filepath.Join(t.TempDir(), "colored.txt")
	want := State{Paths: []string{doc}, Active: doc, TabColors: map[string]uint8{doc: 5}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("colored session = %+v, %v", got, err)
	}
	for _, color := range []uint8{0, 6} {
		want.TabColors[doc] = color
		if err := Save(path, want); err == nil {
			t.Fatalf("accepted invalid tab color %d", color)
		}
	}
	quoted := jsonString(t, doc)
	if err := os.WriteFile(path, []byte(`{"version":11,"paths":["`+quoted+`"],"active":"`+quoted+`","view":{},"tab_colors":{"`+quoted+`":5}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || len(got.TabColors) != 0 {
		t.Fatalf("v11 tab colors = %+v, %v", got.TabColors, err)
	}
}

func TestLegacyV2SessionStillLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"paths":[],"active":"","view":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || len(got.Encodings) != 0 {
		t.Fatalf("v2 session = %+v, %v", got, err)
	}
}

// jsonString escapes value for use inside a JSON string literal, so fixtures that
// embed a path stay valid where the path contains backslashes (Windows).
func jsonString(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded[1 : len(encoded)-1])
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
