package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/recovery"
	"vivapad/internal/session"
	"vivapad/internal/syntax"
)

func clickLanguage(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	for _, direction := range []string{"Next languages", "Previous languages"} {
		for range len(syntax.Languages()) {
			for _, hit := range driver.Snapshot().Hits {
				if hit.Label == label && hit.Role == gfx.RoleMenuItem {
					clickID(t, driver, hit.ID)
					return
				}
			}
			clickLabeledControl(t, driver, direction)
		}
	}
	t.Fatalf("language %q is unreachable", label)
}

func TestRegisteredLanguageEditingOptionsUseCatalog(t *testing.T) {
	for _, spec := range syntax.Languages() {
		if spec.Mode == "" {
			continue
		}
		options, known := codeOptions(spec.Mode)
		if !known || options.LineComment != spec.LineComment || options.IndentAfter != spec.IndentAfter {
			t.Fatalf("%q code options = %+v, known=%v", spec.Label, options, known)
		}
	}
}

func TestLanguageMenuReachesRegisteredLexersAndPreservesSplitText(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	left := state.space.Current()
	left.Editor.Insert("left")
	state.command("Go")
	right := state.space.NewDocument()
	right.Editor.Insert("fn main() {}")
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	clickID(t, driver, menuScope.At(5))
	goIndexPreserved := false
	for _, hit := range driver.Snapshot().Hits {
		goIndexPreserved = goIndexPreserved || hit.ID == menuItemScope.At(1) && hit.Label == "Go"
	}
	if !goIndexPreserved {
		t.Fatal("original Go menu index changed")
	}
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(0) {
			if err := driver.Scroll(gfx.ScrollEvent{X: hit.Bounds.X + 20, Y: hit.Bounds.Y + 8, DeltaY: 120}); err != nil {
				t.Fatal(err)
			}
			if state.languageOffset == 0 {
				t.Fatal("Language menu did not respond to wheel input")
			}
			if err := driver.Scroll(gfx.ScrollEvent{X: hit.Bounds.X + 20, Y: hit.Bounds.Y + 8, DeltaY: -120}); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	seen := make(map[string]bool)
	for range len(syntax.Languages()) {
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID.Scope() == menuItemScope {
				seen[hit.Label] = true
				if hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("language row clipped: %+v", hit.Bounds)
				}
			}
		}
		if len(seen) == len(syntax.Languages()) {
			break
		}
		clickLabeledControl(t, driver, "Next languages")
	}
	if len(seen) != len(syntax.Languages()) {
		t.Fatalf("reachable languages = %d, want %d", len(seen), len(syntax.Languages()))
	}
	clickLanguage(t, driver, "Rust")
	if state.languageFor(left) != ".go" || state.languageFor(right) != ".rs" || left.Editor.String() != "left" || right.Editor.String() != "fn main() {}" {
		t.Fatalf("split language selection = left:%q right:%q", state.languageFor(left), state.languageFor(right))
	}
	if options, ok := right.Editor.Code(); !ok || options.LineComment != "//" {
		t.Fatalf("Rust code options = %+v, %v", options, ok)
	}
}

func TestLanguageOverridePersistsSessionAndRecovery(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "source.go"), filepath.Join(root, "notes.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{first, second}, sessionPath)
	state.space.Select(0)
	state.command("Normal Text")
	state.space.Select(1)
	state.command("Rust")
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	if err != nil || len(stored.Languages) != 2 || stored.Languages[state.space.Documents[0].Path] != "" || stored.Languages[state.space.Documents[1].Path] != ".rs" {
		t.Fatalf("saved languages = %+v, %v", stored.Languages, err)
	}
	restarted := newStartupModel(nil, sessionPath)
	if state := restarted; state.languageFor(state.space.Documents[0]) != "" || state.languageFor(state.space.Documents[1]) != ".rs" {
		t.Fatalf("restored languages = %q / %q", state.languageFor(state.space.Documents[0]), state.languageFor(state.space.Documents[1]))
	}
	explicit := newStartupModel([]string{first}, sessionPath)
	if explicit.languageFor(explicit.space.Current()) != "" {
		t.Fatalf("explicit CLI open lost Normal Text: %q", explicit.languageFor(explicit.space.Current()))
	}
	restarted.space.Select(0)
	restarted.space.Current().Editor.Insert("draft")
	restarted.space.Select(1)
	restarted.space.Current().Editor.Insert("draft")
	backup := restarted.currentRecovery()
	if len(backup.Tabs) != 2 || !backup.Tabs[0].LanguageSelected || backup.Tabs[0].Language != "" || backup.Tabs[1].Language != ".rs" {
		t.Fatalf("recovery language snapshot = %+v", backup.Tabs)
	}
	if err := recovery.Save(restarted.recoveryPath, backup); err != nil {
		t.Fatal(err)
	}
	recovered := newStartupModel(nil, sessionPath)
	if len(recovered.space.Documents) != 2 || recovered.languageFor(recovered.space.Documents[0]) != "" || recovered.languageFor(recovered.space.Documents[1]) != ".rs" {
		t.Fatalf("recovered language choices = %+v, notice=%q", recovered.languages, recovered.notice)
	}
}

