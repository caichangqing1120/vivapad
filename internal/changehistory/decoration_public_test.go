package changehistory

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestConsumerDecorationsCopyExactRunsAndDeletionPoints(t *testing.T) {
	editor := text.NewEditor("a漢b\n")
	consumer, err := NewConsumer(editor)
	if err != nil {
		t.Fatal(err)
	}
	editor.MoveTo(1, false)
	editor.Insert("é")
	if err := consumer.Sync(); err != nil {
		t.Fatal(err)
	}
	runs, points, err := consumer.Decorations()
	if err != nil || !reflect.DeepEqual(runs, []DecorationRun{{Start: 1, End: 3, Edition: 3}}) || len(points) != 0 {
		t.Fatalf("insert decorations = %+v, %+v, %v", runs, points, err)
	}
	editor.SetSavePoint()
	if err := consumer.Sync(); err != nil {
		t.Fatal(err)
	}
	editor.MoveTo(3, false)
	editor.DeleteForward()
	if err := consumer.Sync(); err != nil {
		t.Fatal(err)
	}
	runs, points, err = consumer.Decorations()
	if err != nil || !reflect.DeepEqual(runs, []DecorationRun{{Start: 1, End: 3, Edition: 2}}) || !reflect.DeepEqual(points, []DeletionPoint{{Position: 6, Editions: 4}}) {
		t.Fatalf("saved/deleted decorations = %+v, %+v, %v", runs, points, err)
	}
	runs[0].Edition = 0
	points[0].Editions = 0
	runs, points, err = consumer.Decorations()
	if err != nil || runs[0].Edition != 2 || points[0].Editions != 4 {
		t.Fatalf("caller mutated consumer metadata: %+v, %+v, %v", runs, points, err)
	}
	consumer.invalidate(errors.New("lost checkpoint"))
	if runs, points, err := consumer.Decorations(); err == nil || runs != nil || points != nil {
		t.Fatalf("unavailable metadata was exposed: %+v, %+v, %v", runs, points, err)
	}
}
