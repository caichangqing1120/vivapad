package main

import (
	"bufio"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func bookmarkFixtureValue(t *testing.T, value string) string {
	t.Helper()
	if value == "-" {
		return ""
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(decoded)
}

func bookmarkFixtureMarks(t *testing.T, value string) []int {
	t.Helper()
	if value == "-" {
		return nil
	}
	var marked []int
	for _, raw := range strings.Split(value, ",") {
		line, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatal(err)
		}
		marked = append(marked, line)
	}
	return marked
}

func setLiveBookmarkMarks(t *testing.T, state *model, doc *document.Document, wanted []int) {
	t.Helper()
	current := slices.Clone(state.bookmarkLines(doc))
	for line := 0; line < doc.LineCount(); line++ {
		if slices.Contains(current, line) != slices.Contains(wanted, line) {
			doc.GoToLine(line + 1)
			state.toggleBookmark()
		}
	}
}

func TestApplicationBookmarkHistoryMatchesOriginalCellBuffer(t *testing.T) {
	input, err := os.Open("../../internal/linemarks/testdata/scintilla-marker-line-facts.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	var state *model
	var doc *document.Document
	active, checked := "", 0
	for scanner.Scan() {
		row := scanner.Text()
		if strings.HasPrefix(row, "#") || strings.HasPrefix(row, "case\t") {
			continue
		}
		fields := strings.Split(row, "\t")
		if len(fields) != 8 {
			t.Fatal("invalid original bookmark row")
		}
		before, after := bookmarkFixtureValue(t, fields[2]), bookmarkFixtureValue(t, fields[3])
		if fields[1] == "forward" {
			state = newModel()
			doc = state.space.Current()
			doc.Editor.Insert(before)
			doc.Editor.ClearHistory()
			active = fields[0]
		}
		if active != fields[0] || doc.Editor.String() != before {
			t.Fatalf("%s/%s: incorrect starting document", fields[0], fields[1])
		}
		setLiveBookmarkMarks(t, state, doc, bookmarkFixtureMarks(t, fields[4]))
		switch fields[1] {
		case "forward":
			state.editWithBookmarkAnchor(doc, func() {
				if err := doc.Editor.Transact(func(draft *text.Editor) error {
					if fields[7] != "-" {
						for _, raw := range strings.Split(fields[7], ",") {
							parts := strings.Split(raw, ":")
							if len(parts) != 3 {
								t.Fatal("invalid original mutation")
							}
							start, startErr := strconv.Atoi(parts[0])
							removed, removedErr := strconv.Atoi(parts[1])
							if startErr != nil || removedErr != nil {
								t.Fatal("invalid original mutation range")
							}
							draft.ReplaceRange(text.Range{Start: draft.RuneIndex(start), End: draft.RuneIndex(start + removed)}, bookmarkFixtureValue(t, parts[2]))
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			})
		case "undo":
			state.command("Undo")
		case "redo":
			state.command("Redo")
		default:
			t.Fatal("unknown original history phase", fields[1])
		}
		got := state.bookmarkLines(doc)
		if doc.Editor.String() != after || !slices.Equal(got, bookmarkFixtureMarks(t, fields[6])) {
			t.Fatalf("%s/%s: text=%q markers=%v, want text=%q markers=%s", fields[0], fields[1], doc.Editor.String(), got, after, fields[6])
		}
		checked++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if checked != 5460 {
		t.Fatalf("checked %d original application bookmark states, want 5460", checked)
	}
	t.Logf("checked %d original states through application bookmark commands", checked)
}

func TestBookmarkMissingHistoryIsExplicitAndPreservesUnmappedMarks(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb")
	doc.GoToLine(2)
	state.toggleBookmark()
	for range text.EditJournalLimit + 1 {
		doc.Editor.ReplaceRange(text.Range{}, "\n")
	}
	if got := state.bookmarkLines(doc); len(got) != 0 || state.notice == "" || !slices.Equal(state.bookmarks[doc].lines, []int{1}) {
		t.Fatalf("missing journal was silently projected/discarded: visible=%v retained=%v notice=%q", got, state.bookmarks[doc].lines, state.notice)
	}
	state.command("Inverse Bookmarks")
	if !slices.Equal(state.bookmarks[doc].lines, []int{1}) {
		t.Fatal("inverse replaced unresolved markers")
	}
	state.markDialogOpen, state.markBookmarkLine = true, true
	state.query.Insert("a")
	state.markCurrentDocument()
	if !slices.Equal(state.bookmarks[doc].lines, []int{1}) {
		t.Fatal("Mark All replaced unresolved markers")
	}
	state.markInSelection = true
	doc.Editor.SetSelection(text.Range{Start: 0, End: 1})
	state.clearMarkDialog()
	if !slices.Equal(state.bookmarks[doc].lines, []int{1}) {
		t.Fatal("partial clear discarded unresolved markers")
	}
	state.command("Clear All Bookmarks")
	if strings.Contains(state.notice, "unavailable") {
		t.Fatal("explicit reset left a stale missing-history error")
	}
	doc.GoToLine(1)
	state.toggleBookmark()
	if got := state.bookmarkLines(doc); !slices.Equal(got, []int{0}) {
		t.Fatalf("explicit reset did not start fresh tracking: %v", got)
	}
}

func TestMoveLinesUsesOriginalDeleteThenInsertMarkerEffects(t *testing.T) {
	for _, scenario := range []struct {
		name                   string
		up                     bool
		marked, selected, want []int
	}{
		{"up merges without carrying marker text", true, []int{1}, []int{1}, []int{2}},
		{"down leaves source marker on surviving line", false, []int{0}, []int{0}, []int{0}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert("same\nsame\nend")
			setLiveBookmarkMarks(t, state, doc, scenario.marked)
			doc.GoToLine(scenario.selected[0] + 1)
			command := components.CodeEditorMoveLinesDown
			if scenario.up {
				command = components.CodeEditorMoveLinesUp
			}
			state.applyCodeCommand(doc, command)
			if doc.Editor.String() != "same\nsame\nend" || !slices.Equal(state.bookmarkLines(doc), scenario.want) {
				t.Fatalf("move result=%q markers=%v; want %v", doc.Editor.String(), state.bookmarkLines(doc), scenario.want)
			}
		})
	}
}

func TestMoveLinesOriginalTerminalLFStepsDoNotRestoreMergedSets(t *testing.T) {
	for _, scenario := range []struct {
		name, before, after string
		line                int
		up                  bool
		marks, want, undo   []int
	}{
		{"down across unterminated EOF", "a\nb", "b\na", 1, false, []int{0, 1}, []int{0}, []int{0}},
		{"up from unterminated EOF", "a\nb", "b\na", 2, true, []int{0, 1}, []int{0}, []int{0}},
		{"up from empty terminal line", "a\n", "\na", 2, true, []int{1}, []int{1}, []int{0}},
		{"Unicode down across EOF", "\u732b\n\u72ac", "\u72ac\n\u732b", 1, false, []int{0, 1}, []int{0}, []int{0}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := newModel()
			doc := state.space.Current()
			doc.Editor.Insert(scenario.before)
			doc.Editor.ClearHistory()
			setLiveBookmarkMarks(t, state, doc, scenario.marks)
			doc.GoToLine(scenario.line)
			state.moveLinesWithBookmarks(doc, scenario.up)
			if doc.Editor.String() != scenario.after || !slices.Equal(state.bookmarkLines(doc), scenario.want) {
				t.Fatalf("terminal move=%q markers=%v, want %q %v", doc.Editor.String(), state.bookmarkLines(doc), scenario.after, scenario.want)
			}
			state.command("Undo")
			if doc.Editor.String() != scenario.before || !slices.Equal(state.bookmarkLines(doc), scenario.undo) {
				t.Fatalf("terminal undo=%q markers=%v, want %q %v", doc.Editor.String(), state.bookmarkLines(doc), scenario.before, scenario.undo)
			}
		})
	}
}

func TestNPPSessionRefusesUnmappedBookmarkHistory(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor.Insert("a\nb")
	doc.GoToLine(2)
	state.toggleBookmark()
	for range text.EditJournalLimit + 1 {
		doc.Editor.ReplaceRange(text.Range{}, "\n")
	}
	if err := doc.Save(filepath.Join(t.TempDir(), "gap.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := state.nppSessionState(); err == nil || !strings.Contains(err.Error(), "Bookmark") {
		t.Fatalf("session silently exported unresolved markers: %v", err)
	}
}

func TestBookmarkHistoryLiveChangesAcrossCloneAndCapture(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	doc.Name = "history.txt"
	doc.Editor.Insert("alpha\nbeta\ngamma\ndelta")
	doc.Editor.ClearHistory()
	doc.GoToLine(2)
	state.toggleBookmark()
	state.command("Clone to Other View")
	state.editWithBookmarkAnchor(doc, func() {
		if err := doc.Editor.Transact(func(draft *text.Editor) error {
			draft.ReplaceRange(text.Range{Start: 5, End: 6}, "")
			draft.ReplaceRange(text.Range{Start: 2, End: 2}, "\n")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	doc.GoToLine(3)
	state.toggleBookmark()
	if !slices.Equal(state.bookmarkLines(doc), []int{0, 2}) || state.paneDocument(0) != doc || state.paneDocument(1) != doc {
		t.Fatalf("clone did not share live merged marks: %v", state.bookmarkLines(doc))
	}
	capture := func(name string, width, height int) {
		t.Helper()
		output := os.Getenv("VIVAPAD_BOOKMARK_HISTORY_SHOTS")
		if output == "" {
			return
		}
		if err := os.MkdirAll(output, 0755); err != nil {
			t.Fatal(err)
		}
		session, err := headless.Open(newConfig(state), nil, headless.Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		if err := session.Resize(width, height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("bookmark history layout: %+v", problems)
		}
		if err := session.Screenshot(filepath.Join(output, name)); err != nil {
			t.Fatal(err)
		}
	}
	capture("bookmark-history-merged-desktop.png", 1200, 760)
	state.command("Undo")
	if doc.Editor.String() != "alpha\nbeta\ngamma\ndelta" || !slices.Equal(state.bookmarkLines(doc), []int{0, 2}) {
		t.Fatalf("Undo revived old marker sets or lost live changes: %q %v", doc.Editor.String(), state.bookmarkLines(doc))
	}
	capture("bookmark-history-undo-compact.png", 640, 400)
	state.command("Redo")
	if !slices.Equal(state.bookmarkLines(doc), []int{0, 2}) {
		t.Fatalf("Redo lost live shared markers: %v", state.bookmarkLines(doc))
	}
}
