package changehistory

// H02 consumer oracle tests.
//
// Two independent oracles are used:
//  1. The REAL producer: ordinary Viva editor entry points publish frames. The
//     expected Tracker is built by hand from the source predicates in
//     docs/specs/001-vivapad/change-history-editor-contract.md; the consumer's
//     edition/run/deletion/line answers must equal that hand-built reference.
//  2. The ORIGINAL fixtures: each fixture state's source event sequence is
//     encoded back into the HistoryStep/HistoryFrame stream the mapping must
//     consume (the inverse direction), then the consumer's frame validation and
//     mapping are run and every edition/run/deletion column of the fixture is
//     compared. The fixture rows, hashes and expected values are untouched.
//
// Honest boundary: the real-producer part derives expected events by hand; the
// fixture part constructs witness HistoryContext values whose inverse yields the
// fixture's recorded events. It does not claim the original source's private
// p/s/d values, only that the mapping reproduces the original event facts.

import (
	"encoding/hex"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func evInsert(position, length int, collecting, beforeSave bool) Event {
	return Event{Kind: Insert, Position: position, Length: length, CollectingUndo: collecting, BeforeSave: beforeSave}
}

func evDelete(position, length int, beforeSave, detached bool) Event {
	return Event{Kind: Delete, Position: position, Length: length, BeforeSave: beforeSave, Detached: detached}
}

func evUndoInsert(position, length int, reverting bool) Event {
	return Event{Kind: UndoInsert, Position: position, Length: length, Reverting: reverting}
}

func evUndoDelete(position, length int, detached bool) Event {
	return Event{Kind: UndoDelete, Position: position, Length: length, Detached: detached}
}

var (
	evSave  = Event{Kind: Save}
	evStart = Event{Kind: StartReversion}
	evEnd   = Event{Kind: EndReversion}
)

// TestConsumerMatchesActualProducerAndOriginal is the H02 acceptance oracle.
func TestConsumerMatchesActualProducerAndOriginal(t *testing.T) {
	t.Run("typing save undo redo", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, 3)
		checkConsumerMatchesTracker(t, consumer, editor, want, "bootstrap")
		syncConsumer(t, consumer, "bootstrap")

		editor.Insert("x")
		// The real producer samples Collecting=true on every step; the mapping
		// consumes that fact instead of inventing it.
		frames, _, ok := editor.HistorySince(consumer.state.cursor)
		if !ok || len(frames) != 1 {
			t.Fatalf("pending frames = %d/ok=%t", len(frames), ok)
		}
		for _, step := range frames[0].Steps {
			if !step.Context.Collecting {
				t.Fatalf("real producer reported an uncollected step: %+v", step)
			}
		}
		syncConsumer(t, consumer, "insert x")
		applyReferenceEvents(t, want, evInsert(3, 1, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "insert x")
		if mask, err := consumer.LineMask(0, 4, 4); err != nil || mask != 4 {
			t.Fatalf("modified insert mask = %d/%v, want 4", mask, err)
		}

		editor.SetSavePoint()
		syncConsumer(t, consumer, "save")
		applyReferenceEvents(t, want, evSave)
		checkConsumerMatchesTracker(t, consumer, editor, want, "save")
		if mask, err := consumer.LineMask(0, 4, 4); err != nil || mask != 2 {
			t.Fatalf("saved insert mask = %d/%v, want 2", mask, err)
		}

		editor.Insert("y")
		syncConsumer(t, consumer, "insert y")
		applyReferenceEvents(t, want, evInsert(4, 1, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "insert y")
		if mask, err := consumer.LineMask(0, 5, 5); err != nil || mask != 6 {
			t.Fatalf("mixed saved/modified mask = %d/%v, want 6", mask, err)
		}

		editor.Undo()
		syncConsumer(t, consumer, "undo y")
		applyReferenceEvents(t, want, evUndoInsert(4, 1, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "undo y")
		if mask, err := consumer.LineMask(0, 4, 4); err != nil || mask != 2 {
			t.Fatalf("mask after undo y = %d/%v, want 2", mask, err)
		}

		editor.Undo()
		syncConsumer(t, consumer, "undo x across save")
		applyReferenceEvents(t, want, evStart, evUndoInsert(3, 1, true))
		checkConsumerMatchesTracker(t, consumer, editor, want, "undo x across save")
		if mask, err := consumer.LineMask(0, 3, 3); err != nil || mask != 1 {
			t.Fatalf("reverted-to-origin mask = %d/%v, want 1", mask, err)
		}

		editor.Redo()
		syncConsumer(t, consumer, "redo x")
		applyReferenceEvents(t, want, evInsert(3, 1, true, true))
		checkConsumerMatchesTracker(t, consumer, editor, want, "redo x")
		if mask, err := consumer.LineMask(0, 4, 4); err != nil || mask != 2 {
			t.Fatalf("mask after redo x = %d/%v, want 2", mask, err)
		}

		editor.Redo()
		syncConsumer(t, consumer, "redo y reaching save")
		applyReferenceEvents(t, want, evInsert(4, 1, true, false), evEnd)
		checkConsumerMatchesTracker(t, consumer, editor, want, "redo y reaching save")
		if mask, err := consumer.LineMask(0, 5, 5); err != nil || mask != 6 {
			t.Fatalf("mask after redo y = %d/%v, want 6", mask, err)
		}

		syncConsumer(t, consumer, "empty read")
		checkConsumerMatchesTracker(t, consumer, editor, want, "empty read")
	})

	t.Run("utf-8 lines eol and deletion points", func(t *testing.T) {
		editor := text.NewEditor("aé\nb漢c")
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, len("aé\nb漢c"))

		editor.MoveTo(3, false)
		if !editor.DeleteForward() {
			t.Fatal("DeleteForward refused")
		}
		syncConsumer(t, consumer, "delete b")
		applyReferenceEvents(t, want, evDelete(4, 1, false, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "delete b")
		if mask, err := consumer.LineMask(0, 3, 4); err != nil || mask != 0 {
			t.Fatalf("first line mask = %d/%v, want 0", mask, err)
		}
		if mask, err := consumer.LineMask(4, 8, 8); err != nil || mask != 4 {
			t.Fatalf("deletion point mask = %d/%v, want 4", mask, err)
		}

		editor.SetSavePoint()
		syncConsumer(t, consumer, "save")
		applyReferenceEvents(t, want, evSave)

		editor.MoveTo(0, false)
		editor.Insert("é")
		syncConsumer(t, consumer, "insert multibyte")
		applyReferenceEvents(t, want, evInsert(0, 2, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "insert multibyte")
		if mask, err := consumer.LineMask(0, 5, 6); err != nil || mask != 4 {
			t.Fatalf("shifted first line mask = %d/%v, want 4", mask, err)
		}
		if mask, err := consumer.LineMask(6, 10, 10); err != nil || mask != 2 {
			t.Fatalf("shifted saved deletion point mask = %d/%v, want 2", mask, err)
		}

		editor.Undo()
		syncConsumer(t, consumer, "undo multibyte")
		applyReferenceEvents(t, want, evUndoInsert(0, 2, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "undo multibyte")
		if mask, err := consumer.LineMask(0, 3, 4); err != nil || mask != 0 {
			t.Fatalf("mask after undo multibyte = %d/%v, want 0", mask, err)
		}

		editor.Undo()
		syncConsumer(t, consumer, "undo delete")
		applyReferenceEvents(t, want, evStart, evUndoDelete(4, 1, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "undo delete")
		if editor.String() != "aé\nb漢c" {
			t.Fatalf("restored text = %q", editor.String())
		}
		if mask, err := consumer.LineMask(4, 8, 8); err != nil || mask != 1 {
			t.Fatalf("reverted-to-origin mask = %d/%v, want 1", mask, err)
		}
	})

	t.Run("replace-ranges batch", func(t *testing.T) {
		editor := text.NewEditor("abcdef")
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, 6)

		if err := editor.ReplaceRanges([]text.Replacement{
			{Range: text.Range{Start: 1, End: 3}, Text: "XY"},
			{Range: text.Range{Start: 4, End: 5}, Text: "Z"},
		}); err != nil {
			t.Fatal(err)
		}
		syncConsumer(t, consumer, "replace ranges")
		// Default order is last target first: delete/insert 'e' at byte 4, then
		// delete "bc"/insert "XY" at byte 1; one frame, four primitive steps.
		applyReferenceEvents(t, want,
			evDelete(4, 1, false, false), evInsert(4, 1, true, false),
			evDelete(1, 2, false, false), evInsert(1, 2, true, false))
		if editor.String() != "aXYdZf" {
			t.Fatalf("batch text = %q", editor.String())
		}
		checkConsumerMatchesTracker(t, consumer, editor, want, "replace ranges")
		if mask, err := consumer.LineMask(0, 6, 6); err != nil || mask != 4 {
			t.Fatalf("batch mask = %d/%v, want 4", mask, err)
		}

		if err := editor.ReplaceRangesWith([]text.Replacement{
			{Range: text.Range{Start: 0, End: 1}, Text: "Q"},
			{Range: text.Range{Start: 5, End: 6}, Text: "R"},
		}, text.ReplaceRangesOptions{Forward: true}); err != nil {
			t.Fatal(err)
		}
		syncConsumer(t, consumer, "replace ranges forward")
		applyReferenceEvents(t, want,
			evDelete(0, 1, false, false), evInsert(0, 1, true, false),
			evDelete(5, 1, false, false), evInsert(5, 1, true, false))
		if editor.String() != "QXYdZR" {
			t.Fatalf("forward batch text = %q", editor.String())
		}
		checkConsumerMatchesTracker(t, consumer, editor, want, "replace ranges forward")
	})

	t.Run("equal text and zero net", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, 3)

		editor.ReplaceRange(text.Range{Start: 0, End: 1}, "a")
		syncConsumer(t, consumer, "equal replacement")
		applyReferenceEvents(t, want, evDelete(0, 1, false, false), evInsert(0, 1, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "equal replacement")

		if err := editor.ReplaceRangesWith([]text.Replacement{
			{Range: text.Range{Start: 1, End: 2}, Text: "b"},
		}, text.ReplaceRangesOptions{ApplyUnchanged: true}); err != nil {
			t.Fatal(err)
		}
		syncConsumer(t, consumer, "apply unchanged")
		applyReferenceEvents(t, want, evDelete(1, 1, false, false), evInsert(1, 1, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "apply unchanged")

		if err := editor.Transact(func(draft *text.Editor) error {
			draft.SetSelection(text.Range{Start: 3, End: 3})
			draft.Insert("xy")
			if !draft.DeleteBackward() || !draft.DeleteBackward() {
				t.Fatal("draft delete refused")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		syncConsumer(t, consumer, "zero-net transaction")
		applyReferenceEvents(t, want,
			evInsert(3, 2, true, false), evDelete(4, 1, false, false), evDelete(3, 1, false, false))
		if editor.String() != "abc" {
			t.Fatalf("zero-net text = %q", editor.String())
		}
		checkConsumerMatchesTracker(t, consumer, editor, want, "zero-net transaction")

		before := queryTracker(consumer.tracker)
		beforeFrames := consumer.frames
		editor.SetSelection(text.Range{Start: 1, End: 1})
		editor.Insert("")
		editor.ReplaceRange(text.Range{Start: 0, End: 0}, "")
		editor.MoveTo(0, false)
		editor.DeleteBackward()
		editor.Redo()
		syncConsumer(t, consumer, "genuine no-ops")
		if !reflect.DeepEqual(queryTracker(consumer.tracker), before) {
			t.Fatal("no-op edits changed consumer metadata")
		}
		if consumer.frames != beforeFrames {
			t.Fatalf("no-op edits consumed %d frames", consumer.frames-beforeFrames)
		}
	})

	t.Run("branch after undo and detach flags", func(t *testing.T) {
		editor := text.NewEditor("")
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, 0)

		for _, value := range []string{"a", "b", "c", "d"} {
			editor.BreakUndoGroup()
			editor.Insert(value)
			position := len(editor.String()) - 1
			syncConsumer(t, consumer, "typed unit "+value)
			applyReferenceEvents(t, want, evInsert(position, 1, true, false))
			checkConsumerMatchesTracker(t, consumer, editor, want, "typed unit "+value)
		}
		editor.SetSavePoint()
		syncConsumer(t, consumer, "save at 4")
		applyReferenceEvents(t, want, evSave)
		checkConsumerMatchesTracker(t, consumer, editor, want, "save at 4")

		if !editor.Undo() || !editor.Undo() {
			t.Fatal("Undo refused")
		}
		syncConsumer(t, consumer, "undo to saved branch point")
		applyReferenceEvents(t, want, evStart, evUndoInsert(3, 1, true), evStart, evUndoInsert(2, 1, true))
		checkConsumerMatchesTracker(t, consumer, editor, want, "undo to saved branch point")

		editor.Insert("x")
		syncConsumer(t, consumer, "first branch")
		applyReferenceEvents(t, want, evInsert(2, 1, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "first branch")

		editor.BreakUndoGroup()
		editor.Insert("y")
		syncConsumer(t, consumer, "second append after branch")
		applyReferenceEvents(t, want, evInsert(3, 1, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "second append after branch")

		if !editor.Undo() || !editor.Undo() || !editor.Undo() {
			t.Fatal("Undo after branch refused")
		}
		syncConsumer(t, consumer, "undo branch")
		applyReferenceEvents(t, want,
			evStart, evUndoInsert(3, 1, false),
			evStart, evUndoInsert(2, 1, false),
			evStart, evUndoInsert(1, 1, true))
		checkConsumerMatchesTracker(t, consumer, editor, want, "undo branch")

		editor.Insert("z")
		syncConsumer(t, consumer, "earlier branch moves detach")
		applyReferenceEvents(t, want, evInsert(1, 1, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "earlier branch moves detach")

		editor.SetSavePoint()
		syncConsumer(t, consumer, "save after branch")
		applyReferenceEvents(t, want, evSave)
		checkConsumerMatchesTracker(t, consumer, editor, want, "save after branch")
		if editor.String() != "az" {
			t.Fatalf("branch text = %q", editor.String())
		}
	})

	t.Run("save-only frames and repeated sync", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, 3)

		editor.SetSavePoint()
		syncConsumer(t, consumer, "first save")
		applyReferenceEvents(t, want, evSave)
		editor.SetSavePoint()
		syncConsumer(t, consumer, "second save")
		applyReferenceEvents(t, want, evSave)
		checkConsumerMatchesTracker(t, consumer, editor, want, "two saves")
		if consumer.frames != 2 {
			t.Fatalf("Save frames consumed = %d, want 2", consumer.frames)
		}
		if int64(consumer.metadataBytes) != estimateTrackerBytes(want) {
			t.Fatalf("Save-only frames charged retained metadata: %d vs %d", consumer.metadataBytes, estimateTrackerBytes(want))
		}
		for range 3 {
			syncConsumer(t, consumer, "empty save read")
		}
		if consumer.frames != 2 {
			t.Fatalf("empty syncs consumed frames: %d", consumer.frames)
		}

		editor.Insert("x")
		syncConsumer(t, consumer, "insert after saves")
		applyReferenceEvents(t, want, evInsert(3, 1, true, false))
		editor.Undo()
		syncConsumer(t, consumer, "undo after saves")
		applyReferenceEvents(t, want, evUndoInsert(3, 1, false))
		editor.SetSavePoint()
		syncConsumer(t, consumer, "save after undo")
		applyReferenceEvents(t, want, evSave)
		checkConsumerMatchesTracker(t, consumer, editor, want, "save after undo")
	})

	t.Run("keeps up past the frame ring", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, 3)

		// More metadata events than the producer's 256-frame delivery ring. A
		// consumer that reads them incrementally keeps a valid cursor.
		for index := 0; index < 300; index++ {
			editor.SetSavePoint()
			syncConsumer(t, consumer, "save past ring")
			applyReferenceEvents(t, want, evSave)
		}
		if consumer.frames != 300 {
			t.Fatalf("frames consumed = %d, want 300", consumer.frames)
		}
		checkConsumerMatchesTracker(t, consumer, editor, want, "past frame ring")

		// A single Sync may also consume the whole retained 256-frame window.
		batch := text.NewEditor("abc")
		batchConsumer := mustConsumer(t, batch)
		batchWant := referenceMetadata(t, 3)
		for index := 0; index < 256; index++ {
			batch.SetSavePoint()
			applyReferenceEvents(t, batchWant, evSave)
		}
		syncConsumer(t, batchConsumer, "single 256-frame sync")
		if batchConsumer.frames != 256 {
			t.Fatalf("single sync frames = %d, want 256", batchConsumer.frames)
		}
		checkConsumerMatchesTracker(t, batchConsumer, batch, batchWant, "single 256-frame sync")
		for index := 0; index < 100; index++ {
			batch.SetSavePoint()
			applyReferenceEvents(t, batchWant, evSave)
		}
		syncConsumer(t, batchConsumer, "incremental after single sync")
		if batchConsumer.frames != 356 {
			t.Fatalf("frames after incremental sync = %d, want 356", batchConsumer.frames)
		}
		checkConsumerMatchesTracker(t, batchConsumer, batch, batchWant, "incremental after single sync")
	})

	t.Run("mixed forward and undo atomic frame", func(t *testing.T) {
		editor := text.NewEditor("abc")
		if !editor.EnableTentativeComposition() {
			t.Fatal("EnableTentativeComposition refused")
		}
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, 3)

		editor.SetSelection(text.Range{Start: 3, End: 3})
		if !editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "に"}) {
			t.Fatal("first update refused")
		}
		syncConsumer(t, consumer, "first tentative update")
		applyReferenceEvents(t, want, evInsert(3, 3, true, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "first tentative update")

		if !editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "には"}) {
			t.Fatal("second update refused")
		}
		syncConsumer(t, consumer, "mixed atomic update")
		applyReferenceEvents(t, want,
			evUndoInsert(3, 3, false),
			evInsert(3, 3, true, false),
			evInsert(6, 3, true, false))
		if editor.String() != "abcには" {
			t.Fatalf("mixed update text = %q", editor.String())
		}
		checkConsumerMatchesTracker(t, consumer, editor, want, "mixed atomic update")

		if !editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCommit}) {
			t.Fatal("commit refused")
		}
		syncConsumer(t, consumer, "commit boundary")
		if consumer.frames != 2 {
			t.Fatalf("commit published a metadata frame: total %d", consumer.frames)
		}

		if !editor.Undo() {
			t.Fatal("Undo after commit refused")
		}
		syncConsumer(t, consumer, "undo committed composition")
		applyReferenceEvents(t, want, evUndoInsert(6, 3, false), evUndoInsert(3, 3, false))
		checkConsumerMatchesTracker(t, consumer, editor, want, "undo committed composition")
		if editor.String() != "abc" {
			t.Fatalf("text after undo = %q", editor.String())
		}
	})

	t.Run("large multi-step atomic frame", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		want := referenceMetadata(t, 3)
		const primitives = 32

		if err := editor.Transact(func(draft *text.Editor) error {
			end := 3
			for range primitives {
				draft.SetSelection(text.Range{Start: end, End: end})
				draft.Insert("x")
				end++
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		events := make([]Event, 0, primitives)
		for index := 0; index < primitives; index++ {
			events = append(events, evInsert(3+index, 1, true, false))
		}
		syncConsumer(t, consumer, "32-primitive transaction")
		applyReferenceEvents(t, want, events...)
		if consumer.frames != 1 || consumer.steps != primitives {
			t.Fatalf("large frame consumed %d frames/%d steps, want 1/%d", consumer.frames, consumer.steps, primitives)
		}
		checkConsumerMatchesTracker(t, consumer, editor, want, "32-primitive transaction")

		if !editor.Undo() {
			t.Fatal("Undo refused")
		}
		undoEvents := make([]Event, 0, primitives)
		for index := primitives - 1; index >= 0; index-- {
			undoEvents = append(undoEvents, evUndoInsert(3+index, 1, false))
		}
		syncConsumer(t, consumer, "undo 32 primitives")
		applyReferenceEvents(t, want, undoEvents...)
		checkConsumerMatchesTracker(t, consumer, editor, want, "undo 32 primitives")
		if editor.String() != "abc" {
			t.Fatalf("text after undo = %q", editor.String())
		}
	})

	t.Run("original unmodified prefix fixture", func(t *testing.T) {
		checkConsumerOriginalFixture(t, "testdata/scintilla-change-history.tsv", 2293)
	})

	t.Run("original span-corrected reference fixture", func(t *testing.T) {
		checkConsumerOriginalFixture(t, "testdata/scintilla-change-history-span-fixed.tsv", 8299)
	})
}

// ---------------------------------------------------------------------------
// Original fixture encoding

type fixtureRow struct {
	caseName, step, textHex, events string
	queries                         [4][]int
}

func readHistoryFixture(t *testing.T, path string) []fixtureRow {
	t.Helper()
	reader := historyFixtureReader(t, path, 8)
	var rows []fixtureRow
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		row := fixtureRow{caseName: fields[0], step: fields[1], textHex: fields[2], events: fields[3]}
		for query := 0; query < 4; query++ {
			for _, encoded := range strings.Split(fields[4+query], ",") {
				row.queries[query] = append(row.queries[query], referenceInteger(t, encoded))
			}
		}
		rows = append(rows, row)
	}
	return rows
}

