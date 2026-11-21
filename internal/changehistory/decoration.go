package changehistory

import "fmt"

// DecorationRun is a nonempty UTF-8 byte range with one original edition (1..4).
type DecorationRun struct {
	Start, End, Edition int
}

// DeletionPoint retains every original edition bit at an insertion position.
type DeletionPoint struct {
	Position int
	Editions uint8
}

// Decorations copies the last committed sparse metadata for presentation. It
// neither consumes pending frames nor exposes the Tracker's mutable buffers.
func (consumer *Consumer) Decorations() ([]DecorationRun, []DeletionPoint, error) {
	if consumer == nil {
		return nil, nil, fmt.Errorf("nil change-history consumer")
	}
	if consumer.unavailable {
		return nil, nil, consumer.reason
	}
	if consumer.tracker == nil {
		return nil, nil, fmt.Errorf("change-history consumer has no metadata")
	}
	tracker := consumer.tracker
	var runs []DecorationRun
	for start := 0; start < tracker.Length(); {
		end := tracker.EndRun(start)
		if edition := tracker.EditionAt(start); edition != 0 {
			runs = append(runs, DecorationRun{Start: start, End: end, Edition: edition})
		}
		start = end
	}
	var points []DeletionPoint
	for position := 0; position <= tracker.Length(); position = tracker.NextDelete(position) {
		if editions := tracker.DeletesAt(position); editions != 0 {
			points = append(points, DeletionPoint{Position: position, Editions: uint8(editions)})
		}
	}
	return runs, points, nil
}
