package changehistory

// H02 consumer contract tests. These drive the real Viva text history producer
// through ordinary editor entry points and the change-history metadata engine.
// Expected values are derived from the source predicates recorded in
// docs/specs/001-vivapad/change-history-editor-contract.md and from the original
// fixture-validated Tracker behaviour, never from the consumer implementation.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func mustConsumer(t *testing.T, editor *text.Editor) *Consumer {
	t.Helper()
	consumer, err := NewConsumer(editor)
	if err != nil {
		t.Fatalf("NewConsumer refused an editor with no Undo/Redo history: %v", err)
	}
	if consumer == nil {
		t.Fatal("NewConsumer returned a nil consumer")
	}
	return consumer
}

func syncConsumer(t *testing.T, consumer *Consumer, stage string) {
	t.Helper()
	if err := consumer.Sync(); err != nil {
		t.Fatalf("%s: Sync failed: %v", stage, err)
	}
	if consumer.unavailable {
		t.Fatalf("%s: consumer became unavailable after a successful Sync", stage)
	}
}

// textLineBounds returns UTF-8 start/content-end/next bounds for every logical
// line, using the same normalized-LF adapter convention as the navigation tests.
func textLineBounds(value string) [][3]int {
	contents := strings.SplitAfter(value, "\n")
	bounds := make([][3]int, 0, len(contents))
	start := 0
	for _, content := range contents {
		next := start + len(content)
		end := next
		if strings.HasSuffix(content, "\n") {
			end--
		}
		bounds = append(bounds, [3]int{start, end, next})
		start = next
	}
	return bounds
}

// checkConsumerMatchesTracker compares every edition/run/deletion query the
// engine exposes, at every byte position, plus the line projection for every
// line of the current text.
func checkConsumerMatchesTracker(t *testing.T, consumer *Consumer, editor *text.Editor, want *Tracker, stage string) {
	t.Helper()
	if consumer.unavailable {
		t.Fatalf("%s: consumer is unavailable", stage)
	}
	if consumer.tracker == nil {
		t.Fatalf("%s: consumer has no metadata", stage)
	}
	got := queryTracker(consumer.tracker)
	expected := queryTracker(want)
	if !reflect.DeepEqual(got, expected) {
		for position := range got {
			if position >= len(expected) || got[position] != expected[position] {
				t.Fatalf("%s: position %d got edition/end-run/deletes/next-delete %v, want %v (length %d vs %d)",
					stage, position, got[position], expected[position], len(got)-1, len(expected)-1)
			}
		}
		t.Fatalf("%s: tracker queries differ", stage)
	}
	if consumer.tracker.Length() != want.Length() {
		t.Fatalf("%s: metadata length %d, want %d", stage, consumer.tracker.Length(), want.Length())
	}
	for _, bounds := range textLineBounds(editor.String()) {
		gotMask, err := consumer.LineMask(bounds[0], bounds[1], bounds[2])
		if err != nil {
			t.Fatalf("%s: LineMask%v: %v", stage, bounds, err)
		}
		wantMask, err := want.LineMask(bounds[0], bounds[1], bounds[2])
		if err != nil {
			t.Fatalf("%s: reference LineMask%v: %v", stage, bounds, err)
		}
		if gotMask != wantMask {
			t.Fatalf("%s: LineMask%v = %d, want %d for text %q", stage, bounds, gotMask, wantMask, editor.String())
		}
	}
}