type fixtureEvent struct {
	kind                             string
	position, length                 int
	collecting, beforeSave, detached bool
	reverting                        bool
	number                           int
}

func parseFixtureEvent(t *testing.T, raw string) fixtureEvent {
	t.Helper()
	parts := strings.Split(raw, ":")
	event := fixtureEvent{kind: parts[0]}
	switch event.kind {
	case "insert", "delete", "undo-insert", "undo-delete":
		event.position = referenceInteger(t, parts[1])
		event.length = referenceInteger(t, parts[2])
		switch event.kind {
		case "insert":
			event.collecting, event.beforeSave = parts[3] == "1", parts[4] == "1"
		case "delete":
			event.beforeSave, event.detached = parts[3] == "1", parts[4] == "1"
		case "undo-insert":
			event.reverting = parts[3] == "1"
		case "undo-delete":
			event.detached = parts[3] == "1"
		}
	case "reset":
		event.number = referenceInteger(t, parts[1])
	case "save", "start", "end":
	default:
		t.Fatalf("unknown fixture event %q", raw)
	}
	return event
}

// fixtureEncoder is the inverse-mapping witness. Its current/save/detach fields
// follow the source AppendAction/save/undo/redo position evolution recorded in
// the original fixture step labels, starting from 0/0/absent after a reset. It
// does not claim the original's private positions: it is the witness context
// that makes the mapping reproduce the fixture's recorded event facts.
type fixtureEncoder struct {
	current, save, detach int
}

