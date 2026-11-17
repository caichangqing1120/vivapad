package changehistory

// H02 Codex-review regressions. Each test targets one of the four P2 findings on
// the frame-validation path:
//  1. cursor-neutral frames bypass chain validation;
//  2. an empty read bypasses the live-document length check;
//  3. replay completion may leave the declared retained-history bounds;
//  4. Save accepts an impossible sampled context.
//
// The tests drive the real producer for their valid frames and only inject a
// tampered/suppressed read through the consumer's test-only frame source, so the
// production path and the mapping stay exactly as accepted.

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/text"
)

// injectPendingFrames replaces the consumer's frame source with one fixed
// response. It is a test-only seam (production always reads editor.HistorySince).
func injectPendingFrames(t *testing.T, consumer *Consumer, frames []text.HistoryFrame, latest text.HistoryCursor) {
	t.Helper()
	consumer.read = func(text.HistoryCursor) ([]text.HistoryFrame, text.HistoryCursor, bool) {
		return frames, latest, true
	}
}

// pendingFrames reads the frames a consumer would consume at its pinned cursor.
func pendingFrames(t *testing.T, consumer *Consumer) ([]text.HistoryFrame, text.HistoryCursor) {
	t.Helper()
	frames, latest, ok := consumer.editor.HistorySince(consumer.state.cursor)
	if !ok || len(frames) == 0 {
		t.Fatalf("expected pending producer frames, got %d/ok=%t", len(frames), ok)
	}
	return frames, latest
}

func snapshotTracker(t *testing.T, consumer *Consumer) (*Tracker, [][4]int) {
	t.Helper()
	tracker := consumer.tracker
	if tracker == nil {
		t.Fatal("consumer has no committed metadata")
	}
	return tracker, queryTracker(tracker)
}

func requireUnchangedCommittedMetadata(t *testing.T, tracker *Tracker, snapshot [][4]int) {
	t.Helper()
	if !reflect.DeepEqual(queryTracker(tracker), snapshot) {
		t.Fatal("a rejected frame changed the previously committed metadata")
	}
}

// TestConsumerRejectsCursorNeutralFrames is the finding 1 regression: a frame
// whose From and To equal the pinned cursor must not be accepted (it would apply
// events without the producer's sequence advancing). It covers the Save shape
// used by the review, a destination token that repeats the pinned cursor, and a
// completed Save frame with no pending real frame at all.
func TestConsumerRejectsCursorNeutralFrames(t *testing.T) {
	save := func(from, to text.HistoryCursor, current, retainedTo int) text.HistoryFrame {
		return text.HistoryFrame{
			From: from, To: to, BeforeBytes: 4, AfterBytes: 4,
			Steps: []text.HistoryStep{{
				Operation: text.HistorySave, Primitive: text.HistoryNoPrimitive,
				BeforeBytes: 4, AfterBytes: 4,
				Context: text.HistoryContext{
					Current: current, Save: current, Detach: -1,
					RetainedFrom: 0, RetainedTo: retainedTo, Collecting: true,
				},
			}},
		}
	}

	t.Run("neutral save after a modified edition", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		editor.Insert("x")
		syncConsumer(t, consumer, "modified edition")
		if consumer.tracker.EditionAt(3) != 3 {
			t.Fatalf("insert edition = %d, want 3", consumer.tracker.EditionAt(3))
		}
		tracker, snapshot := snapshotTracker(t, consumer)
		epoch := consumer.state.cursor
		injectPendingFrames(t, consumer, []text.HistoryFrame{save(epoch, epoch, 1, 1)}, epoch)
		err := consumer.Sync()
		if err == nil {
			t.Fatalf("Sync accepted a cursor-neutral Save frame; modified edition became %d (want the frame refused before Saving)",
				consumer.tracker.EditionAt(3))
		}
		assertUnavailable(t, consumer, "cursor-neutral save")
		requireUnchangedCommittedMetadata(t, tracker, snapshot)
		if mask, err := consumer.LineMask(0, 4, 4); err == nil {
			t.Fatalf("LineMask returned %d instead of the unavailable history", mask)
		}
	})

	t.Run("neutral save when the producer published nothing", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		editor.Insert("x")
		syncConsumer(t, consumer, "modified edition")
		_ = consumer.tracker
		tracker, snapshot := snapshotTracker(t, consumer)
		epoch := consumer.state.cursor
		// No real pending frame exists here: the injected Save is the only frame.
		injectPendingFrames(t, consumer, []text.HistoryFrame{save(epoch, epoch, 1, 1)}, epoch)
		if err := consumer.Sync(); err == nil {
			t.Fatalf("Sync accepted a cursor-neutral Save frame with no producer event; edition %d", consumer.tracker.EditionAt(3))
		}
		assertUnavailable(t, consumer, "standalone cursor-neutral save")
		requireUnchangedCommittedMetadata(t, tracker, snapshot)
	})

	t.Run("destination token repeats the pinned cursor", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		epoch := consumer.state.cursor
		editor.Insert("x")
		frames, latest := pendingFrames(t, consumer)
		// First frame advances (epoch -> latest); the second frame pretends to
		// advance "from latest" but its destination is the old pinned cursor.
		second := save(latest, epoch, 1, 1)
		second.BeforeBytes, second.AfterBytes = 4, 4
		second.Steps[0].BeforeBytes, second.Steps[0].AfterBytes = 4, 4
		injectPendingFrames(t, consumer, []text.HistoryFrame{frames[0], second}, epoch)
		if err := consumer.Sync(); err == nil {
			t.Fatal("Sync accepted a frame whose destination token repeated the pinned cursor")
		}
		assertUnavailable(t, consumer, "repeated destination token")
		if mask, err := consumer.LineMask(0, 4, 4); err == nil {
			t.Fatalf("LineMask returned %d instead of the unavailable history", mask)
		}
	})
}

