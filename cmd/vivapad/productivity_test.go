package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/compare"
	"vivapad/internal/document"
	"vivapad/internal/preferences"
)

func clickProductivity(t *testing.T, driver *app.Driver, menu int, parent, command string) {
	t.Helper()
	clickID(t, driver, menuScope.At(menu))
	clickID(t, driver, menuItemScope.At(slices.Index(menuCommands[menu], parent)))
	clickMenuLabel(t, driver, command)
}

func TestProductivityJSONDriverUndoRedoAndEOL(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n"} {
		t.Run(fmt.Sprintf("EOL %q", ending), func(t *testing.T) {
			state, driver := startDriver(t)
			path := filepath.Join(t.TempDir(), "source.json")
			const raw = `{"z":900719925474099312345,"a":"<>&"}`
			source := "prefix " + raw + ending + "untouched" + ending
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := state.space.Open(path); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			state.showInActivePane(doc)
			doc.Editor.SetSelection(text.Range{Start: 7, End: 7 + len(raw)})
			selection := doc.Editor.Selection()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			clickProductivity(t, driver, 7, structuredToolsMenu, "Sort JSON Keys")
			want := strings.Join([]string{`prefix {`, `  "a": "<>&",`, `  "z": 900719925474099312345`, `}`, `untouched`, ``}, ending)
			if got, err := doc.TextRangeWithLineEndings(text.Range{End: doc.Editor.RuneCount()}); err != nil || got != want {
				t.Fatalf("formatted text = %q, %v; want %q (%s)", got, err, want, state.notice)
			}
			state.command("Undo")
			if got, err := doc.TextRangeWithLineEndings(text.Range{End: doc.Editor.RuneCount()}); err != nil || got != source || doc.Editor.CanUndo() || doc.Dirty() || doc.Editor.Selection() != selection {
				t.Fatalf("one Undo = %q, %v, dirty=%v, selection=%v", got, err, doc.Dirty(), doc.Editor.Selection())
			}
			state.command("Redo")
			if got, err := doc.TextRangeWithLineEndings(text.Range{End: doc.Editor.RuneCount()}); err != nil || got != want || doc.Editor.CanRedo() {
				t.Fatalf("one Redo = %q, %v", got, err)
			}
			if saved, err := os.ReadFile(path); err != nil || string(saved) != source {
				t.Fatalf("conversion wrote disk: %q, %v", saved, err)
			}
		})
	}
}

func TestProductivityErrorsAndLargeReadOnlyProtection(t *testing.T) {
	state, driver := startDriver(t)
	const invalid = "{\n  \"key\": ]\n}"
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	before := doc.Editor.Selection()
	clickProductivity(t, driver, 7, structuredToolsMenu, "Format JSON (2 Spaces)")
	if doc.Editor.String() != invalid || doc.Editor.Selection() != before || doc.Dirty() || doc.Editor.CanUndo() || !strings.Contains(state.notice, "line 2, column 10") {
		t.Fatalf("failed conversion changed document or lost diagnostics: %q / %s", doc.Editor.String(), state.notice)
	}
	doc.Editor.Insert("[]")
	state.readOnly[doc] = true
	clickProductivity(t, driver, 7, encodingToolsMenu, "URL Encode")
	if doc.Editor.String() != "[]"+invalid || state.notice != "Read-only document" {
		t.Fatalf("read-only conversion: %q, %s", doc.Editor.String(), state.notice)
	}
	oldLight, oldLarge := document.LightweightThreshold, document.LargeFileThreshold
	document.LightweightThreshold, document.LargeFileThreshold = 64, 128
	t.Cleanup(func() { document.LightweightThreshold, document.LargeFileThreshold = oldLight, oldLarge })
	for _, size := range []int{96, 192} {
		path := filepath.Join(t.TempDir(), "bigger.txt")
		if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0600); err != nil {
			t.Fatal(err)
		}
		if err := state.space.Open(path); err != nil {
			t.Fatal(err)
		}
		big := state.space.Current()
		state.showInActivePane(big)
		state.command("Base64 Encode")
		if big.Dirty() || big.Editor.CanUndo() || !strings.Contains(strings.ToLower(state.notice), "large") {
			t.Fatalf("size %d did not block tools: %s", size, state.notice)
		}
		state.command(compareSavedCommand)
		if state.comparison != nil || !strings.Contains(state.notice, "large") {
			t.Fatalf("size %d did not block compare: %s", size, state.notice)
		}
		if large := big.LargeFile(); large != nil {
			t.Cleanup(func() { _ = large.Close() })
		}
	}
}

