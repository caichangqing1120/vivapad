package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"vivapad/internal/document"
	"vivapad/internal/session"
)

func TestWindowCloseKeepsSettingsForConfirmedTab(t *testing.T) {
	const original = "func main() {\n    work()\n}\n"
	const prefix = "// edit\n"
	for _, tc := range []struct {
		name    string
		discard bool
		text    string
		folds   []int
	}{
		{name: "save", text: prefix + original, folds: []int{1}},
		{name: "discard", discard: true, text: original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			leftPath, rightPath := filepath.Join(root, "left.txt"), filepath.Join(root, "right.txt")
			for _, path := range []string{leftPath, rightPath} {
				if err := os.WriteFile(path, []byte(original), 0600); err != nil {
					t.Fatal(err)
				}
			}
			sessionPath := filepath.Join(root, "settings", "session.json")
			state := newStartupModel([]string{leftPath}, sessionPath)
			if err := state.space.OpenWithEncoding(rightPath, document.ShiftJIS); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			state.showInActivePane(doc)
			state.toggleSplit()
			state.command("Go")
			doc.Editor.MoveTo(0, false)
			doc.Editor.Insert(prefix)
			state.command("Fold All")
			state.command("Pin Tab")
			state.setTabColor("Apply Color 3")
			state.command("Read-Only on Current Document")
			if state.activePane != 1 || !doc.Dirty() || !state.isPinned(doc) || !state.readOnly[doc] || state.tabColors[doc] != 3 || state.languages[doc] != ".go" || !slices.Equal(state.foldHeaders(doc), []int{1}) {
				t.Fatalf("did not prepare the dirty right tab and its settings: %s", state.notice)
			}
			allowed := false
			newConfig(state).OnCloseRequest(func() { allowed = true })
			if allowed || state.pendingClose != state.documentIndex(doc) {
				t.Fatal("window close did not wait for the dirty right tab")
			}
			state.finishClose(tc.discard)
			if !allowed {
				t.Fatalf("confirmation did not allow close: %s", state.notice)
			}
			if data, err := os.ReadFile(rightPath); err != nil || string(data) != tc.text {
				t.Fatalf("saved bytes = %q, %v; want %q", data, err, tc.text)
			}
			reopened := newStartupModel(nil, sessionPath)
			right := reopened.paneDocument(1)
			if !reopened.splitEnabled || reopened.activePane != 1 || right.Path != doc.Path || reopened.paneOwner[right] != 1 || right.Encoding() != document.ShiftJIS || right.Editor.String() != tc.text {
				t.Fatalf("session lost the saved right tab: split=%v active=%d path=%q encoding=%v text=%q", reopened.splitEnabled, reopened.activePane, right.Path, right.Encoding(), right.Editor.String())
			}
			if !reopened.isPinned(right) || reopened.tabColors[right] != 3 || !reopened.readOnly[right] || reopened.languages[right] != ".go" {
				t.Fatalf("session lost confirmed tab settings: pin=%v color=%d read-only=%v language=%q", reopened.isPinned(right), reopened.tabColors[right], reopened.readOnly[right], reopened.languages[right])
			}
			if got := reopened.foldHeaders(right); !slices.Equal(got, tc.folds) {
				t.Fatalf("restored fold headers = %v; want %v", got, tc.folds)
			}
		})
	}
}

func TestWindowCloseCancelLeavesPriorSessionAfterConfirmingAnotherTab(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := session.Save(sessionPath, session.State{Paths: []string{second}, Active: second}); err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{first, second}, sessionPath)
	secondTab := state.space.Documents[1]
	for _, doc := range state.space.Documents {
		doc.Editor.Insert("edited")
	}
	allowed := false
	newConfig(state).OnCloseRequest(func() { allowed = true })
	if state.pendingClose != 0 {
		t.Fatal("close did not wait for the first dirty tab")
	}
	state.finishClose(true)
	if allowed || state.pendingClose != 0 || len(state.space.Documents) != 1 || state.space.Documents[0] != secondTab {
		t.Fatal("close did not wait for the second dirty tab")
	}
	state.cancelClose()
	if data, err := os.ReadFile(sessionPath); err != nil || !bytes.Equal(data, previous) {
		t.Fatalf("canceled close changed the previous session: %s, %v", data, err)
	}
	if allowed || state.pendingClose >= 0 || !state.space.Current().Dirty() {
		t.Fatal("cancel closed the window or discarded the remaining edit")
	}
}

func TestRepeatedWindowCloseKeepsOriginalSession(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("saved"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sessionPath := filepath.Join(root, "settings", "session.json")
	state := newStartupModel([]string{first, second}, sessionPath)
	left, right := state.space.Documents[0], state.space.Documents[1]
	state.space.Select(state.documentIndex(left))
	state.showInActivePane(left)
	state.command("Pin Tab")
	state.setTabColor("Apply Color 3")
	state.space.Select(state.documentIndex(right))
	state.toggleSplit()
	state.activatePane(0)
	left.Editor.Insert("draft left")
	right.Editor.Insert("draft right")
	if !state.splitEnabled || state.paneDocument(0) != left || state.paneDocument(1) != right || !state.isPinned(left) || state.tabColors[left] != 3 {
		t.Fatal("did not prepare the two dirty panes")
	}
	config := newConfig(state)
	firstAllowed, repeatedAllowed := 0, 0
	config.OnCloseRequest(func() { firstAllowed++ })
	if state.pendingClose != state.documentIndex(left) {
		t.Fatal("first close did not wait for the left tab")
	}
	state.finishClose(true)
	if firstAllowed != 0 || len(state.space.Documents) != 1 || state.pendingClose != state.documentIndex(right) || state.pendingClose < 0 {
		t.Fatal("close did not advance to the right tab")
	}
	config.OnCloseRequest(func() { repeatedAllowed++ })
	if firstAllowed != 0 || repeatedAllowed != 0 || state.pendingClose != state.documentIndex(right) {
		t.Fatal("repeat request bypassed the open confirmation")
	}
	state.finishClose(true)
	stored, err := session.Load(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stored.Paths, []string{left.Path, right.Path}) {
		t.Fatalf("repeat close lost the original session paths: %v", stored.Paths)
	}
	if firstAllowed != 1 || repeatedAllowed != 0 {
		t.Fatalf("close authorization calls: first=%d repeated=%d", firstAllowed, repeatedAllowed)
	}
	reopened := newStartupModel(nil, sessionPath)
	restoredLeft, restoredRight := reopened.paneDocument(0), reopened.paneDocument(1)
	if !reopened.splitEnabled || reopened.activePane != 0 || restoredLeft.Path != left.Path || restoredRight.Path != right.Path || reopened.paneOwner[restoredRight] != 1 || !reopened.isPinned(restoredLeft) || reopened.tabColors[restoredLeft] != 3 {
		t.Fatal("repeat close lost the original split or tab settings")
	}
	if restoredLeft.Editor.String() != "saved" || restoredRight.Editor.String() != "saved" {
		t.Fatal("discard changed the saved contents")
	}
}
