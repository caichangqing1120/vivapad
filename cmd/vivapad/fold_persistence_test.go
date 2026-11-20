package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"vivapad/internal/recovery"
	"vivapad/internal/session"
)

func TestFoldStatePrivateSessionChecksDiskAndDirtyRecoveryRestores(t *testing.T) {
	root := t.TempDir()
	path, sessionPath := filepath.Join(root, "fold.go"), filepath.Join(root, "settings", "session.json")
	value := "func main() {\n  if true {\n    work()\n  }\n}\n"
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	state.command("Fold All")
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	canonical := state.space.Current().Path
	if err != nil || len(stored.Folds[canonical].Headers) != 2 || stored.Folds[canonical].Digest != state.space.Current().DiskDigest() {
		t.Fatalf("private fold state = %+v, %v", stored.Folds, err)
	}
	restarted := newStartupModel(nil, sessionPath)
	if ranges := restarted.foldRanges(restarted.space.Current()); len(ranges) != 1 || ranges[0].Header != 0 {
		t.Fatalf("restored clean fold = %+v, notice=%q", ranges, restarted.notice)
	}
	doc := restarted.space.Current()
	doc.Editor.MoveTo(0, false)
	doc.Editor.Insert("// draft\n")
	restarted.command("Fold All")
	backup := restarted.currentRecovery()
	if len(backup.Tabs) != 1 || len(backup.Tabs[0].FoldedHeaders) != 2 || backup.Tabs[0].FoldedHeaders[0] != 1 {
		t.Fatalf("dirty recovery fold snapshot = %+v", backup.Tabs)
	}
	if err := recovery.Save(restarted.recoveryPath, backup); err != nil {
		t.Fatal(err)
	}
	recovered := newStartupModel(nil, sessionPath)
	if got := recovered.space.Current(); got.Editor.String() != "// draft\n"+value || len(recovered.foldRanges(got)) != 1 || recovered.foldRanges(got)[0].Header != 1 {
		t.Fatalf("dirty fold recovery = %+v, notice=%q", recovered.foldRanges(got), recovered.notice)
	}
	if err := recovery.Clear(restarted.recoveryPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value+"changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := newStartupModel(nil, sessionPath)
	if len(changed.foldRanges(changed.space.Current())) != 0 || changed.space.Current().Editor.String() != value+"changed\n" {
		t.Fatal("stale fold state was applied to changed disk content")
	}
}

func TestIntelHEXFoldStateRestoresFromSessionWithoutEditing(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "address.HEX")
	sessionPath := filepath.Join(root, "settings", "session.json")
	const original = ":020000040002F8\r\n:0100000001FE\r\n:00000001FF\r\n"
	const text = ":020000040002F8\n:0100000001FE\n:00000001FF\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	state.command("Fold All")
	doc := state.space.Current()
	if got := state.foldRanges(doc); len(got) != 1 || got[0].Header != 0 || got[0].LastLine != 1 {
		t.Fatalf("Intel HEX fold ranges = %+v", got)
	}
	exported, err := state.nppSessionState()
	if err != nil || len(exported.Main) != 1 || exported.Main[0].Language != "ihex" ||
		len(exported.Main[0].Folds) != 1 || exported.Main[0].Folds[0] != 0 {
		t.Fatalf("NPP HEX fold export = %+v, %v", exported, err)
	}
	newConfig(state).OnCloseRequest(func() {})
	restarted := newStartupModel(nil, sessionPath)
	doc = restarted.space.Current()
	if got := restarted.foldRanges(doc); len(got) != 1 || got[0].Header != 0 || got[0].LastLine != 1 {
		t.Fatalf("restored HEX fold = %+v, notice=%q", got, restarted.notice)
	}
	restarted.command("Normal Text")
	if got := restarted.foldRanges(doc); len(got) != 0 || doc.Dirty() || doc.Editor.CanUndo() || doc.Editor.String() != text {
		t.Fatal("explicit plain text did not clear the fold without editing")
	}
	if value, err := os.ReadFile(path); err != nil || string(value) != original {
		t.Fatalf("folding modified CRLF bytes: %q, %v", value, err)
	}
}

func TestInnoSetupFoldingRestoresSessionsAndExpandsInCompactEditor(t *testing.T) {
	checkLanguageFoldingRoundTrip(t, "script.ISS", "inno",
		"\xef\xbb\xbf[Setup]\r\nAppName=Demo\r\n[Files]\r\nSource: data.txt\r\n",
		[]components.CodeEditorFoldRange{{Header: 0, LastLine: 1}, {Header: 2, LastLine: 3}},
		[]string{"AppName=Demo", "Source: data.txt"})
}

func TestCSoundFoldingRestoresSessionsAndExpandsInCompactEditor(t *testing.T) {
	checkLanguageFoldingRoundTrip(t, "score.CSD", "csound",
		"\xef\xbb\xbfinstr 1\r\n  a1 oscil .5, p4\r\nendin\r\ninstr 2\r\n  out a1\r\nendin\r\n",
		[]components.CodeEditorFoldRange{{Header: 0, LastLine: 2}, {Header: 3, LastLine: 5}},
		[]string{"a1 oscil .5", "out a1"})
}