func referenceMetadata(t *testing.T, bootstrap int, events ...Event) *Tracker {
	t.Helper()
	tracker, err := New(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	applyReferenceEvents(t, tracker, events...)
	return tracker
}

func applyReferenceEvents(t *testing.T, tracker *Tracker, events ...Event) {
	t.Helper()
	for _, event := range events {
		if err := tracker.Apply(event); err != nil {
			t.Fatalf("reference event %+v refused: %v", event, err)
		}
	}
}

func assertUnavailable(t *testing.T, consumer *Consumer, stage string) {
	t.Helper()
	if !consumer.unavailable {
		t.Fatalf("%s: consumer did not enter the unavailable state", stage)
	}
	if consumer.reason == nil {
		t.Fatalf("%s: unavailable consumer has no recorded reason", stage)
	}
	if consumer.tracker != nil {
		t.Fatalf("%s: unavailable consumer retained metadata instead of releasing it", stage)
	}
	if consumer.metadataBytes != 0 {
		t.Fatalf("%s: unavailable consumer still charges %d metadata bytes", stage, consumer.metadataBytes)
	}
	if err := consumer.Sync(); err == nil {
		t.Fatalf("%s: Sync after invalidation reported success", stage)
	}
	mask, err := consumer.LineMask(0, 0, 0)
	if err == nil {
		t.Fatalf("%s: LineMask after invalidation returned mask %d and no error", stage, mask)
	}
}

// TestConsumerRejectsIncompleteFramesAtomically verifies that malformed,
// truncated, reordered, stale and foreign frame chains never partially mutate
// the committed metadata, always enter the unavailable state and make LineMask
// refuse instead of answering an empty "no changes" mask.
func TestConsumerRejectsIncompleteFramesAtomically(t *testing.T) {
	type rejection struct {
		name  string
		setup func(t *testing.T) (*Consumer, *text.Editor)
	}

	// realFrames returns the frames a consumer would read from its pinned cursor.
	realFrames := func(t *testing.T, consumer *Consumer) ([]text.HistoryFrame, text.HistoryCursor) {
		t.Helper()
		frames, latest, ok := consumer.editor.HistorySince(consumer.state.cursor)
		if !ok || len(frames) == 0 {
			t.Fatalf("expected pending producer frames, got %d/ok=%t", len(frames), ok)
		}
		return frames, latest
	}

	staged := func(t *testing.T) (*Consumer, *text.Editor) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		return consumer, editor
	}
	inject := func(consumer *Consumer, frames []text.HistoryFrame, latest text.HistoryCursor) {
		consumer.read = func(text.HistoryCursor) ([]text.HistoryFrame, text.HistoryCursor, bool) {
			return frames, latest, true
		}
	}

	cases := []rejection{
		{name: "malformed step combination", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			frames, latest := realFrames(t, consumer)
			frames[0].Steps[0].Primitive = text.HistoryNoPrimitive
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "delete byte range inconsistent", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.MoveTo(0, false)
			editor.DeleteForward()
			frames, latest := realFrames(t, consumer)
			frames[0].Steps[0].AfterBytes++
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "insert position outside running bytes", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			frames, latest := realFrames(t, consumer)
			frames[0].Steps[0].Position = 99
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "step before-bytes discontinuity", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.ReplaceRange(text.Range{Start: 0, End: 1}, "a")
			frames, latest := realFrames(t, consumer)
			if len(frames) != 1 || len(frames[0].Steps) != 2 {
				t.Fatalf("replacement published %d frames", len(frames))
			}
			frames[0].Steps[1].BeforeBytes++
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "frame before-bytes discontinuity", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			frames, latest := realFrames(t, consumer)
			frames[0].BeforeBytes++
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "frame after-bytes mismatch", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			frames, latest := realFrames(t, consumer)
			frames[0].AfterBytes++
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "save step with a primitive", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.SetSavePoint()
			frames, latest := realFrames(t, consumer)
			frames[0].Steps[0].Primitive = text.HistoryInsert
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "unknown operation", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			frames, latest := realFrames(t, consumer)
			frames[0].Steps[0].Operation = text.HistoryOperation(200)
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "action position discontinuity", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			editor.Insert("y")
			frames, latest := realFrames(t, consumer)
			frames[1].Steps[0].Context.Current++
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "negative action position", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			frames, latest := realFrames(t, consumer)
			frames[0].Steps[0].Context.Current = -1
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "truncated steps", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.ReplaceRange(text.Range{Start: 0, End: 1}, "xy")
			frames, latest := realFrames(t, consumer)
			if len(frames) != 1 || len(frames[0].Steps) < 2 {
				t.Fatalf("replacement published %d frames/%d steps", len(frames), len(frames[0].Steps))
			}
			frames[0].Steps = frames[0].Steps[:1]
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "truncated frame list", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			editor.Insert("y")
			frames, latest := realFrames(t, consumer)
			inject(consumer, frames[:1], latest)
			return consumer, editor
		}},
		{name: "out-of-order frames", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			editor.Insert("y")
			frames, latest := realFrames(t, consumer)
			frames[0], frames[1] = frames[1], frames[0]
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "frame from cursor does not match pinned cursor", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			editor.Insert("y")
			frames, latest := realFrames(t, consumer)
			frames[0].From = frames[1].From
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "latest cursor ahead of delivered frames", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			editor.Insert("y")
			frames, latest := realFrames(t, consumer)
			inject(consumer, frames[:1], latest)
			return consumer, editor
		}},
		{name: "final byte length disagrees with live document", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			frames, latest := realFrames(t, consumer)
			// Restore the live text without delivering the restoring frame, so a
			// complete-looking frame chain ends one byte past the document.
			editor.ReplaceRange(text.Range{Start: 3, End: 4}, "")
			inject(consumer, frames, latest)
			return consumer, editor
		}},
		{name: "foreign cursor", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			other := text.NewEditor("abc")
			foreign, _, ok := other.TrackHistory()
			if !ok {
				t.Fatal("foreign TrackHistory refused")
			}
			consumer.state.cursor = foreign
			return consumer, editor
		}},
		{name: "swapped editor source", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			swapped := text.NewEditor("abc")
			if _, _, ok := swapped.TrackHistory(); !ok {
				t.Fatal("swapped TrackHistory refused")
			}
			consumer.editor = swapped
			return consumer, editor
		}},
		{name: "zero cursor", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			consumer.state.cursor = text.HistoryCursor{}
			return consumer, editor
		}},
		{name: "padding frame before first real event", setup: func(t *testing.T) (*Consumer, *text.Editor) {
			consumer, editor := staged(t)
			editor.Insert("x")
			frames, latest := realFrames(t, consumer)
			inject(consumer, append([]text.HistoryFrame{{From: frames[0].From, To: frames[0].To, BeforeBytes: 3, AfterBytes: 3, Steps: []text.HistoryStep{{
				Operation: text.HistorySave, Primitive: text.HistoryNoPrimitive, BeforeBytes: 3, AfterBytes: 3,
				Context: text.HistoryContext{Current: 0, Save: 0, Detach: -1, RetainedTo: 0, Collecting: true},
			}}}}, frames...), latest)
			return consumer, editor
		}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			consumer, editor := test.setup(t)
			oldTracker := consumer.tracker
			var snapshot [][4]int
			if oldTracker != nil {
				snapshot = queryTracker(oldTracker)
			}
			err := consumer.Sync()
			if err == nil {
				t.Fatal("Sync accepted an incomplete or foreign frame chain")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "unavailable") {
				t.Fatalf("Sync error does not report the unavailable history: %v", err)
			}
			assertUnavailable(t, consumer, test.name)
			if oldTracker != nil {
				if !reflect.DeepEqual(queryTracker(oldTracker), snapshot) {
					t.Fatalf("%s: a rejected chain partially mutated the committed metadata", test.name)
				}
			}
			// The line projection must never fall back to an empty mask.
			bounds := textLineBounds(editor.String())
			for _, line := range bounds {
				if mask, err := consumer.LineMask(line[0], line[1], line[2]); err == nil {
					t.Fatalf("%s: unavailable LineMask%v returned %d without an error", test.name, line, mask)
				}
			}
		})
	}
}

