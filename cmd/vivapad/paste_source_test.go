package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestPasteHTMLSourceFromCompactMenu(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("left select right")
	doc.Editor.SetSelection(text.Range{Start: 5, End: 11})
	const html = "<b>literal &amp; \u6587</b>"
	driver.SetClipboardHTML(html, "fallback")
	parent := slices.Index(menuCommands[1], "Paste Special")
	if parent < 0 {
		t.Fatal("Edit lacks Paste Special")
	}
	clickID(t, driver, menuScope.At(1))
	for range menuCommands[1] {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			found = found || hit.ID == menuItemScope.At(parent)
		}
		if found {
			break
		}
		clickID(t, driver, editMenuNextID)
	}
	clickID(t, driver, menuItemScope.At(parent))
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Paste HTML Content" {
			clickID(t, driver, hit.ID)
			if got := doc.Editor.String(); got != "left "+html+" right" {
				t.Fatalf("literal paste: %q", got)
			}
			state.command("Undo")
			if got := doc.Editor.String(); got != "left select right" {
				t.Fatalf("one undo: %q", got)
			}
			return
		}
	}
	t.Fatal("Paste HTML Content is missing from the compact menu")
}

func TestPasteSourceRefusalPreservesSelectionAndHistory(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("before")
	selection := text.Range{Start: 1, End: 4}
	doc.Editor.SetSelection(selection)
	for _, fixture := range []struct {
		value  string
		notice string
	}{{"", "requested source format"}, {"\xff", "invalid text"}, {"a\x00b", "invalid text"}, {strings.Repeat("x", maxClipboardSourceSize+1), "byte limit"}} {
		driver.SetClipboardHTML(fixture.value, "plain text")
		state.command("Paste HTML Content")
		if doc.Editor.String() != "before" || doc.Editor.Selection() != selection || !strings.Contains(state.notice, fixture.notice) {
			t.Fatalf("source of %d bytes: text=%q, selection=%v, notice=%q", len(fixture.value), doc.Editor.String(), doc.Editor.Selection(), state.notice)
		}
	}
	state.command("Undo")
	if got := doc.Editor.String(); got != "" {
		t.Fatalf("failed commands changed the undo stack: %q", got)
	}
	state.command("Paste")
	if got := doc.Editor.String(); got != "plain text" {
		t.Fatalf("ordinary paste: %q", got)
	}
}

func TestPasteRTFSourceInTheActiveSplit(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("right")
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.splitEnabled = true
	state.activatePane(1)
	const rtf = `{\rtf1\ansi \u25991?}`
	driver.SetClipboardRTF(rtf, "readable text")
	state.readOnly[right] = true
	state.command("Paste RTF Content")
	if got := right.Editor.String(); got != "right" || state.notice != "Read-only document" {
		t.Fatalf("read-only pane: %q, %q", got, state.notice)
	}
	delete(state.readOnly, right)
	right.Editor.SetSelection(text.Range{Start: 0, End: 5})
	state.command("Paste RTF Content")
	if right.Editor.String() != rtf || left.Editor.String() != "left" {
		t.Fatalf("active pane: left=%q, right=%q", left.Editor.String(), right.Editor.String())
	}
	state.command("Undo")
	if got := right.Editor.String(); got != "right" {
		t.Fatalf("one undo: %q", got)
	}
}

func TestMissingClipboardSourceDoesNotCommitOrCancelPreedit(t *testing.T) {
	state, doc, input := liveCompositionState(t)
	driver, err := app.NewDriver(app.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	driver.SetClipboard("plain")
	state.wordCompletion = nil
	before := captureComposition(state, doc)
	state.command("Paste Special")
	state.command("Paste RTF Content")
	if after := captureComposition(state, doc); !reflect.DeepEqual(after, before) {
		t.Fatalf("failed paste changed state: before %+v, after %+v", before, after)
	}
	sendComposition(t, input, text.CompositionCancel, "")
	if got := doc.Editor.String(); got != "seed" {
		t.Fatalf("cancel after failed paste: %q", got)
	}
}