func (encoder *fixtureEncoder) reset() {
	encoder.current, encoder.save = 0, 0
}

func (encoder *fixtureEncoder) updateBranch(boundary int) {
	if boundary < encoder.save {
		encoder.save = -1
		if encoder.detach == -1 {
			encoder.detach = boundary
		}
	} else if encoder.detach != -1 && encoder.detach > boundary {
		encoder.detach = boundary
	}
}

func (encoder *fixtureEncoder) retainedTo() int {
	value := max(encoder.current, 0)
	if encoder.save > value {
		value = encoder.save
	}
	if encoder.detach > value {
		value = encoder.detach
	}
	return value
}

func fixtureRedo(label string) bool {
	return strings.HasPrefix(label, "redo") || label == "step-6"
}

func fixtureUndo(label string, events []fixtureEvent) bool {
	if label == "step-5" {
		return true
	}
	for _, event := range events {
		if strings.HasPrefix(event.kind, "undo-") {
			return true
		}
	}
	return false
}

func fixtureLabel(step string) string {
	if index := strings.LastIndex(step, "/"); index >= 0 {
		return step[index+1:]
	}
	return step
}

// encodeFixtureFrame builds the HistoryFrame a consumer must consume so that its
// mapping reproduces the fixture row's event facts. It returns the frame and the
// encoded running byte length.
func encodeFixtureFrame(t *testing.T, row fixtureRow, before int, encoder *fixtureEncoder) (text.HistoryFrame, int) {
	t.Helper()
	label := fixtureLabel(row.step)
	events := []fixtureEvent{}
	if row.events != "-" {
		for _, raw := range strings.Split(row.events, ";") {
			events = append(events, parseFixtureEvent(t, raw))
		}
	}
	redo := fixtureRedo(label)
	undo := fixtureUndo(label, events)
	uncollected := strings.Contains(row.caseName, "without-undo")
	running := before
	steps := make([]text.HistoryStep, 0, len(events))
	pendingStart := false
	for index := 0; index < len(events); index++ {
		event := events[index]
		switch event.kind {
		case "start":
			pendingStart = true
			continue
		case "end":
			t.Fatalf("%s/%s: unconsumed fixture end event", row.caseName, row.step)
		case "save":
			if pendingStart {
				t.Fatalf("%s/%s: start event before save", row.caseName, row.step)
			}
			steps = append(steps, text.HistoryStep{
				Operation: text.HistorySave, Primitive: text.HistoryNoPrimitive,
				BeforeBytes: running, AfterBytes: running,
				Context: text.HistoryContext{
					Current: encoder.current, Save: encoder.current, Detach: -1,
					RetainedFrom: 0, RetainedTo: max(encoder.current, 0), Collecting: true,
				},
			})
			encoder.save, encoder.detach = encoder.current, -1
		case "insert":
			if undo {
				t.Fatalf("%s/%s: plain insert on an undo row", row.caseName, row.step)
			}
			if redo {
				beforeSave := (encoder.save < 0 || encoder.save > encoder.current) &&
					!(encoder.detach >= 0 && encoder.detach <= encoder.current)
				if beforeSave != event.beforeSave {
					t.Fatalf("%s/%s: redo insert before-save got %t, fixture %t (p=%d s=%d d=%d)",
						row.caseName, row.step, beforeSave, event.beforeSave, encoder.current, encoder.save, encoder.detach)
				}
				endReversion := encoder.save >= 0 && encoder.save <= encoder.current
				steps = append(steps, fixtureStep(text.HistoryRedo, text.HistoryInsert, event, running, encoder))
				running += event.length
				encoder.current++
				pendingStart = false
				index = consumeFixtureEnd(t, row, events, index, endReversion, true)
				continue
			}
			collected := event.collecting
			encoder.updateBranch(encoder.current)
			post := encoder.current
			if collected {
				post++
			}
			beforeSave := encoder.save > 0 && encoder.save > post
			if beforeSave != event.beforeSave {
				t.Fatalf("%s/%s: forward insert before-save got %t, fixture %t (p=%d s=%d d=%d)",
					row.caseName, row.step, beforeSave, event.beforeSave, encoder.current, encoder.save, encoder.detach)
			}
			retainedTo := encoder.retainedTo()
			if post > retainedTo {
				retainedTo = post
			}
			steps = append(steps, text.HistoryStep{
				Operation: text.HistoryForward, Primitive: text.HistoryInsert,
				Position: event.position, Length: event.length,
				BeforeBytes: running, AfterBytes: running + event.length,
				Context: text.HistoryContext{
					Current: post, Save: encoder.save, Detach: encoder.detach,
					RetainedFrom: 0, RetainedTo: retainedTo, Collecting: collected,
				},
			})
			running += event.length
			encoder.current = post
			pendingStart = false
		case "delete":
			if undo {
				t.Fatalf("%s/%s: plain delete on an undo row", row.caseName, row.step)
			}
			if redo {
				beforeSave := encoder.save > 0 && encoder.save > encoder.current
				detached := encoder.detach >= 0 && encoder.detach <= encoder.current
				if beforeSave != event.beforeSave || detached != event.detached {
					t.Fatalf("%s/%s: redo delete flags got %t/%t, fixture %t/%t (p=%d s=%d d=%d)",
						row.caseName, row.step, beforeSave, detached, event.beforeSave, event.detached,
						encoder.current, encoder.save, encoder.detach)
				}
				endReversion := encoder.save >= 0 && encoder.save <= encoder.current
				steps = append(steps, fixtureStep(text.HistoryRedo, text.HistoryDelete, event, running, encoder))
				running -= event.length
				encoder.current++
				pendingStart = false
				index = consumeFixtureEnd(t, row, events, index, endReversion, true)
				continue
			}
			encoder.updateBranch(encoder.current)
			post := encoder.current
			if !uncollected {
				post++
			}
			beforeSave := encoder.save > 0 && encoder.save > post
			detached := encoder.detach >= 0 && encoder.detach <= post
			if beforeSave != event.beforeSave || detached != event.detached {
				t.Fatalf("%s/%s: forward delete flags got %t/%t, fixture %t/%t (p=%d s=%d d=%d)",
					row.caseName, row.step, beforeSave, detached, event.beforeSave, event.detached,
					encoder.current, encoder.save, encoder.detach)
			}
			retainedTo := encoder.retainedTo()
			if post > retainedTo {
				retainedTo = post
			}
			steps = append(steps, text.HistoryStep{
				Operation: text.HistoryForward, Primitive: text.HistoryDelete,
				Position: event.position, Length: event.length,
				BeforeBytes: running, AfterBytes: running - event.length,
				Context: text.HistoryContext{
					Current: post, Save: encoder.save, Detach: encoder.detach,
					RetainedFrom: 0, RetainedTo: retainedTo, Collecting: !uncollected,
				},
			})
			running -= event.length
			encoder.current = post
			pendingStart = false
		case "undo-insert", "undo-delete":
			expectedStart := encoder.save < 0 || encoder.save >= encoder.current
			if pendingStart != expectedStart {
				t.Fatalf("%s/%s: start event presence got %t, source predicate %t (p=%d s=%d d=%d)",
					row.caseName, row.step, pendingStart, expectedStart, encoder.current, encoder.save, encoder.detach)
			}
			pendingStart = false
			primitive := text.HistoryInsert
			after := running + event.length
			if event.kind == "undo-insert" {
				primitive = text.HistoryDelete
				after = running - event.length
				reverting := expectedStart && !(encoder.detach >= 0 && encoder.detach < encoder.current)
				if reverting != event.reverting {
					t.Fatalf("%s/%s: undo-insert reverting got %t, fixture %t (p=%d s=%d d=%d)",
						row.caseName, row.step, reverting, event.reverting, encoder.current, encoder.save, encoder.detach)
				}
			} else {
				detached := encoder.detach >= 0 && encoder.detach < encoder.current
				if detached != event.detached {
					t.Fatalf("%s/%s: undo-delete detached got %t, fixture %t (p=%d s=%d d=%d)",
						row.caseName, row.step, detached, event.detached, encoder.current, encoder.save, encoder.detach)
				}
			}
			steps = append(steps, text.HistoryStep{
				Operation: text.HistoryUndo, Primitive: primitive,
				Position: event.position, Length: event.length,
				BeforeBytes: running, AfterBytes: after,
				Context: text.HistoryContext{
					Current: encoder.current, Save: encoder.save, Detach: encoder.detach,
					RetainedFrom: 0, RetainedTo: encoder.retainedTo(), Collecting: true,
				},
			})
			running = after
			encoder.current--
			if encoder.current < 0 {
				t.Fatalf("%s/%s: fixture undo drove the action position negative", row.caseName, row.step)
			}
		default:
			t.Fatalf("%s/%s: unexpected fixture event %q", row.caseName, row.step, event.kind)
		}
	}
	if pendingStart {
		t.Fatalf("%s/%s: fixture start event was not followed by an undo primitive", row.caseName, row.step)
	}
	return text.HistoryFrame{BeforeBytes: before, AfterBytes: running, Steps: steps}, running
}