// TestConsumerMissingCheckpointAndLineMaskRefusal pins the stale-cursor path
// against the real producer: when the retained frame prefix is evicted, the
// consumer reports unavailable, releases its metadata and refuses queries.
func TestConsumerMissingCheckpointAndLineMaskRefusal(t *testing.T) {
	editor := text.NewEditor("abc")
	consumer := mustConsumer(t, editor)
	editor.Insert("x")
	syncConsumer(t, consumer, "initial edit")
	oldTracker := consumer.tracker
	snapshot := queryTracker(oldTracker)

	// More metadata events than the producer's retained frame ring.
	for index := 0; index < 300; index++ {
		editor.SetSavePoint()
	}
	err := consumer.Sync()
	if err == nil {
		t.Fatal("Sync silently accepted a lost frame prefix")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unavailable") {
		t.Fatalf("missing-checkpoint error does not report unavailable: %v", err)
	}
	assertUnavailable(t, consumer, "missing checkpoint")
	if !reflect.DeepEqual(queryTracker(oldTracker), snapshot) {
		t.Fatal("missing checkpoint partially mutated the committed metadata")
	}
	if mask, err := consumer.LineMask(0, 4, 4); err == nil {
		t.Fatalf("LineMask returned %d instead of reporting unavailable history", mask)
	}
}