func TestNPPXMLSupportsExpandedLanguageAndRejectsUnknownBeforeImport(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("fn main() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{file})
	doc := state.space.Current()
	state.command("Rust")
	if nppLanguage(state.languageFor(doc)) != "rust" {
		t.Fatal("Rust NPP language was not exported")
	}
	if exported, err := state.nppSessionState(); err != nil || len(exported.Main) != 1 || exported.Main[0].Language != "rust" {
		t.Fatalf("app NPP export = %+v, %v", exported, err)
	}
	xmlPath := filepath.Join(root, "session.xml")
	if err := session.SaveNPP(xmlPath, session.NPPState{Main: []session.NPPFile{{Path: doc.Path, Encoding: -1, Language: "rust"}}}); err != nil {
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
	if loaded.languageFor(loaded.space.Current()) != ".rs" {
		t.Fatalf("Rust NPP import = %q, notice=%q", loaded.languageFor(loaded.space.Current()), loaded.notice)
	}
	if _, err := nppExtension("unknown-lang"); err == nil {
		t.Fatal("unknown NPP language accepted")
	}
	badXML := filepath.Join(root, "invalid.xml")
	if err := session.SaveNPP(badXML, session.NPPState{Main: []session.NPPFile{
		{Path: doc.Path, Encoding: -1, Language: "rust"},
		{Path: filepath.Join(root, "another.txt"), Encoding: -1, Language: "unknown-lang"},
	}}); err != nil {
		t.Fatal(err)
	}
	dirty := loaded.space.NewDocument()
	dirty.Editor.Insert("keep")
	before := len(loaded.space.Documents)
	if err := driver.QueueOpenFiles([]string{badXML}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(0))
	if len(loaded.space.Documents) != before || dirty.Editor.String() != "keep" || !dirty.Dirty() {
		t.Fatalf("invalid NPP language changed workspace: docs=%d notice=%q", len(loaded.space.Documents), loaded.notice)
	}
}

func TestLanguageMenuEscapeKeepsDirtyDocumentAndYAMLAliasChecked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("enabled: true"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	state.space.Current().Editor.MoveTo(state.space.Current().Editor.RuneCount(), false)
	state.space.Current().Editor.Insert("\nnew: true")
	clickID(t, driver, menuScope.At(5))
	checked := false
	found := false
	for range len(syntax.Languages()) {
		for _, hit := range driver.Snapshot().Hits {
			if hit.Label == "YAML" {
				found, checked = true, hit.Checked
				break
			}
		}
		if found {
			break
		}
		clickLabeledControl(t, driver, "Next languages")
	}
	if !found || !checked {
		t.Fatal("automatic .yml mode did not check YAML")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.menuOpen != -1 || state.space.Current().Editor.String() != "enabled: true\nnew: true" || !state.space.Current().Dirty() {
		t.Fatalf("closing Language menu: open=%d text=%q dirty=%v", state.menuOpen, state.space.Current().Editor.String(), state.space.Current().Dirty())
	}
}

func TestNPPExpandedLanguagesRemainIndependentAcrossSplitViews(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "main.go"), filepath.Join(root, "notes.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("clean"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{first})
	left := state.space.Current()
	if err := state.space.Open(second); err != nil {
		t.Fatal(err)
	}
	right := state.space.Current()
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(5))
	clickLanguage(t, driver, "Smalltalk")
	if left.Dirty() || right.Dirty() || left.Editor.String() != "clean" || right.Editor.String() != "clean" {
		t.Fatal("selecting Smalltalk changed split document contents")
	}
	exported, err := state.nppSessionState()
	if err != nil || len(exported.Main) != 1 || len(exported.Sub) != 1 || exported.Main[0].Language != "go" || exported.Sub[0].Language != "smalltalk" {
		t.Fatalf("split language export = %+v, %v", exported, err)
	}
	xmlPath := filepath.Join(root, "split.xml")
	if err := session.SaveNPP(xmlPath, exported); err != nil {
		t.Fatal(err)
	}
	loaded := newModel()
	importDriver, err := app.NewDriver(newConfig(loaded), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(importDriver.Close)
	if err := importDriver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, importDriver, menuScope.At(0))
	clickID(t, importDriver, menuItemScope.At(13))
	clickID(t, importDriver, sessionScope.At(0))
	if !loaded.splitEnabled || loaded.languageFor(loaded.paneDocs[0]) != ".go" || loaded.languageFor(loaded.paneDocs[1]) != ".st" {
		t.Fatalf("split language import = %q / %q, notice=%q", loaded.languageFor(loaded.paneDocs[0]), loaded.languageFor(loaded.paneDocs[1]), loaded.notice)
	}
}

func TestExplicitLanguageVariantsKeepExtensionDefaults(t *testing.T) {
	for _, tc := range []struct{ extension, label, defaultName, nppName, mode string }{
		{".tex", "TeX", "latex", "tex", "lang:tex"},
		{".sql", "Microsoft Transact-SQL", "sql", "mssql", ".tsql"},
		{".json", "JSON5", "json", "json5", ".json5"},
		{".txt", "Inno Setup", "normal", "inno", ".iss"},
	} {
		t.Run(tc.nppName, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "notes"+tc.extension)
			const source = "unchanged\r\n"
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{path})
			doc := state.space.Current()
			if got := nppLanguage(state.languageFor(doc)); got != tc.defaultName {
				t.Fatalf("default for %s = %q, want %q", tc.extension, got, tc.defaultName)
			}
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, menuScope.At(5))
			clickLanguage(t, driver, tc.label)
			exported, err := state.nppSessionState()
			if err != nil || len(exported.Main) != 1 || exported.Main[0].Language != tc.nppName {
				t.Fatalf("NPP session after selecting %s = %+v, %v", tc.label, exported, err)
			}
			mode, err := nppExtension(exported.Main[0].Language)
			if err != nil || mode != tc.mode || state.languageFor(doc) != tc.mode {
				t.Fatalf("round trip for %s = %q, %v; active %q", tc.label, mode, err, state.languageFor(doc))
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != source || doc.Dirty() || doc.Editor.String() != "unchanged\n" {
				t.Fatalf("selecting %s changed the document: disk %q, %v, dirty=%v", tc.label, data, err, doc.Dirty())
			}
		})
	}
}

