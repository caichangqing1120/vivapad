package changehistory

// H02 metadata budget and prune/unavailable contract tests.
//
// The consumer retains no text, no per-byte array and no frame/step replay log.
// Retained metadata is the change-history engine's sparse edition runs, sparse
// deletion editions, counted restoration spans and the fixed headers. These
// tests measure that retention against document length and edit count, and pin
// the over-budget / missing-checkpoint contract: invalidate + release, never a
// silent empty "no changes" answer.

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/text"
)

func trackerByteBreakdown(tracker *Tracker) string {
	if tracker == nil {
		return "nil"
	}
	header := int(unsafe.Sizeof(*tracker))
	log := &tracker.log
	runs := fmt.Sprintf("runs=%d/cap%d", len(log.insertions.runs), cap(log.insertions.runs))
	deletions := fmt.Sprintf("deletions=%d/cap%d", len(log.deletions.positions), cap(log.deletions.positions))
	values := 0
	for _, value := range log.deletions.values {
		values += cap(value)
	}
	stack := fmt.Sprintf("steps=%d/cap%d changes=%d/cap%d", len(log.stack.steps), cap(log.stack.steps), len(log.stack.changes), cap(log.stack.changes))
	reversions := "absent"
	if tracker.reversions != nil {
		revValues := 0
		for _, value := range tracker.reversions.deletions.values {
			revValues += cap(value)
		}
		reversions = fmt.Sprintf("present(runs=%d deletions=%d/%d stack=%d/%d)",
			len(tracker.reversions.insertions.runs), len(tracker.reversions.deletions.positions), revValues,
			len(tracker.reversions.stack.steps), len(tracker.reversions.stack.changes))
	}
	return fmt.Sprintf("header=%dB %s %s edition-values=%d %s reversions=%s total=%dB",
		header, runs, deletions, values, stack, reversions, estimateTrackerBytes(tracker))
}