func TestREBOLFoldingRestoresSessionsAndExpandsInCompactEditor(t *testing.T) {
	checkLanguageFoldingRoundTrip(t, "script.R3", "rebol",
		"\xef\xbb\xbfREBOL [\r\n  Title: \"demo\"\r\n]\r\nmain: [\r\n  print {inside [ignored]}\r\n]\r\n",
		[]components.CodeEditorFoldRange{{Header: 0, LastLine: 2}, {Header: 3, LastLine: 5}},
		[]string{"Title:", "inside [ignored]"})
}

func checkLanguageFoldingRoundTrip(t *testing.T, name, language, original string, want []components.CodeEditorFoldRange, bodies []string) {
	t.Helper()
	root := t.TempDir()
	path, sessionPath := filepath.Join(root, name), filepath.Join(root, "session.json")
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	doc := state.space.Current()
	before := doc.Editor.String()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, 11)
	clickID(t, driver, menuItemScope.At(11))
	if got := state.foldRanges(doc); !slices.Equal(got, want) {
		t.Fatalf("%s Fold All = %+v, want %+v", language, got, want)
	}
	for _, label := range driver.Snapshot().Texts {
		for _, body := range bodies {
			if strings.Contains(label.Value, body) {
				t.Fatalf("painted a collapsed body: %q", label.Value)
			}
		}
	}
	clickID(t, driver, leftFoldScope.At(0))
	if got := state.foldRanges(doc); !slices.Equal(got, want[1:]) {
		t.Fatalf("gutter expansion changed the other section: %+v", got)
	}
	clickID(t, driver, leftFoldScope.At(0))
	if doc.Dirty() || doc.Editor.CanUndo() || doc.Editor.String() != before {
		t.Fatal("folding changed buffer content or history")
	}
	exported, err := state.nppSessionState()
	headers := make([]int, 0, len(want))
	for _, region := range want {
		headers = append(headers, region.Header)
	}
	if err != nil || len(exported.Main) != 1 || exported.Main[0].Language != language || !slices.Equal(exported.Main[0].Folds, headers) {
		t.Fatalf("NPP %s fold export = %+v, %v", language, exported, err)
	}
	newConfig(state).OnCloseRequest(func() {})
	reopened := newStartupModel(nil, sessionPath)
	if got := reopened.foldRanges(reopened.space.Current()); !slices.Equal(got, want) {
		t.Fatalf("private %s fold restore = %+v; %q", language, got, reopened.notice)
	}
	xmlPath := filepath.Join(root, "session.xml")
	if err := session.SaveNPP(xmlPath, exported); err != nil {
		t.Fatal(err)
	}
	loaded, loader := startDriver(t)
	if err := loader.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, loader, menuScope.At(0))
	clickID(t, loader, menuItemScope.At(13))
	clickID(t, loader, sessionScope.At(0))
	if got := loaded.foldRanges(loaded.space.Current()); !slices.Equal(got, want) {
		t.Fatalf("NPP %s fold import = %+v; %q", language, got, loaded.notice)
	}
	loaded.command("Unfold All")
	if doc := loaded.space.Current(); len(loaded.foldRanges(doc)) != 0 || doc.Dirty() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("Unfold All changed source or history")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != original {
		t.Fatalf("%s folding changed the saved bytes: %q, %v", language, data, err)
	}
}

func TestNPPFoldXMLExportImportAndInvalidPreflight(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fold.go")
	if err := os.WriteFile(path, []byte("func main() {\n  work()\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.command("Fold All")
	exported, err := state.nppSessionState()
	if err != nil || len(exported.Main) != 1 || len(exported.Main[0].Folds) != 1 || exported.Main[0].Folds[0] != 0 {
		t.Fatalf("NPP fold export = %+v, %v", exported, err)
	}
	xmlPath := filepath.Join(root, "fold.xml")
	if err := session.SaveNPP(xmlPath, exported); err != nil {
		t.Fatal(err)
	}
	loaded := newModel()
	driver, err := app.NewDriver(newConfig(loaded), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	if got := loaded.foldRanges(loaded.space.Current()); len(got) != 1 || got[0].Header != 0 {
		t.Fatalf("NPP fold import = %+v, notice=%q", got, loaded.notice)
	}
	bad := filepath.Join(root, "bad.xml")
	if err := session.SaveNPP(bad, session.NPPState{Main: []session.NPPFile{{Path: path, Encoding: -1, Folds: []int{99}}}}); err != nil {
		t.Fatal(err)
	}
	invalid := newModel()
	invalid.space.Current().Editor.Insert("keep")
	invalidDriver, err := app.NewDriver(newConfig(invalid), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer invalidDriver.Close()
	if err := invalidDriver.QueueOpenFiles([]string{bad}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, invalidDriver, menuScope.At(0))
	clickID(t, invalidDriver, menuItemScope.At(13))
	clickID(t, invalidDriver, sessionScope.At(0))
	if len(invalid.space.Documents) != 1 || invalid.space.Current().Editor.String() != "keep" || !invalid.space.Current().Dirty() {
		t.Fatalf("invalid NPP fold changed dirty workspace: %+v", invalid.space.Documents)
	}
}