func TestLanguageAliasesUseEditingOptionsAndExportOriginalSessionNames(t *testing.T) {
	for _, tc := range []struct{ file, label, nppName, comment string }{
		{"demo.ADB", "Ada", "ada", "--"},
		{"demo.ads", "Ada", "ada", "--"},
		{"demo.cmd", "Batch", "batch", "REM"},
		{"demo.mli", "Caml", "caml", ""},
		{"demo.cpy", "COBOL", "cobol", ""},
		{"demo.nsh", "NSIS", "nsis", ";"},
		{"demo.jsonc", "JSON5", "json5", "//"},
		{"demo.RC", "Resource file", "rc", "//"},
		{"demo.ISS", "Inno Setup", "inno", ";"},
		{"demo.MIB", "ASN.1", "asn1", ""},
		{"demo.MMS", "MMIXAL", "mmixal", "%"},
		{"demo.ERR", "ErrorList", "errorlist", "#"},
		{"demo.SCP", "Spice", "spice", "*"},
		{"demo.OUT", "Spice", "spice", "*"},
		{"demo.KIX", "KIXtart", "kix", ";"},
		{"demo.ORC", "CSound", "csound", ";"},
		{"demo.SCO", "CSound", "csound", ";"},
		{"demo.CSD", "CSound", "csound", ";"},
		{"demo.REB", "REBOL", "rebol", ";"},
		{"demo.R2", "REBOL", "rebol", ";"},
		{"demo.R3", "REBOL", "rebol", ";"},
		{"demo.AVS", "AviSynth", "avs", "#"},
		{"demo.AVSI", "AviSynth", "avs", "#"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			const source = "example\r\n"
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			state := newModelWithFiles([]string{path})
			doc := state.space.Current()
			driver, err := app.NewDriver(newConfig(state), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Close)
			if !state.menuSelected(tc.label) {
				t.Errorf("%s did not select %s automatically", tc.file, tc.label)
			}
			if options, ok := doc.Editor.Code(); !ok || options.LineComment != tc.comment {
				t.Errorf("%s editing options = %+v (%v), want comment %q", tc.file, options, ok, tc.comment)
			}
			if exported, err := state.nppSessionState(); err != nil || len(exported.Main) != 1 || exported.Main[0].Language != tc.nppName {
				t.Errorf("%s exported session = %+v, %v; want %q", tc.file, exported, err, tc.nppName)
			}
			state.command("Normal Text")
			exported, err := state.nppSessionState()
			if err != nil || len(exported.Main) != 1 || exported.Main[0].Language != "normal" || state.menuSelected(tc.label) {
				t.Errorf("%s ignored explicit Normal Text: %+v, %v", tc.file, exported, err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != source || doc.Dirty() {
				t.Errorf("language detection or override modified document: %q, %v", data, err)
			}
		})
	}
}

func TestNewLanguageModesRoundTripSessionsWithoutEditing(t *testing.T) {
	for _, tc := range []struct{ label, mode, nppName, source string }{
		{"Resource file", ".rc", "rc", "\xef\xbb\xbfSTRINGTABLE\r\nBEGIN\r\n  IDS_TITLE \"Demo\"\r\nEND\r\n"},
		{"Inno Setup", ".iss", "inno", "\xef\xbb\xbf[Setup]\r\nAppName=Demo\r\n[Code]\r\nbegin\r\nend;\r\n"},
		{"ASN.1", ".mib", "asn1", "\xef\xbb\xbfDEMO DEFINITIONS ::= BEGIN\r\nitem OBJECT IDENTIFIER ::= { parent 42 }\r\nEND\r\n"},
		{"MMIXAL", ".mms", "mmixal", "\xef\xbb\xbfMain\tGET $1,:rA\r\n\tTRAP 0,Fputs,StdOut\r\n"},
		{"ErrorList", ".err", "errorlist", "\xef\xbb\xbfmain.c:7:4: error: invalid type\r\n  ordinary output\r\n"},
		{"Spice", ".scp", "spice", "\xef\xbb\xbf* Example circuit\r\n.TRAN 100n 1.5e-3\r\n"},
		{"KIXtart", ".kix", "kix", "\xef\xbb\xbfDIM $name\r\nRETURN @CRLF\r\n"},
		{"CSound", ".orc", "csound", "\xef\xbb\xbfinstr 1\r\n  a1 oscil .5, p4\r\nendin\r\n"},
		{"REBOL", ".reb", "rebol", "\xef\xbb\xbfREBOL []\r\nprint {demo}\r\n"},
		{"AviSynth", ".avs", "avs", "\xef\xbb\xbfclip = BlankClip()\r\nReturn clip\r\n"},
	} {
		t.Run(tc.nppName, func(t *testing.T) {
			checkLanguageSessionRoundTrip(t, tc.label, tc.mode, tc.nppName, tc.source)
		})
	}
}

func checkLanguageSessionRoundTrip(t *testing.T, label, mode, nppName, source string) {
	t.Helper()
	root := t.TempDir()
	path, sessionPath := filepath.Join(root, "script.txt"), filepath.Join(root, "session.json")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
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
	clickID(t, driver, menuScope.At(5))
	clickLanguage(t, driver, label)
	if !state.menuSelected(label) || doc.Dirty() || doc.Editor.CanUndo() || doc.Editor.String() != before {
		t.Fatalf("selecting %s did not keep the clean buffer and history", label)
	}
	newConfig(state).OnCloseRequest(func() {})
	reopened := newStartupModel(nil, sessionPath)
	if doc := reopened.space.Current(); doc.Path == "" || reopened.languageFor(doc) != mode || doc.Editor.String() != before {
		t.Fatalf("private session did not restore %s: %q", label, reopened.notice)
	}
	exported, err := reopened.nppSessionState()
	if err != nil || len(exported.Main) != 1 || exported.Main[0].Language != nppName {
		t.Fatalf("%s export = %+v, %v", label, exported, err)
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
	if doc := loaded.space.Current(); doc.Path == "" || loaded.languageFor(doc) != mode || doc.Dirty() || doc.Editor.String() != before {
		t.Fatalf("NPP session did not restore %s: %q", label, loaded.notice)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != source {
		t.Fatalf("language round trip changed disk bytes = %q, %v", data, err)
	}
}