// consumeFixtureEnd verifies that the mapping's derived EndReversion matches the
// fixture's recorded end event, consuming that event when present.
func consumeFixtureEnd(t *testing.T, row fixtureRow, events []fixtureEvent, index int, expected, redo bool) int {
	t.Helper()
	if !redo {
		t.Fatalf("%s/%s: internal end encoding outside a redo primitive", row.caseName, row.step)
	}
	hasEnd := index+1 < len(events) && events[index+1].kind == "end"
	if expected != hasEnd {
		t.Fatalf("%s/%s: end event presence got %t, AfterSavePoint predicate %t", row.caseName, row.step, hasEnd, expected)
	}
	if hasEnd {
		return index + 1
	}
	return index
}

func fixtureStep(operation text.HistoryOperation, primitive text.HistoryPrimitive, event fixtureEvent, running int, encoder *fixtureEncoder) text.HistoryStep {
	after := running + event.length
	if primitive == text.HistoryDelete {
		after = running - event.length
	}
	retainedTo := encoder.retainedTo()
	// A redo step samples the retained tail it must fit into; the producer
	// publishes a frame only when the completed position is at or before that
	// tail. Model the tail for the whole replay position, not only the
	// pre-step position.
	if operation == text.HistoryRedo && retainedTo <= encoder.current {
		retainedTo = encoder.current + 1
	}
	return text.HistoryStep{
		Operation: operation, Primitive: primitive,
		Position: event.position, Length: event.length,
		BeforeBytes: running, AfterBytes: after,
		Context: text.HistoryContext{
			Current: encoder.current, Save: encoder.save, Detach: encoder.detach,
			RetainedFrom: 0, RetainedTo: retainedTo, Collecting: true,
		},
	}
}