func TestProductivityEncodingAndTimestampTargets(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.SetLineEnding(document.CRLF)
	doc.Editor.Insert("before\na\n中\nafter")
	doc.Editor.SetSelection(text.Range{Start: 7, End: 10})
	clickProductivity(t, driver, 7, encodingToolsMenu, "Base64 Encode")
	if got := doc.Editor.String(); got != "before\nYQ0K5Lit\nafter" {
		t.Fatalf("Base64 converted wrong input: %q", got)
	}
	doc.Editor.SetSelection(text.Range{Start: 7, End: 15})
	state.command("Base64 Decode")
	if got, err := doc.TextRangeWithLineEndings(text.Range{End: doc.Editor.RuneCount()}); err != nil || got != "before\r\na\r\n中\r\nafter" {
		t.Fatalf("Base64 round trip = %q, %v", got, err)
	}
	doc.Editor.SetSelection(text.Range{End: doc.Editor.RuneCount()})
	doc.Editor.Insert("event=1700000000; ok\n<foo>\n")
	doc.Editor.MoveTo(10, false)
	clickProductivity(t, driver, 7, encodingToolsMenu, timestampCommand)
	want := "event=" + time.Unix(1700000000, 0).In(time.Local).Format("2006-01-02 15:04:05") + "; ok\n<foo>\n"
	if got := doc.Editor.String(); got != want {
		t.Fatalf("timestamp transformed wrong range: %q, %s", got, state.notice)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount()-3, false)
	state.command("HTML Escape")
	if !strings.HasSuffix(doc.Editor.String(), "\n&lt;foo&gt;\n") {
		t.Fatalf("HTML line fallback = %q", doc.Editor.String())
	}
}

