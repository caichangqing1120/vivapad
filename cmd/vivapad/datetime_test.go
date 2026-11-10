package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/datetime"
	"vivapad/internal/document"
)

func TestDateTimeInsertMenuAtCompactSizeAndOneUndo(t *testing.T) {
	state, driver := startDriver(t)
	state.clock = func() time.Time { return time.Date(2026, 11, 6, 15, 4, 5, 0, time.UTC) }
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("preXXpost"); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	if err := driver.SelectText(3, 5); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(20) && hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
			t.Fatalf("Edit Insert item overlaps status: %+v", hit.Bounds)
		}
	}
	clickID(t, driver, menuItemScope.At(20))
	for index := range insertDateCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == insertDateScope.At(index) {
				found = true
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("date-time command clipped: %+v", hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("date-time command %d missing", index)
		}
	}
	clickID(t, driver, insertDateScope.At(2))
	if got := doc.Editor.String(); got != "pre2026-11-06 15:04:05post" {
		t.Fatalf("customized insertion = %q", got)
	}
	state.command("Undo")
	if got := doc.Editor.String(); got != "preXXpost" {
		t.Fatalf("time insertion undo = %q", got)
	}
	state.command("Read-Only on Current Document")
	state.command("Date Time (short)")
	if doc.Editor.String() != "preXXpost" || state.notice != "Read-only document" {
		t.Fatalf("read-only time insertion = %q notice=%q", doc.Editor.String(), state.notice)
	}
}

func TestDateTimeSettingsPersistAndCancelInvalidInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "session.json")
	state := newStartupModel(nil, path)
	state.clock = func() time.Time { return time.Date(2026, 11, 6, 15, 4, 5, 0, time.UTC) }
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(6))
	clickID(t, driver, menuItemScope.At(9))
	if !state.dateTimeDialogOpen {
		t.Fatal("date-time settings dialog did not open")
	}
	clickID(t, driver, newID)
	if len(state.space.Documents) != 1 {
		t.Fatal("underlying New button received a modal click")
	}
	clickID(t, driver, dateTimeFieldID)
	if _, err := driver.ShortcutChord(primary(), 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("yyyy 'invalid"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, dateTimeApplyID)
	if !state.dateTimeDialogOpen || state.dateTimeFormat != datetime.DefaultFormat || state.dateTimeError == "" {
		t.Fatalf("invalid settings applied: format=%q error=%q", state.dateTimeFormat, state.dateTimeError)
	}
	clickID(t, driver, dateTimeFieldID)
	if _, err := driver.ShortcutChord(primary(), 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("dd/MM/yyyy HH:mm"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, dateTimeReverseID)
	checked := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == dateTimeReverseID && hit.Role == ui.RoleCheckbox && hit.Checked {
			checked = true
		}
	}
	if !checked {
		t.Fatal("date-before-time control did not expose checked checkbox state")
	}
	clickID(t, driver, dateTimeApplyID)
	if state.dateTimeDialogOpen || state.dateTimeFormat != "dd/MM/yyyy HH:mm" || !state.dateTimeReverse {
		t.Fatalf("settings not applied: format=%q reverse=%v", state.dateTimeFormat, state.dateTimeReverse)
	}
	restored := newStartupModel(nil, path)
	if restored.dateTimeFormat != state.dateTimeFormat || !restored.dateTimeReverse {
		t.Fatalf("date-time settings not restored: %q reverse=%v", restored.dateTimeFormat, restored.dateTimeReverse)
	}
	state.command("Date Time (customized)")
	if state.space.Current().Editor.String() != "06/11/2026 15:04" {
		t.Fatalf("customized format after settings = %q", state.space.Current().Editor.String())
	}
	state.command("Date Time Format...")
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if state.dateTimeDialogOpen || state.dateTimeFormat != "dd/MM/yyyy HH:mm" || !state.dateTimeReverse {
		t.Fatal("Escape changed the persisted settings")
	}
}

func TestDateTimeCommandsUseActiveSplitPaneAndDoNotLoseBookmarks(t *testing.T) {
	state, driver := startDriver(t)
	state.clock = func() time.Time { return time.Date(2026, 11, 6, 15, 4, 5, 0, time.UTC) }
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("first\nvalue\n")
	right.Editor.MoveTo(6, false)
	state.toggleBookmark()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[right] = 1
	clickID(t, driver, rightEditorID)
	right.Editor.SetSelection(text.Range{Start: 6, End: 11})
	state.command("Date Time (long)")
	if state.activePane != 1 || left.Editor.String() != "left" || !strings.HasPrefix(right.Editor.String(), "first\n") || strings.Contains(right.Editor.String(), "value") {
		t.Fatalf("split time insert = left:%q right:%q pane:%d", left.Editor.String(), right.Editor.String(), state.activePane)
	}
	if marks := state.bookmarkLines(right); len(marks) != 1 || marks[0] != 1 {
		t.Fatalf("time insertion moved bookmark: %v", marks)
	}
}

func TestDateTimeInsertionPreservesLegacyEncodingAndCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.txt")
	if err := os.WriteFile(path, []byte("caf\xe9\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state, _ := startDriver(t)
	state.clock = func() time.Time { return time.Date(2026, 11, 6, 15, 4, 5, 0, time.UTC) }
	if err := state.space.OpenWithEncoding(path, document.Windows1252); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	state.command("Date Time (customized)")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "caf\xe9\r\n2026-11-06 15:04:05" {
		t.Fatalf("legacy saved time = %q, %v", data, err)
	}
	doc.Editor.ClearHistory()
	state.dateTimeFormat = "yyyy 'incomplete"
	state.command("Date Time (customized)")
	if doc.Dirty() || doc.Editor.String() != "café\n2026-11-06 15:04:05" || !strings.Contains(state.notice, "format") {
		t.Fatalf("invalid insertion changed document: %q notice=%q", doc.Editor.String(), state.notice)
	}
}