// harvestHistoryCursors produces a contiguous opaque cursor chain from a real
// producer so the fixture frames can be validated through the same frame-chain
// code path as live reads.
func harvestHistoryCursors(t *testing.T, count int) []text.HistoryCursor {
	t.Helper()
	editor := text.NewEditor("")
	cursor, _, ok := editor.TrackHistory()
	if !ok {
		t.Fatal("cursor harvest TrackHistory refused")
	}
	chain := make([]text.HistoryCursor, 0, count+1)
	chain = append(chain, cursor)
	for len(chain) <= count {
		editor.SetSavePoint()
		frames, latest, ok := editor.HistorySince(chain[len(chain)-1])
		if !ok || len(frames) != 1 || frames[0].From != chain[len(chain)-1] {
			t.Fatalf("cursor harvest read failed: %d frames/ok=%t", len(frames), ok)
		}
		chain = append(chain, latest)
	}
	return chain
}

func checkConsumerOriginalFixture(t *testing.T, path string, wantedStates int) {
	t.Helper()
	rows := readHistoryFixture(t, path)
	chain := harvestHistoryCursors(t, 60)
	var tracker *Tracker
	encoder := &fixtureEncoder{detach: -1}
	cursorIndex := 0
	state := consumerState{cursor: chain[0]}
	activeCase := ""
	checked, frames, steps := 0, 0, 0
	for _, row := range rows {
		var before []byte
		var err error
		if row.textHex != "-" {
			before, err = hex.DecodeString(row.textHex)
			if err != nil {
				t.Fatalf("%s/%s: %v", row.caseName, row.step, err)
			}
		}
		if row.events != "-" && strings.Contains(row.events, "reset") {
			reset := parseFixtureEvent(t, row.events)
			if reset.kind != "reset" || row.events != "reset:"+strconv.Itoa(reset.number) {
				t.Fatalf("%s/%s: malformed reset row %q", row.caseName, row.step, row.events)
			}
			if len(before) != reset.number {
				t.Fatalf("%s/%s: reset length %d, text has %d bytes", row.caseName, row.step, reset.number, len(before))
			}
			if row.caseName != activeCase {
				encoder = &fixtureEncoder{detach: -1}
				activeCase = row.caseName
			}
			tracker = referenceMetadata(t, reset.number)
			encoder.reset()
			cursorIndex = 0
			state = consumerState{cursor: chain[0], bytes: reset.number}
			checkFixtureQueries(t, tracker, row, before)
			checked++
			continue
		}
		if tracker == nil {
			t.Fatalf("%s/%s: fixture row before any reset", row.caseName, row.step)
		}
		if row.events != "-" {
			frame, after := encodeFixtureFrame(t, row, state.bytes, encoder)
			if after != len(before) {
				t.Fatalf("%s/%s: encoded byte length %d, fixture text has %d bytes", row.caseName, row.step, after, len(before))
			}
			frame.From, frame.To = chain[cursorIndex], chain[cursorIndex+1]
			next, err := applyFrames(tracker, []text.HistoryFrame{frame}, chain[cursorIndex+1], state)
			if err != nil {
				t.Fatalf("%s/%s: consumer rejected the fixture frame: %v", row.caseName, row.step, err)
			}
			state = next
			cursorIndex++
			frames++
			steps += len(frame.Steps)
		}
		if tracker.Length() != len(before) {
			t.Fatalf("%s/%s: metadata length %d, text has %d bytes", row.caseName, row.step, tracker.Length(), len(before))
		}
		checkFixtureQueries(t, tracker, row, before)
		checked++
	}
	if checked != wantedStates {
		t.Fatalf("fixture contains %d states, want %d", checked, wantedStates)
	}
	t.Logf("matched %d original states through %d encoded frames and %d steps", checked, frames, steps)
}

func checkFixtureQueries(t *testing.T, tracker *Tracker, row fixtureRow, textBytes []byte) {
	t.Helper()
	checkFixtureDecorations(t, tracker, row, len(textBytes))
	if tracker.Length() != len(textBytes) {
		t.Fatalf("%s/%s: metadata length %d, text has %d bytes", row.caseName, row.step, tracker.Length(), len(textBytes))
	}
	for position := 0; position <= len(textBytes); position++ {
		got := [4]int{
			tracker.EditionAt(position),
			tracker.EndRun(position),
			int(tracker.DeletesAt(position)),
			tracker.NextDelete(position),
		}
		for query := 0; query < 4; query++ {
			if position >= len(row.queries[query]) {
				t.Fatalf("%s/%s: fixture query %d has no position %d", row.caseName, row.step, query, position)
			}
			if got[query] != row.queries[query][position] {
				t.Fatalf("%s/%s: query=%d position=%d got %d, want %d; events=%s",
					row.caseName, row.step, query, position, got[query], row.queries[query][position], row.events)
			}
		}
	}
}