// TestConsumerLineMaskBoundsAndNoMutation keeps the consumer projection
// consistent with Tracker.LineMask: invalid bounds are refused and a query does
// not change the metadata or consume producer frames.
func TestConsumerLineMaskBoundsAndNoMutation(t *testing.T) {
	editor := text.NewEditor("aé\nb漢c")
	consumer := mustConsumer(t, editor)
	editor.MoveTo(3, false)
	if !editor.DeleteForward() {
		t.Fatal("DeleteForward refused")
	}
	syncConsumer(t, consumer, "delete")
	before := queryTracker(consumer.tracker)
	beforeFrames := consumer.frames
	if mask, err := consumer.LineMask(0, 3, 4); err != nil || mask != 0 {
		t.Fatalf("first line mask = %d/%v, want 0", mask, err)
	}
	if mask, err := consumer.LineMask(4, 8, 8); err != nil || mask != 4 {
		t.Fatalf("second line deletion mask = %d/%v, want 4", mask, err)
	}
	for _, bounds := range [][3]int{{-1, 0, 1}, {2, 1, 3}, {1, 3, 2}, {0, 0, 9}, {9, 9, 9}} {
		if _, err := consumer.LineMask(bounds[0], bounds[1], bounds[2]); err == nil {
			t.Fatalf("invalid bounds %v accepted", bounds)
		}
	}
	if !reflect.DeepEqual(queryTracker(consumer.tracker), before) {
		t.Fatal("LineMask query mutated the consumer metadata")
	}
	if consumer.frames != beforeFrames {
		t.Fatal("LineMask query consumed producer frames")
	}
	// A repeated Sync with no producer frames must not change metadata either.
	syncConsumer(t, consumer, "repeated sync")
	if !reflect.DeepEqual(queryTracker(consumer.tracker), before) {
		t.Fatal("empty Sync changed the consumer metadata")
	}
}

// TestConsumerConstructionRefusals keeps construction fail-closed: a nil editor,
// a producer that cannot be enabled (live Undo history) and an active composition
// all return an explicit error instead of an available-looking consumer.
func TestConsumerConstructionRefusals(t *testing.T) {
	if consumer, err := NewConsumer(nil); err == nil || consumer != nil {
		t.Fatalf("NewConsumer(nil) = %v/%v, want nil/error", consumer, err)
	}
	edited := text.NewEditor("abc")
	edited.Insert("x")
	if consumer, err := NewConsumer(edited); err == nil || consumer != nil {
		t.Fatalf("NewConsumer with live Undo history = %v/%v, want nil/error", consumer, err)
	}
	composing := text.NewEditor("abc")
	composing.SetMarkedText("x", text.Range{})
	if consumer, err := NewConsumer(composing); err == nil || consumer != nil {
		t.Fatalf("NewConsumer during composition = %v/%v, want nil/error", consumer, err)
	}
}

// TestConsumerLineMaskUsesCommittedMetadata pins the staleness contract: queries
// project the metadata committed by the last successful Sync, so an un-Synced
// edit is not visible until Sync publishes it.
func TestConsumerLineMaskUsesCommittedMetadata(t *testing.T) {
	editor := text.NewEditor("abc")
	consumer := mustConsumer(t, editor)
	if mask, err := consumer.LineMask(0, 3, 3); err != nil || mask != 0 {
		t.Fatalf("bootstrap mask = %d/%v, want 0", mask, err)
	}
	editor.MoveTo(0, false)
	editor.Insert("x")
	if mask, err := consumer.LineMask(0, 3, 3); err != nil || mask != 0 {
		t.Fatalf("un-Synced edit leaked into the mask: %d/%v", mask, err)
	}
	syncConsumer(t, consumer, "publish edit")
	if mask, err := consumer.LineMask(0, 4, 4); err != nil || mask != 4 {
		t.Fatalf("published mask = %d/%v, want 4", mask, err)
	}
}