// TestConsumerRejectsSuppressedEmptyRead is the finding 2 regression: when the
// producer response is an empty successful read while the live document already
// moved, the consumer must invalidate instead of keeping stale metadata that can
// answer an empty "no changes" mask for changed bounds.
func TestConsumerRejectsSuppressedEmptyRead(t *testing.T) {
	editor := text.NewEditor("abc")
	consumer := mustConsumer(t, editor)
	syncConsumer(t, consumer, "bootstrap")
	tracker, snapshot := snapshotTracker(t, consumer)
	if mask, err := consumer.LineMask(0, 3, 3); err != nil || mask != 0 {
		t.Fatalf("bootstrap mask = %d/%v, want 0", mask, err)
	}

	editor.MoveTo(0, false)
	editor.Insert("x") // live text "xabc"; the producer frame is deliberately suppressed
	epoch := consumer.state.cursor
	injectPendingFrames(t, consumer, nil, epoch)
	err := consumer.Sync()
	if err == nil {
		mask, maskErr := consumer.LineMask(0, 3, 3)
		t.Fatalf("Sync accepted a suppressed empty read: live text %q, mask=%d/%v (stale metadata)",
			editor.String(), mask, maskErr)
	}
	assertUnavailable(t, consumer, "suppressed empty read")
	requireUnchangedCommittedMetadata(t, tracker, snapshot)
	if mask, err := consumer.LineMask(0, 3, 3); err == nil {
		t.Fatalf("LineMask returned %d instead of the unavailable history", mask)
	}
}