// TestConsumerMetadataIsIndependentOfDocumentLength proves there is no
// per-document-byte retention: a multi-megabyte document costs the same fixed
// metadata as a three-byte document.
func TestConsumerMetadataIsIndependentOfDocumentLength(t *testing.T) {
	small := text.NewEditor("abc")
	smallConsumer := mustConsumer(t, small)
	syncConsumer(t, smallConsumer, "small document")

	bigText := strings.Repeat("0123456789abcdef\n", 512*1024)
	big := text.NewEditor(bigText)
	bigConsumer := mustConsumer(t, big)
	syncConsumer(t, bigConsumer, "big document")

	t.Logf("small document bytes=%d metadata=%d", len(small.String()), smallConsumer.metadataBytes)
	t.Logf("big document bytes=%d metadata=%d", len(big.String()), bigConsumer.metadataBytes)
	t.Logf("big breakdown: %s", trackerByteBreakdown(bigConsumer.tracker))
	if bigConsumer.metadataBytes != smallConsumer.metadataBytes {
		t.Fatalf("metadata depends on document length: %d bytes for %d-byte document, %d for %d-byte document",
			bigConsumer.metadataBytes, len(big.String()), smallConsumer.metadataBytes, len(small.String()))
	}
	if bigConsumer.metadataBytes > 1024 {
		t.Fatalf("unchanged big document retained %d metadata bytes", bigConsumer.metadataBytes)
	}

	// A few real edits on the large document stay byte-local too.
	big.MoveTo(0, false)
	big.Insert("é")
	syncConsumer(t, bigConsumer, "big document insert")
	delta := bigConsumer.metadataBytes - smallConsumer.metadataBytes
	if delta <= 0 {
		t.Fatalf("insert on big document did not add edition metadata: %s", trackerByteBreakdown(bigConsumer.tracker))
	}
	if delta > 4096 {
		t.Fatalf("one insert charged %d metadata bytes", delta)
	}
	t.Logf("one multibyte insert on %d-byte document: +%d metadata bytes", len(big.String()), delta)

	// 256 MiB logical document: construction remains a fixed-size run.
	huge, err := New(256 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if bytes := estimateTrackerBytes(huge); bytes > 1024 {
		t.Fatalf("256 MiB unchanged document retained %d metadata bytes", bytes)
	} else {
		t.Logf("256 MiB unchanged document metadata=%d (%s)", bytes, trackerByteBreakdown(huge))
	}
}

// TestConsumerMetadataBoundedForManyEdits drives a long mixed edit workload
// through the real producer and consumer. It records the metadata high-water
// mark against the document length and edit count and proves that consumed
// frames/steps are not retained: repeated empty Syncs and Save-only frames leave
// the retained charge unchanged.
func TestConsumerMetadataBoundedForManyEdits(t *testing.T) {
	editor := text.NewEditor("abc")
	consumer := mustConsumer(t, editor)
	initial := consumer.metadataBytes

	// Save-only frames must not add retained frame/step metadata.
	for index := 0; index < 256; index++ {
		editor.SetSavePoint()
		syncConsumer(t, consumer, "save-only")
	}
	if consumer.frames != 256 {
		t.Fatalf("frames consumed = %d, want 256", consumer.frames)
	}
	if consumer.metadataBytes != initial {
		t.Fatalf("Save-only frames changed retained metadata: %d -> %d", initial, consumer.metadataBytes)
	}
	if consumer.steps != 256 {
		t.Fatalf("steps consumed = %d, want 256", consumer.steps)
	}

	// Deterministic mixed workload: coalesced typing, deletion, saves and
	// undo/redo. Every accepted primitive is consumed immediately, like a live
	// view would. Kept inside the producer's own 256-unit history window so the
	// measurement is not the documented trim invalidation path.
	high := consumer.metadataBytes
	operations := 0
	for round := 0; round < 40; round++ {
		for range 20 {
			editor.Insert("x")
			syncConsumer(t, consumer, "workload insert")
			operations++
			if consumer.metadataBytes > high {
				high = consumer.metadataBytes
			}
		}
		editor.SetSavePoint()
		syncConsumer(t, consumer, "workload save")
		operations++
		for range 5 {
			if editor.DeleteBackward() {
				syncConsumer(t, consumer, "workload delete")
				operations++
			}
		}
		if round%8 == 0 && editor.CanUndo() {
			editor.Undo()
			syncConsumer(t, consumer, "workload undo")
			operations++
			editor.Redo()
			syncConsumer(t, consumer, "workload redo")
			operations++
		}
		if consumer.unavailable {
			t.Fatalf("workload invalidated the consumer at operation %d", operations)
		}
	}
	documentBytes := len(editor.String())
	if consumer.metadataBytes > high {
		high = consumer.metadataBytes
	}
	if int64(high) != estimateTrackerBytes(consumer.tracker) {
		t.Fatalf("reported metadata %d != measured %d", high, estimateTrackerBytes(consumer.tracker))
	}
	if high > maxConsumerMetadataBytes {
		t.Fatalf("workload exceeded the consumer metadata budget: %d > %d", high, maxConsumerMetadataBytes)
	}
	t.Logf("workload: %d recorded operations, document=%d bytes, frames=%d steps=%d, metadata high-water=%d bytes (%.3f%% of document, %.4f%% of budget)",
		operations, documentBytes, consumer.frames, consumer.steps, high,
		100*float64(high)/float64(documentBytes), 100*float64(high)/float64(maxConsumerMetadataBytes))
	t.Logf("workload breakdown: %s", trackerByteBreakdown(consumer.tracker))

	// Retained metadata must not grow when Sync consumes nothing.
	beforeBytes, beforeFrames := consumer.metadataBytes, consumer.frames
	for range 100 {
		syncConsumer(t, consumer, "empty sync")
	}
	if consumer.metadataBytes != beforeBytes {
		t.Fatalf("empty Syncs changed retained metadata: %d -> %d", beforeBytes, consumer.metadataBytes)
	}
	if consumer.frames != beforeFrames {
		t.Fatalf("empty Syncs consumed frames: %d -> %d", beforeFrames, consumer.frames)
	}
}

// TestConsumerOverBudgetInvalidatesAndReleases pins the over-budget contract:
// explicit unavailable, metadata released, queries refused, and the producer is
// never queried as if nothing changed.
func TestConsumerOverBudgetInvalidatesAndReleases(t *testing.T) {
	editor := text.NewEditor("abcdef")
	consumer := mustConsumer(t, editor)
	syncConsumer(t, consumer, "bootstrap")
	oldTracker := consumer.tracker
	snapshot := queryTracker(oldTracker)
	consumer.budget = consumer.metadataBytes // any retained growth exceeds it

	editor.MoveTo(0, false)
	if !editor.DeleteForward() {
		t.Fatal("DeleteForward refused")
	}
	err := consumer.Sync()
	if err == nil {
		t.Fatal("Sync accepted metadata beyond the configured budget")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "budget") {
		t.Fatalf("over-budget error does not report the budget: %v", err)
	}
	assertUnavailable(t, consumer, "over budget")
	if !reflect.DeepEqual(queryTracker(oldTracker), snapshot) {
		t.Fatal("over-budget rejection mutated the committed metadata")
	}
	if mask, err := consumer.LineMask(0, 6, 6); err == nil {
		t.Fatalf("LineMask returned %d instead of reporting unavailable history", mask)
	} else {
		t.Logf("over-budget LineMask refusal: %v", err)
	}
}

// TestConsumerMissingCheckpointReleasesRetainedMetadata pins the missing
// checkpoint/prune contract with measured retention: the last good charge is
// released, not kept as usable history and not reported as "no changes".
func TestConsumerMissingCheckpointReleasesRetainedMetadata(t *testing.T) {
	editor := text.NewEditor("abcdef")
	consumer := mustConsumer(t, editor)
	editor.SetSavePoint()
	editor.Insert("x")
	syncConsumer(t, consumer, "retained metadata")
	retained := consumer.metadataBytes
	if retained <= 0 {
		t.Fatalf("expected nonzero retained metadata, got %d", retained)
	}
	t.Logf("retained before checkpoint loss: %d bytes (%s)", retained, trackerByteBreakdown(consumer.tracker))

	// Evict the producer's retained frame prefix without consuming it.
	for index := 0; index < 300; index++ {
		editor.SetSavePoint()
	}
	err := consumer.Sync()
	if err == nil {
		t.Fatal("lost checkpoint was silently accepted")
	}
	assertUnavailable(t, consumer, "missing checkpoint")
	if consumer.metadataBytes != 0 {
		t.Fatalf("unavailable consumer still charges %d metadata bytes", consumer.metadataBytes)
	}
	t.Logf("missing-checkpoint refusal: %v", err)
}

// TestConsumerProducerTrimInvalidates pins the second missing-checkpoint path:
// the producer's ordinary Undo-unit trim invalidates itself instead of rebasing
// the metadata, so a consumer that keeps reading must release its retained
// metadata and report unavailable rather than "no changes".
func TestConsumerProducerTrimInvalidates(t *testing.T) {
	editor := text.NewEditor("abc")
	consumer := mustConsumer(t, editor)
	for index := 0; index < 256; index++ {
		editor.BreakUndoGroup()
		editor.Insert("x")
		syncConsumer(t, consumer, "retained unit")
	}
	retained := consumer.metadataBytes
	if retained <= 0 {
		t.Fatalf("expected nonzero retained metadata, got %d", retained)
	}
	editor.BreakUndoGroup()
	editor.Insert("x")
	err := consumer.Sync()
	if err == nil {
		t.Fatal("ordinary trim was silently accepted as a complete checkpoint")
	}
	assertUnavailable(t, consumer, "producer trim")
	t.Logf("producer trim after 256 units: retained=%d bytes released, refusal=%v", retained, err)
}
