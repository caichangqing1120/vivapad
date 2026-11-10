package changehistory

import (
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func referenceInteger(t *testing.T, value string) int {
	t.Helper()
	number, err := strconv.Atoi(value)
	if err != nil {
		t.Fatal(err)
	}
	return number
}

func referenceEvent(t *testing.T, value string) Event {
	t.Helper()
	parts := strings.Split(value, ":")
	event := Event{}
	switch parts[0] {
	case "insert":
		event.Kind = Insert
		event.CollectingUndo, event.BeforeSave = parts[3] == "1", parts[4] == "1"
	case "delete":
		event.Kind = Delete
		event.BeforeSave, event.Detached = parts[3] == "1", parts[4] == "1"
	case "undo-insert":
		event.Kind, event.Reverting = UndoInsert, parts[3] == "1"
	case "undo-delete":
		event.Kind, event.Detached = UndoDelete, parts[3] == "1"
	case "save":
		event.Kind = Save
	case "start":
		event.Kind = StartReversion
	case "end":
		event.Kind = EndReversion
	case "reset":
		event.Kind, event.Length = Reset, referenceInteger(t, parts[1])
	default:
		t.Fatal("unknown source change-history event", value)
	}
	if len(parts) >= 3 {
		event.Position, event.Length = referenceInteger(t, parts[1]), referenceInteger(t, parts[2])
	}
	return event
}

func TestTrackerMatchesUnmodifiedOriginalPrefix(t *testing.T) {
	checkTrackerFixture(t, "testdata/scintilla-change-history.tsv", 2293)
}

func TestTrackerMatchesSpanBoundaryCorrectedReference(t *testing.T) {
	checkTrackerFixture(t, "testdata/scintilla-change-history-span-fixed.tsv", 8299)
}

func checkTrackerFixture(t *testing.T, fixture string, wantedStates int) {
	t.Helper()
	input, err := os.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	reader := csv.NewReader(input)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 8
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	var tracker *Tracker
	active, checked, queries := "", 0, 0
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if active != fields[0] {
			tracker, err = New(0)
			if err != nil {
				t.Fatal(err)
			}
			active = fields[0]
		}
		if fields[3] != "-" {
			for _, encoded := range strings.Split(fields[3], ";") {
				if err := tracker.Apply(referenceEvent(t, encoded)); err != nil {
					t.Fatalf("%s/%s %s: %v", active, fields[1], encoded, err)
				}
			}
		}
		var original []byte
		if fields[2] != "-" {
			original, err = hex.DecodeString(fields[2])
			if err != nil {
				t.Fatal(err)
			}
		}
		if tracker.Length() != len(original) {
			t.Fatalf("%s/%s: length=%d, want %d", active, fields[1], tracker.Length(), len(original))
		}
		for query := range 4 {
			values := strings.Split(fields[4+query], ",")
			if len(values) != len(original)+1 {
				t.Fatal("incomplete original query positions", active, fields[1])
			}
			for position, encoded := range values {
				wanted := referenceInteger(t, encoded)
				got := 0
				switch query {
				case 0:
					got = tracker.EditionAt(position)
				case 1:
					got = tracker.EndRun(position)
				case 2:
					got = int(tracker.DeletesAt(position))
				case 3:
					got = tracker.NextDelete(position)
				}
				if got != wanted {
					t.Fatalf("%s/%s query=%d position=%d: got %d, want %d; events=%s", active, fields[1], query, position, got, wanted, fields[3])
				}
				queries++
			}
		}
		checked++
	}
	if checked != wantedStates {
		t.Fatalf("reference fixture contains %d states, want %d", checked, wantedStates)
	}
	t.Logf("matched %d reference states and %d edition/boundary queries", checked, queries)
}

func TestDeletionRestoreDoesNotConsumeAnOlderCompressedSpan(t *testing.T) {
	log := newChangeLog(6)
	log.stack.steps = []changeStep{{count: 2}, {count: 3, position: 0, length: 2}}
	log.stack.changes = []changeSpan{
		{position: 1, edition: 3, count: 4, deletion: true},
		{position: 0, length: 1, edition: 2, count: 1},
	}
	log.deletions.set(2, []editionCount{{edition: 3, count: 5}})
	log.popDeletion(0, 2)
	if log.deletionCount(0, 0) != 2 || log.deletionCount(1, 0) != 2 {
		t.Fatalf("restoration crossed an older step: at start=%d, restored=%d", log.deletionCount(0, 0), log.deletionCount(1, 0))
	}
}

func queryTracker(tracker *Tracker) [][4]int {
	result := make([][4]int, tracker.Length()+1)
	for position := range result {
		result[position] = [4]int{tracker.EditionAt(position), tracker.EndRun(position), int(tracker.DeletesAt(position)), tracker.NextDelete(position)}
	}
	return result
}

