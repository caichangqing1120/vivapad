package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestExtendedCaseMenuSourceOrderAndCompactReachability(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("jOHN'S hELLO"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(0, state.space.Current().Editor.RuneCount()); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(11))
	for index := range caseCommands {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == caseScope.At(index) {
				found = true
				if hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("case row %d clipped: %+v", index, hit.Bounds)
				}
			}
		}
		if !found {
			t.Fatalf("case row %d missing", index)
		}
	}
	clickID(t, driver, caseScope.At(2))
	if got := state.space.Current().Editor.String(); got != "John's Hello" {
		t.Fatalf("Proper Case menu = %q", got)
	}
	state.command("Undo")
	state.space.Current().Editor.SetSelection(text.Range{Start: 0, End: state.space.Current().Editor.RuneCount()})
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(11))
	clickID(t, driver, caseScope.At(3))
	if got := state.space.Current().Editor.String(); got != "JOHN'S HELLO" {
		t.Fatalf("Proper Case blend menu = %q", got)
	}
}

func TestSentenceCaseAffectsOnlyActiveSplitPaneAndReadOnlyBlocks(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left stays")
	right := state.space.NewDocument()
	right.Editor.Insert("hELLO. wORLD")
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	right.Editor.SetSelection(text.Range{Start: 0, End: right.Editor.RuneCount()})
	state.command("Sentence case")
	if left.Editor.String() != "left stays" || right.Editor.String() != "Hello. World" {
		t.Fatalf("split sentence case = left:%q right:%q", left.Editor.String(), right.Editor.String())
	}
	state.command("Undo")
	if right.Editor.String() != "hELLO. wORLD" {
		t.Fatal("sentence case did not undo")
	}
	right.Editor.SetSelection(text.Range{Start: 0, End: right.Editor.RuneCount()})
	state.command("Read-Only on Current Document")
	state.command("Sentence case (blend)")
	if right.Editor.String() != "hELLO. wORLD" || !strings.Contains(state.notice, "Read-only") {
		t.Fatalf("read-only sentence case = %q notice=%q", right.Editor.String(), state.notice)
	}
}

func TestRandomCaseMenuKeepsUnselectedTextAndDigits(t *testing.T) {
	state, driver := startDriver(t)
	clickID(t, driver, editorID)
	if err := driver.Type("pre Abc 123 post"); err != nil {
		t.Fatal(err)
	}
	if err := driver.SelectText(4, 7); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(11))
	clickID(t, driver, caseScope.At(7))
	got := state.space.Current().Editor.String()
	if !strings.HasPrefix(got, "pre ") || !strings.HasSuffix(got, " 123 post") || len([]rune(got)) != len([]rune("pre Abc 123 post")) {
		t.Fatalf("random case changed unrelated text: %q", got)
	}
}