func TestProductivityCompareDriverNavigationThemeAndSync(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	right := state.space.NewDocument()
	var lhs, rhs strings.Builder
	for line := range 100 {
		if line == 2 {
			lhs.WriteString("old value\n")
			rhs.WriteString("new value\n")
		}
		if line == 5 {
			lhs.WriteString(strings.Repeat("deleted line\n", 8))
		}
		if line == 9 {
			rhs.WriteString("added line\n")
		}
		if line == 68 {
			lhs.WriteString("old tail\n")
			rhs.WriteString("new tail\n")
		}
		fmt.Fprintf(&lhs, "line %03d\n", line)
		fmt.Fprintf(&rhs, "line %03d\n", line)
	}
	left.Editor.Insert(lhs.String())
	right.Editor.Insert(rhs.String())
	left.Editor.MoveTo(0, false)
	right.Editor.MoveTo(0, false)
	state.command("Split View")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickProductivity(t, driver, 7, compareMenu, compareSplitCommand)
	view := state.comparison
	if view == nil || len(view.result.Hunks) != 4 || view.docs != [2]*document.Document{left, right} {
		t.Fatalf("compare not started: %s", state.notice)
	}
	for _, choice := range []preferences.Theme{preferences.ClassicLight, preferences.ModernDark, preferences.Monokai} {
		state.themeChoice = choice
		if err := driver.Resize(850, 550); err != nil {
			t.Fatal(err)
		}
		assertCompareColors(t, driver, state.palette().diff)
	}
	clickProductivity(t, driver, 3, compareMenu, nextDiffCommand)
	if view.current != 0 || left.Editor.ByteOffset(left.Editor.Caret()) != view.offset(0, 2) || right.Editor.ByteOffset(right.Editor.Caret()) != view.offset(1, 2) {
		t.Fatalf("menu next did not move both carets: %s", state.notice)
	}
	if err := driver.Key(gfx.KeyF8, primary()|gfx.ModifierShift); err != nil || view.current != 3 {
		t.Fatalf("previous shortcut did not wrap: %v / %s", err, state.notice)
	}
	if err := driver.Key(gfx.KeyF8, primary()); err != nil || view.current != 0 {
		t.Fatalf("next shortcut did not wrap: %v / %s", err, state.notice)
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: 125, Y: 260, DeltaY: 620, Precise: true}); err != nil {
		t.Fatal(err)
	}
	main, sub := state.codeEditorForPane(0), state.codeEditorForPane(1)
	leftLine, _ := main.VisibleLines()
	rightLine, _ := sub.VisibleLines()
	if leftLine <= 16 || rightLine != view.result.PeerLine(0, leftLine) || state.syncVertical {
		t.Fatalf("compare scroll = %d / %d with ordinary sync=%v", leftLine, rightLine, state.syncVertical)
	}
	clickProductivity(t, driver, 7, compareMenu, clearCompareCommand)
	if state.comparison != nil || !state.splitEnabled || state.paneDocs != [2]*document.Document{left, right} || left.Editor.String() != lhs.String() || right.Editor.String() != rhs.String() {
		t.Fatal("Clear Compare changed normal split documents")
	}
}

func assertCompareColors(t *testing.T, driver *app.Driver, colors [3]gfx.Color) {
	t.Helper()
	var seen [3]bool
	for _, primitive := range driver.Snapshot().Primitives {
		for index, color := range colors {
			if primitive.Color == color {
				seen[index] = true
			}
		}
	}
	if seen != [3]bool{true, true, true} {
		t.Fatalf("diff colors drawn: %v", seen)
	}
}

func TestProductivityCompareSavedPreviewIsTransient(t *testing.T) {
	state, driver := startDriver(t)
	path := filepath.Join(t.TempDir(), "saved.txt")
	const disk = "same\r\nsaved\r\n"
	if err := os.WriteFile(path, []byte(disk), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	doc.Editor.SetSelection(text.Range{End: doc.Editor.RuneCount()})
	doc.Editor.Insert("same\ndraft\n")
	tabs := len(state.space.Documents)
	clickProductivity(t, driver, 7, compareMenu, compareSavedCommand)
	view := state.comparison
	if view == nil || !view.saved || view.values[view.savedPane] != "same\nsaved\n" || !slices.Equal(view.result.Hunks, []compare.Hunk{{LeftStart: 1, LeftEnd: 2, RightStart: 1, RightEnd: 2}}) {
		t.Fatalf("saved diff not displayed: %s", state.notice)
	}
	assertEditorHitReadOnly(t, driver, comparePreviewID, true)
	clickID(t, driver, comparePreviewID)
	_ = driver.Type("must not edit")
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if doc.Editor.String() != "same\ndraft\n" || len(state.space.Documents) != tabs || state.splitEnabled {
		t.Fatal("preview affected the live document or split state")
	}
	if current, err := os.ReadFile(path); err != nil || string(current) != disk {
		t.Fatalf("disk snapshot changed: %q, %v", current, err)
	}
	clickProductivity(t, driver, 7, compareMenu, clearCompareCommand)
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == comparePreviewID {
			t.Fatal("saved snapshot still on screen after Clear Compare")
		}
	}
	if state.space.Current() != doc || !doc.Dirty() || !doc.Editor.CanUndo() {
		t.Fatal("clearing snapshot did not preserve current draft")
	}
}
