package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/recent"
	"vivapad/internal/recovery"
	"vivapad/internal/session"
)

func TestRegisteredEncodingSurvivesSessionRecentAndClosedHistory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "japanese.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte{0x93, 0xfa, 0x96, 0x7b}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, sessionPath)
	if err := state.space.OpenWithEncoding(path, document.ShiftJIS); err != nil {
		t.Fatal(err)
	}
	path = state.space.Current().Path
	state.showInActivePane(state.space.Current())
	state.rememberRecent(state.space.Current())
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	if err != nil || stored.Encodings[path] != "Shift-JIS" {
		t.Fatalf("saved session = %+v, %v", stored, err)
	}
	entries, err := recent.Load(state.recentPath)
	if err != nil || len(entries) != 1 || entries[0].Encoding != document.ShiftJIS {
		t.Fatalf("recent history = %+v, %v", entries, err)
	}
	restarted := newStartupModel(nil, sessionPath)
	if doc := restarted.space.Current(); doc.Editor.String() != "日本" || doc.Encoding() != document.ShiftJIS {
		t.Fatalf("restarted session = %+v, notice=%q", doc, restarted.notice)
	}
	restarted.recordClosed(restarted.space.Current())
	if entries, err := recent.Load(restarted.closedPath); err != nil || len(entries) != 1 || entries[0].Encoding != document.ShiftJIS {
		t.Fatalf("closed history = %+v, %v", entries, err)
	}
}

func TestRegisteredDiskEncodingRestoresRecoveryWithoutChangingSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "japanese.txt")
	sessionPath := filepath.Join(root, "settings", "session.json")
	data := []byte{0x93, 0xfa, 0x96, 0x7b}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := document.OpenWithEncoding(path, document.ShiftJIS)
	if err != nil {
		t.Fatal(err)
	}
	state := newStartupModel(nil, sessionPath)
	backup := recovery.State{Active: 0, Tabs: []recovery.Tab{{Path: path, Name: doc.Name, Content: "日本語", Encoding: document.ShiftJIS,
		DiskEncoding: document.ShiftJIS, DiskDigest: doc.DiskDigest(), SelectionStart: 3, SelectionEnd: 3}}}
	if err := recovery.Save(state.recoveryPath, backup); err != nil {
		t.Fatal(err)
	}
	restarted := newStartupModel(nil, sessionPath)
	if got := restarted.space.Current(); got.Editor.String() != "日本語" || got.Encoding() != document.ShiftJIS || !got.Dirty() {
		t.Fatalf("recovered document = %+v, notice=%q", got, restarted.notice)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(data) {
		t.Fatalf("recovery changed source = %x, %v", got, err)
	}
}

func TestRegisteredNPPSessionUsesOriginalCodePageBytes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "japanese.txt")
	if err := os.WriteFile(path, []byte{0x93, 0xfa, 0x96, 0x7b, '\r', '\n', 'a', 'b', 'c'}, 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.space.OpenWithEncoding(path, document.ShiftJIS); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	doc.Editor.SetSelection(text.Range{Start: 1, End: 3})
	if got := nppBytePosition(doc, 3); got != 6 {
		t.Fatalf("Shift-JIS CRLF byte position = %d, want 6", got)
	}
	if got := nppSelectionAt(doc, 6, 6).Start; got != 3 {
		t.Fatalf("Shift-JIS CRLF rune position = %d, want 3", got)
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	xmlPath := filepath.Join(root, "session.xml")
	if err := driver.QueueSaveFile(xmlPath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickID(t, driver, menuItemScope.At(13))
	clickID(t, driver, sessionScope.At(1))
	clickID(t, driver, saveSessionConfirmID)
	stored, err := session.LoadNPP(xmlPath)
	if err != nil || len(stored.Main) != 1 || stored.Main[0].Encoding != 932 || stored.Main[0].StartPos != 2 || stored.Main[0].EndPos != 6 {
		t.Fatalf("exported code page and selection = %+v, %v", stored, err)
	}
	loaded := newModel()
	loadedDriver, err := app.NewDriver(newConfig(loaded), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer loadedDriver.Close()
	if err := loadedDriver.QueueOpenFiles([]string{xmlPath}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, loadedDriver, menuScope.At(0))
	clickID(t, loadedDriver, menuItemScope.At(13))
	clickID(t, loadedDriver, sessionScope.At(0))
	if got := loaded.space.Current(); got.Editor.String() != "日本\nabc" || got.Encoding() != document.ShiftJIS || got.Editor.Selection() != (text.Range{Start: 1, End: 3}) {
		t.Fatalf("imported code page and selection = %+v, notice=%q", got, loaded.notice)
	}
}
