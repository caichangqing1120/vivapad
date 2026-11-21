package changehistory

import (
	"reflect"
	"testing"
)

func TestTrackerCloneKeepsCommittedMetadataIndependent(t *testing.T) {
	tracker, err := New(4)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []Event{
		{Kind: Insert, Position: 1, Length: 2, CollectingUndo: true},
		{Kind: Delete, Position: 3, Length: 1},
		{Kind: StartReversion},
		{Kind: UndoInsert, Position: 1, Length: 1, Reverting: true},
	} {
		if err := tracker.Apply(event); err != nil {
			t.Fatalf("fixture event %+v: %v", event, err)
		}
	}
	want := make([][3]int, tracker.Length()+1)
	for position := range want {
		want[position] = [3]int{tracker.EditionAt(position), int(tracker.DeletesAt(position)), tracker.EndRun(position)}
	}
	copy := tracker.clone()
	if copy == tracker || !reflect.DeepEqual(copy, tracker) {
		t.Fatal("clone did not preserve independent initial metadata")
	}
	for _, event := range []Event{{Kind: Insert, Position: 0, Length: 1, CollectingUndo: true}, {Kind: Save}} {
		if err := copy.Apply(event); err != nil {
			t.Fatalf("candidate event %+v: %v", event, err)
		}
	}
	if tracker.Length() != len(want)-1 {
		t.Fatalf("candidate mutation changed committed length: %d", tracker.Length())
	}
	for position, expected := range want {
		got := [3]int{tracker.EditionAt(position), int(tracker.DeletesAt(position)), tracker.EndRun(position)}
		if got != expected {
			t.Fatalf("candidate mutation changed committed metadata at %d: got %v, want %v", position, got, expected)
		}
	}
}
