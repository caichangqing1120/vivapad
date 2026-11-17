package changehistory

import "fmt"

type DecorationRun struct {
	Start, End, Edition int
}

type DeletionPoint struct {
	Position int
	Editions uint8
}

// Decorations copies the last committed sparse metadata for presentation.
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