func TestInvalidEventsLeaveChangeMetadataUntouched(t *testing.T) {
	for _, event := range []Event{
		{Kind: Insert, Position: -1, Length: 1},
		{Kind: Insert, Position: 7, Length: 1},
		{Kind: Insert, Length: -1},
		{Kind: Insert, Length: int(^uint(0) >> 1)},
		{Kind: Delete, Position: 5, Length: 2},
		{Kind: UndoInsert, Position: -1, Length: 2},
		{Kind: UndoDelete, Position: 1, Length: 2},
		{Kind: Reset, Length: -1},
		{Kind: Save, Length: 1},
		{Kind: Insert, Length: 1, Reverting: true},
		{Kind: Kind(255)},
	} {
		tracker, err := New(6)
		if err != nil {
			t.Fatal(err)
		}
		before := queryTracker(tracker)
		if err := tracker.Apply(event); err == nil {
			t.Errorf("invalid event accepted: %+v", event)
		}
		if !reflect.DeepEqual(queryTracker(tracker), before) {
			t.Fatalf("invalid event changed metadata: %+v", event)
		}
	}
	if _, err := New(-1); err == nil {
		t.Fatal("negative initial length accepted")
	}
}

func TestMismatchedDeletionUndoRefusesBeforeChangingRuns(t *testing.T) {
	for _, event := range []Event{
		{Kind: UndoDelete, Position: 1, Length: 1},
		{Kind: UndoDelete, Position: 2, Length: 2},
	} {
		tracker, err := New(6)
		if err != nil {
			t.Fatal(err)
		}
		if err := tracker.Apply(Event{Kind: Insert, Position: 2, Length: 1, CollectingUndo: true}); err != nil {
			t.Fatal(err)
		}
		if err := tracker.Apply(Event{Kind: Delete, Position: 1, Length: 2}); err != nil {
			t.Fatal(err)
		}
		before := queryTracker(tracker)
		if err := tracker.Apply(event); err == nil {
			t.Fatalf("mismatched original deletion accepted: %+v", event)
		}
		if !reflect.DeepEqual(queryTracker(tracker), before) {
			t.Fatalf("mismatched deletion mutated metadata: %+v", event)
		}
		if err := tracker.Apply(Event{Kind: UndoDelete, Position: 1, Length: 2}); err != nil {
			t.Fatal("failed attempt consumed valid Undo", err)
		}
		if tracker.Length() != 7 || tracker.EditionAt(2) != 3 {
			t.Fatal("correct retry failed to restore inserted edition")
		}
	}
}

func TestUnrestorableReversionRefusesBeforeChangingMetadata(t *testing.T) {
	tracker, err := New(8)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []Event{
		{Kind: StartReversion},
		{Kind: UndoInsert, Position: 1, Length: 1, Reverting: true},
		{Kind: UndoInsert, Position: 0, Length: 1},
	} {
		if err := tracker.Apply(event); err != nil {
			t.Fatal(err)
		}
	}
	before := queryTracker(tracker)
	for range 2 {
		if err := tracker.Apply(Event{Kind: Insert, Position: 0, Length: 1, BeforeSave: true, CollectingUndo: true}); err == nil {
			t.Fatal("reversion without enough deletion editions was accepted")
		}
		if !reflect.DeepEqual(queryTracker(tracker), before) {
			t.Fatal("refused reversion changed metadata")
		}
	}
	if err := tracker.Apply(Event{Kind: Insert, Position: 0, Length: 1, CollectingUndo: true}); err != nil {
		t.Fatal("refusal damaged subsequent valid insertion", err)
	}
}

func TestLargeOriginalDocumentUsesRunMetadata(t *testing.T) {
	const length = 1 << 30
	tracker, err := New(length)
	if err != nil {
		t.Fatal(err)
	}
	if tracker.EndRun(0) != length || tracker.NextDelete(0) != length {
		t.Fatal("large unchanged document was not one original run")
	}
	if err := tracker.Apply(Event{Kind: Insert, Position: length / 2, Length: 2, CollectingUndo: true}); err != nil {
		t.Fatal(err)
	}
	if tracker.Length() != length+2 || tracker.EditionAt(length/2) != 3 || tracker.EditionAt(length/2+2) != 0 {
		t.Fatal("large document insertion was not represented locally")
	}
}

var benchmarkMetadata *Tracker

func BenchmarkLargeOriginalMetadata(b *testing.B) {
	b.ReportAllocs()
	defer func() { benchmarkMetadata = nil }()
	for range b.N {
		tracker, err := New(1 << 30)
		if err != nil || tracker.Length() != 1<<30 {
			b.Fatal(err)
		}
		benchmarkMetadata = tracker
	}
}

func FuzzTrackerMalformedEventsDoNotCorruptQueries(f *testing.F) {
	f.Add([]byte{1, 0, 2, 1, 2, 0, 1, 0, 4, 0, 1, 0})
	f.Add([]byte{6, 0, 0, 0, 3, 1, 2, 8, 1, 1, 2, 3})
	f.Add([]byte{6, 1, 1, 0, 3, 2, 2, 8, 3, 1, 2, 0, 1, 1, 2, 3})
	f.Fuzz(func(t *testing.T, input []byte) {
		tracker, err := New(8)
		if err != nil {
			t.Fatal(err)
		}
		for index := 0; index+3 < len(input) && index < 1024; index += 4 {
			event := Event{Kind: Kind(input[index] % 10), Position: int(input[index+1]%66) - 1, Length: int(input[index+2]%66) - 1}
			flags := input[index+3]
			event.CollectingUndo, event.BeforeSave = flags&1 != 0, flags&2 != 0
			event.Detached, event.Reverting = flags&4 != 0, flags&8 != 0
			before := queryTracker(tracker)
			applyError := tracker.Apply(event)
			after := queryTracker(tracker)
			if applyError != nil && !reflect.DeepEqual(before, after) {
				t.Fatalf("refused malformed event mutated queries: %+v", event)
			}
		}
	})
}
