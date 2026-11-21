package main

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"

	"vivapad/internal/document"
)

func TestLocaleAndRandomSortCommandsReachableAtCompactSize(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("item10\nitem2\nitem1\n")
	clickID(t, driver, menuScope.At(1))
	clickID(t, driver, menuItemScope.At(10))
	for step := 0; step < len(lineCommands); step++ {
		found := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == lineScope.At(len(lineCommands)-1) {
				found = true
			}
		}
		if found {
			break
		}
		clickID(t, driver, lineNextID)
	}
	for _, index := range []int{len(lineCommands) - 3, len(lineCommands) - 2, len(lineCommands) - 1} {
		seen := false
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == lineScope.At(index) {
				seen = true
				if hit.Bounds.Y < menuHeight || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("new sort row clipped: %+v", hit.Bounds)
				}
			}
		}
		if !seen {
			t.Fatalf("sort menu item %d unavailable", index)
		}
	}
	clickID(t, driver, lineScope.At(len(lineCommands)-3))
	if got := doc.Editor.String(); got != "item1\nitem2\nitem10\n" {
		t.Fatalf("locale sort menu = %q", got)
	}
	state.command("Read-Only on Current Document")
	state.command("Randomize Line Order")
	if doc.Editor.String() != "item1\nitem2\nitem10\n" || state.notice != "Read-only document" {
		t.Fatalf("read-only random sort = %q notice=%q", doc.Editor.String(), state.notice)
	}
}

func TestLocaleSortClearsSourceWholeBufferMarkersAndKeepsActiveView(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("item10\nitem2\nitem1\n")
	right.Editor.ClearHistory()
	right.Editor.MoveTo(0, false)
	state.toggleBookmark()
	clickID(t, driver, menuScope.At(3))
	clickID(t, driver, menuItemScope.At(5))
	state.paneDocs[0], state.paneDocs[1] = left, right
	state.paneOwner[right] = 1
	clickID(t, driver, rightEditorID)
	right.Editor.SetSelection(text.Range{Start: 0, End: right.Editor.RuneCount()})
	state.command("Sort In Locale Order Ascending")
	if left.Editor.String() != "left" || right.Editor.String() != "item1\nitem2\nitem10\n" || state.activePane != 1 {
		t.Fatalf("split locale sort = left:%q right:%q pane:%d", left.Editor.String(), right.Editor.String(), state.activePane)
	}
	if marks := state.bookmarkLines(right); len(marks) != 0 {
		t.Fatalf("locale sort bookmark = %v", marks)
	}
	state.command("Undo")
	if right.Editor.String() != "item10\nitem2\nitem1\n" || len(state.bookmarkLines(right)) != 0 {
		t.Fatalf("locale undo = %q marks=%v", right.Editor.String(), state.bookmarkLines(right))
	}
	state.command("Sort In Locale Order Descending")
	if right.Editor.String() != "item10\nitem2\nitem1\n" || !right.Editor.CanUndo() {
		t.Fatalf("already descending locale sort = %q", right.Editor.String())
	}
}

func TestRandomSortCommandPreservesLineMultiset(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo\nthree\nfour\nfive\n")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(len([]rune("one\ntwo\n")), false)
	state.toggleBookmark()
	beforeValue := doc.Editor.String()
	before := strings.Split(beforeValue, "\n")
	state.command("Randomize Line Order")
	after := strings.Split(doc.Editor.String(), "\n")
	slices.Sort(before)
	slices.Sort(after)
	if !slices.Equal(before, after) || state.notice == "" {
		t.Fatalf("random sort lines=%v notice=%q", after, state.notice)
	}
	if marks := state.bookmarkLines(doc); len(marks) != 0 {
		t.Fatalf("random sort did not follow source whole-buffer marker policy: %v", marks)
	}
	if doc.Editor.CanUndo() {
		state.command("Undo")
		if doc.Editor.String() != "one\ntwo\nthree\nfour\nfive\n" || len(state.bookmarkLines(doc)) != 0 {
			t.Fatalf("random undo = %q, marks=%v", doc.Editor.String(), state.bookmarkLines(doc))
		}
	}
}

