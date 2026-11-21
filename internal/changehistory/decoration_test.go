package changehistory

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestConsumerDecorationsRetainExactBytesAndDeletionEditions(t *testing.T) {
	editor := text.NewEditor("a\u6f22b\n")
	consumer := mustConsumer(t, editor)
	check := func(wantRuns []DecorationRun, wantPoints []DeletionPoint) {
		t.Helper()
		runs, points, err := consumer.Decorations()
		if err != nil || !reflect.DeepEqual(runs, wantRuns) || !reflect.DeepEqual(points, wantPoints) {
			t.Fatalf("decorations = %+v / %+v / %v, want %+v / %+v", runs, points, err, wantRuns, wantPoints)
		}
	}
	check(nil, nil)
	editor.MoveTo(1, false)
	editor.Insert("\u00e9")
	check(nil, nil)
	syncConsumer(t, consumer, "insert UTF-8")
	check([]DecorationRun{{Start: 1, End: 3, Edition: 3}}, nil)
	editor.SetSavePoint()
	syncConsumer(t, consumer, "save")
	check([]DecorationRun{{Start: 1, End: 3, Edition: 2}}, nil)
	editor.MoveTo(3, false)
	editor.DeleteForward()
	syncConsumer(t, consumer, "delete b")
	check([]DecorationRun{{Start: 1, End: 3, Edition: 2}}, []DeletionPoint{{Position: 6, Editions: 4}})

	runs, points, err := consumer.Decorations()
	if err != nil {
		t.Fatal(err)
	}
	runs[0].Edition, points[0].Editions = 0, 0
	check([]DecorationRun{{Start: 1, End: 3, Edition: 2}}, []DeletionPoint{{Position: 6, Editions: 4}})

	editor.SetSavePoint()
	syncConsumer(t, consumer, "save deletion")
	check([]DecorationRun{{Start: 1, End: 3, Edition: 2}}, []DeletionPoint{{Position: 6, Editions: 2}})
}

func TestConsumerDecorationsKeepEOFPointsAndRefuseUnavailableHistory(t *testing.T) {
	editor := text.NewEditor("x")
	consumer := mustConsumer(t, editor)
	editor.DeleteBackward()
	syncConsumer(t, consumer, "delete all")
	runs, points, err := consumer.Decorations()
	if err != nil || len(runs) != 0 || !reflect.DeepEqual(points, []DeletionPoint{{Position: 0, Editions: 4}}) {
		t.Fatalf("empty document deletion = %+v / %+v / %v", runs, points, err)
	}
	consumer.invalidate(errors.New("fixture lost checkpoint"))
	runs, points, err = consumer.Decorations()
	if err == nil || runs != nil || points != nil {
		t.Fatalf("unavailable history exposed decorations: %+v / %+v / %v", runs, points, err)
	}
	var absent *Consumer
	if _, _, err := absent.Decorations(); err == nil {
		t.Fatal("nil consumer accepted")
	}
}

func checkFixtureDecorations(t *testing.T, tracker *Tracker, row fixtureRow, length int) {
	t.Helper()
	consumer := &Consumer{tracker: tracker}
	runs, points, err := consumer.Decorations()
	if err != nil {
		t.Fatal(err)
	}
	for position := 0; position <= length; position++ {
		edition, mask := 0, 0
		for _, run := range runs {
			if run.Start <= position && position < run.End {
				if edition != 0 {
					t.Fatal("decoration runs overlap")
				}
				edition = run.Edition
			}
		}
		for _, point := range points {
			if point.Position == position {
				mask |= int(point.Editions)
			}
		}
		if position < length && edition != row.queries[0][position] || mask != row.queries[2][position] {
			t.Fatalf("%s/%s: decoration position %d = edition %d/deletions %d, original %d/%d", row.caseName, row.step, position, edition, mask, row.queries[0][position], row.queries[2][position])
		}
	}
}
