package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/recovery"
)

func TestMixedEOLRecoveryUndoRetainsDiskSeparators(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	privatePath := filepath.Join(root, "config", "session.json")
	original := []byte("a\r\nb\nc\rd")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, privatePath)
	state.space.Current().Editor.ReplaceRange(text.Range{Start: 2, End: 2}, "X")
	if err := recovery.Save(state.recoveryPath, state.currentRecovery()); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, privatePath).space.Current()
	if !restored.Editor.Undo() || restored.Editor.String() != "a\nb\nc\nd" {
		t.Fatal("recovery Undo did not restore disk text")
	}
	copyPath := filepath.Join(root, "undone.txt")
	if err := restored.SaveCopy(copyPath); err != nil {
		t.Fatal(err)
	}
	if disk, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(disk, original) {
		t.Fatalf("recovery Undo lost disk separators: %q, %v", disk, err)
	}
}

func TestMixedEOLRecoveryPreservesBreaksAfterUnsavedEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	privatePath := filepath.Join(root, "config", "session.json")
	if err := os.WriteFile(path, []byte("a\r\nb\nc"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, privatePath)
	doc := state.space.Current()
	doc.Editor.ReplaceRange(text.Range{Start: 2, End: 2}, "X")
	stored := state.currentRecovery()
	if len(stored.Tabs) != 1 || stored.Tabs[0].LineEndings != "WL" {
		t.Fatalf("recovery did not capture mixed EOL map: %+v", stored)
	}
	if err := recovery.Save(state.recoveryPath, stored); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, privatePath)
	current := restored.space.Current()
	if !current.MixedLineEndings() || current.Editor.String() != "a\nXb\nc" {
		t.Fatalf("mixed recovery = %q mixed=%v", current.Editor.String(), current.MixedLineEndings())
	}
	if err := current.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nXb\nc" {
		t.Fatalf("saved recovered EOLs = %q, %v", data, err)
	}
}

func TestMixedEOLConflictRecoveryKeepsBreaksInUntitledCopy(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	privatePath := filepath.Join(root, "config", "session.json")
	if err := os.WriteFile(path, []byte("a\r\nb\nc"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, privatePath)
	state.space.Current().Editor.ReplaceRange(text.Range{Start: 2, End: 2}, "X")
	if err := recovery.Save(state.recoveryPath, state.currentRecovery()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, privatePath)
	copy := restored.space.Current()
	if copy.Path != "" || !strings.HasPrefix(copy.Name, "Recovered ") || !copy.MixedLineEndings() {
		t.Fatalf("conflict recovery lost mixed map: %+v", copy)
	}
	copyPath := filepath.Join(root, "copy.txt")
	if err := copy.Save(copyPath); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(copyPath); err != nil || string(data) != "a\r\nXb\nc" {
		t.Fatalf("conflict copy changed breaks = %q, %v", data, err)
	}
}

func TestExplicitMixedEOLConversionSurvivesRecoveryBeforeSave(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.txt")
	privatePath := filepath.Join(root, "config", "session.json")
	if err := os.WriteFile(path, []byte("a\r\nb\nc"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, privatePath)
	state.space.Current().SetLineEnding(document.CRLF)
	stored := state.currentRecovery()
	if len(stored.Tabs) != 1 || stored.Tabs[0].LineEndings != "" {
		t.Fatalf("explicit conversion backup = %+v", stored)
	}
	if err := recovery.Save(state.recoveryPath, stored); err != nil {
		t.Fatal(err)
	}
	restored := newStartupModel(nil, privatePath)
	doc := restored.space.Current()
	if !doc.Dirty() || doc.MixedLineEndings() {
		t.Fatalf("explicit EOL conversion did not survive recovery: dirty=%v mixed=%v", doc.Dirty(), doc.MixedLineEndings())
	}
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "a\r\nb\r\nc" {
		t.Fatalf("recovered conversion saved %q, %v", data, err)
	}
}

func TestMixedEOLBytePositionsAndMarkedLineClipboard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("猫\r\na\nb\rc"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.showEOL = true
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	doc := state.space.Current()
	var labels []string
	for _, label := range driver.Snapshot().Texts {
		if label.Value == "CRLF" || label.Value == "LF" || label.Value == "CR" {
			labels = append(labels, label.Value)
		}
	}
	if strings.Join(labels, ",") != "CRLF,LF,CR" {
		t.Fatalf("visible mixed EOL markers = %v", labels)
	}
	if !doc.MixedLineEndings() || nppBytePosition(doc, 2) != 5 || nppBytePosition(doc, 4) != 7 || nppBytePosition(doc, 6) != 9 {
		t.Fatalf("NPP mixed byte positions = %d,%d,%d", nppBytePosition(doc, 2), nppBytePosition(doc, 4), nppBytePosition(doc, 6))
	}
	if got := runeFromNPPPosition(doc, 7); got != 4 {
		t.Fatalf("mixed byte-to-rune position = %d", got)
	}
	state.setBookmarkLines(doc, []int{0, 1, 2})
	state.copyBookmarkedLines(false)
	if got := driver.Clipboard(); got != "猫\r\na\nb\r" {
		t.Fatalf("mixed marked-line clipboard = %q", got)
	}
	if doc.LineEnding() != document.CRLF || doc.Editor.String() != "猫\na\nb\nc" {
		t.Fatal("mixed bookmark copy changed document")
	}
}

func TestCaptureMixedEOLWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_MIXED_EOL_SHOT")
	if output == "" {
		t.Skip("set VIVAPAD_MIXED_EOL_SHOT to capture a mixed-EOL UI")
	}
	path := filepath.Join(t.TempDir(), "mixed.txt")
	if err := os.WriteFile(path, []byte("first\r\nsecond\nthird\rfourth"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.showEOL = true
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if err := session.Screenshot(output); err != nil {
		t.Fatal(err)
	}
}