// TestConsumerReplayCompletionStaysInsideRetainedHistory is the finding 3
// regression: the position *after* a replay primitive must stay inside the
// declared retained tail/floor, not merely the sampled pre-step position.
func TestConsumerReplayCompletionStaysInsideRetainedHistory(t *testing.T) {
	t.Run("redo completion beyond retained tail", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		editor.Insert("x")
		syncConsumer(t, consumer, "insert")
		editor.Undo()
		syncConsumer(t, consumer, "undo")
		editor.Redo()
		tracker, snapshot := snapshotTracker(t, consumer)

		frames, latest := pendingFrames(t, consumer) // pending Redo frame
		step := &frames[0].Steps[0]
		if step.Operation != text.HistoryRedo {
			t.Fatalf("pending step operation = %d, want redo", step.Operation)
		}
		// Pre-step position is inside the tail; the completed position is not.
		step.Context.RetainedTo = step.Context.Current
		injectPendingFrames(t, consumer, frames, latest)
		if err := consumer.Sync(); err == nil {
			t.Fatal("Sync accepted a redo whose completed position exceeded the retained tail")
		}
		assertUnavailable(t, consumer, "redo completion beyond tail")
		requireUnchangedCommittedMetadata(t, tracker, snapshot)
	})

	t.Run("undo completion below retained floor", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer := mustConsumer(t, editor)
		editor.Insert("x")
		syncConsumer(t, consumer, "insert")
		editor.Undo()
		tracker, snapshot := snapshotTracker(t, consumer)

		frames, latest := pendingFrames(t, consumer) // pending Undo frame
		step := &frames[0].Steps[0]
		if step.Operation != text.HistoryUndo {
			t.Fatalf("pending step operation = %d, want undo", step.Operation)
		}
		// Pre-step position is at the floor; the completed position would cross it.
		step.Context.RetainedFrom = step.Context.Current
		injectPendingFrames(t, consumer, frames, latest)
		if err := consumer.Sync(); err == nil {
			t.Fatal("Sync accepted an undo whose completed position crossed the retained floor")
		}
		assertUnavailable(t, consumer, "undo completion below floor")
		requireUnchangedCommittedMetadata(t, tracker, snapshot)
	})

	t.Run("advance operation boundaries", func(t *testing.T) {
		redo := func(retainedTo int) text.HistoryStep {
			return text.HistoryStep{
				Operation: text.HistoryRedo, Primitive: text.HistoryInsert,
				Position: 0, Length: 1, BeforeBytes: 0, AfterBytes: 1,
				Context: text.HistoryContext{Current: 0, Save: -1, Detach: -1, RetainedFrom: 0, RetainedTo: retainedTo, Collecting: true},
			}
		}
		if next, err := advanceOperation(redo(0), 0); err == nil || next != 0 {
			t.Fatalf("redo completing exactly at the retained tail = %d/%v, want refusal", next, err)
		}
		if next, err := advanceOperation(redo(1), 0); err != nil || next != 1 {
			t.Fatalf("redo completing one past the tail = %d/%v, want 1", next, err)
		}

		undo := func(retainedFrom int) text.HistoryStep {
			return text.HistoryStep{
				Operation: text.HistoryUndo, Primitive: text.HistoryInsert,
				Position: 0, Length: 1, BeforeBytes: 1, AfterBytes: 2,
				Context: text.HistoryContext{Current: 1, Save: -1, Detach: -1, RetainedFrom: retainedFrom, RetainedTo: 2, Collecting: true},
			}
		}
		if next, err := advanceOperation(undo(1), 1); err == nil || next != 1 {
			t.Fatalf("undo completing exactly at the retained floor = %d/%v, want refusal", next, err)
		}
		if next, err := advanceOperation(undo(0), 1); err != nil || next != 0 {
			t.Fatalf("undo completing one past the floor = %d/%v, want 0", next, err)
		}
	})
}

// TestConsumerRejectsImpossibleSaveContext is the finding 4 regression: a Save
// step must sample save == current and detach == absent, exactly as the source
// does after SetSavePoint, before any event is mapped or applied.
func TestConsumerRejectsImpossibleSaveContext(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*text.HistoryStep)
	}{
		{"save position differs from current", func(step *text.HistoryStep) {
			step.Context.Save = step.Context.Current + 1
			step.Context.RetainedTo = step.Context.Save
		}},
		{"detach not cleared", func(step *text.HistoryStep) {
			step.Context.Detach = step.Context.Current
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor := text.NewEditor("abc")
			consumer := mustConsumer(t, editor)
			tracker, snapshot := snapshotTracker(t, consumer)
			editor.SetSavePoint()
			frames, latest := pendingFrames(t, consumer)
			if frames[0].Steps[0].Operation != text.HistorySave {
				t.Fatalf("pending step = %+v, want save", frames[0].Steps[0])
			}
			test.mutate(&frames[0].Steps[0])
			injectPendingFrames(t, consumer, frames, latest)
			if err := consumer.Sync(); err == nil {
				t.Fatal("Sync accepted a Save step with an impossible sampled context")
			}
			assertUnavailable(t, consumer, test.name)
			requireUnchangedCommittedMetadata(t, tracker, snapshot)
		})
	}
}