// TestRandomizeLineOrderDropsBookmarksWhenOrderIsUnchanged pins the source
// whole-buffer marker policy on the identity permutation, the path the
// randomize command takes when the shuffled order matches the source order.
// The seed is brute-forced against the real document entry point, so the test
// is reproducible instead of depending on the wall clock.
func TestRandomizeLineOrderDropsBookmarksWhenOrderIsUnchanged(t *testing.T) {
	state, _ := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("one\ntwo\nthree\nfour\nfive\n")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(len([]rune("one\ntwo\n")), false)
	state.toggleBookmark()
	if len(state.bookmarkLines(doc)) != 1 {
		t.Fatalf("precondition: bookmarks = %v", state.bookmarkLines(doc))
	}
	identity := uint64(0)
	for candidate := uint64(1); candidate < 20000 && identity == 0; candidate++ {
		probe := document.New("probe")
		probe.Editor.Insert("one\ntwo\nthree\nfour\nfive\n")
		if !probe.RandomizeLines(rand.New(rand.NewPCG(candidate, 1))) {
			identity = candidate
		}
	}
	if identity == 0 {
		t.Fatal("no identity seed found")
	}
	changed, err := state.randomizeLineOrder(doc, identity)
	if err != nil {
		t.Fatalf("identity randomize error: %v", err)
	}
	if changed {
		t.Fatalf("identity seed %d reported a change", identity)
	}
	if marks := state.bookmarkLines(doc); len(marks) != 0 {
		t.Fatalf("identity randomize kept bookmarks %v", marks)
	}
	if doc.Editor.String() != "one\ntwo\nthree\nfour\nfive\n" {
		t.Fatalf("identity randomize changed the text: %q", doc.Editor.String())
	}
}

// identitySeedFor finds a seed whose shuffle of lines reproduces the input, so
// identity-permutation paths are exercised deterministically.
func identitySeedFor(t *testing.T, lines []string) uint64 {
	t.Helper()
	for candidate := uint64(1); candidate < 20000; candidate++ {
		shuffled := append([]string(nil), lines...)
		random := rand.New(rand.NewPCG(candidate, 1))
		random.Shuffle(len(shuffled), func(first, second int) { shuffled[first], shuffled[second] = shuffled[second], shuffled[first] })
		if slices.Equal(lines, shuffled) {
			return candidate
		}
	}
	t.Fatal("no identity seed found")
	return 0
}

// TestRandomizeLineOrderScopesIdentityMarkerPolicy pins the target-scoped marker
// policy: when a randomize reproduces the source order inside an eligible block,
// only bookmarks in that block are dropped; bookmarks outside it survive, and an
// ineligible single-line document keeps its bookmark and records no undo unit.
func TestRandomizeLineOrderScopesIdentityMarkerPolicy(t *testing.T) {
	t.Run("selected block keeps unselected bookmarks", func(t *testing.T) {
		state, _ := startDriver(t)
		doc := state.space.Current()
		doc.Editor.Insert("head\nsame\nsame\ntail\n")
		doc.Editor.ClearHistory()
		seed := identitySeedFor(t, []string{"same", "same"})
		// bookmarks on the first, both selected and the last line
		doc.Editor.MoveTo(0, false)
		state.toggleBookmark()
		doc.Editor.MoveTo(len([]rune("head\n")), false)
		state.toggleBookmark()
		doc.Editor.MoveTo(len([]rune("head\nsame\nsame\n")), false)
		state.toggleBookmark()
		doc.Editor.SetSelection(text.Range{Start: len([]rune("head\n")), End: len([]rune("head\nsame\nsame"))})
		if got := state.bookmarkLines(doc); !slices.Equal(got, []int{0, 1, 3}) {
			t.Fatalf("precondition bookmarks = %v", got)
		}
		if changed, err := state.randomizeLineOrder(doc, seed); changed || err != nil {
			t.Fatalf("identity seed %d changed=%v err=%v", seed, changed, err)
		}
		if got := state.bookmarkLines(doc); !slices.Equal(got, []int{0, 3}) {
			t.Fatalf("identity randomize bookmarks = %v, want the unselected lines 0 and 3", got)
		}
		if doc.Editor.CanUndo() {
			t.Fatal("identity randomize recorded an undo unit")
		}
	})

	// A selection spanning two empty lines is normalized to a single line by the
	// editor before the command runs, so that reviewed scenario is not reachable
	// through the public selection API. randomizeLineOrder still captures the
	// target range before the operation, so a future normalization cannot shrink
	// the marker scope.
	t.Run("single line is ineligible and keeps its bookmark", func(t *testing.T) {
		state, _ := startDriver(t)
		doc := state.space.Current()
		doc.Editor.Insert("only\n")
		doc.Editor.ClearHistory()
		doc.Editor.MoveTo(0, false)
		state.toggleBookmark()
		if got := state.bookmarkLines(doc); !slices.Equal(got, []int{0}) {
			t.Fatalf("precondition bookmarks = %v", got)
		}
		if changed, err := state.randomizeLineOrder(doc, 7); changed || err != nil {
			t.Fatalf("single-line randomize changed=%v err=%v", changed, err)
		}
		if got := state.bookmarkLines(doc); !slices.Equal(got, []int{0}) {
			t.Fatalf("ineligible randomize dropped the bookmark: %v", got)
		}
		if doc.Editor.CanUndo() {
			t.Fatal("single-line randomize recorded an undo unit")
		}
	})
}
