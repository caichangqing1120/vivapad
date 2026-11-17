package changehistory

import (
	"testing"

	"github.com/vivaui/viva/text"
)

func TestConsumerTracksPublishedHistoryFrames(t *testing.T) {
	editor := text.NewEditor("ab\ncd")
	consumer, err := NewConsumer(editor)
	if err != nil {
		t.Fatal(err)
	}
	if mask, err := consumer.LineMask(0, 2, 3); err != nil || mask != 0 {
		t.Fatalf("initial line mask = %d, %v", mask, err)
	}
	editor.MoveTo(1, false)
	editor.Insert("中")
	if err := consumer.Sync(); err != nil {
		t.Fatal(err)
	}
	if mask, err := consumer.LineMask(0, len("a中b"), len("a中b\n")); err != nil || mask == 0 {
		t.Fatalf("committed insert is absent from line margin: %d, %v", mask, err)
	}
	editor.SetSavePoint()
	if err := consumer.Sync(); err != nil {
		t.Fatal(err)
	}
	if !editor.Undo() {
		t.Fatal("source Undo failed")
	}
	if err := consumer.Sync(); err != nil {
		t.Fatal(err)
	}
	if consumer.tracker == nil || consumer.tracker.Length() != len(editor.String()) {
		t.Fatal("Undo did not retain coherent committed metadata")
	}
}

func TestConsumerRejectsMalformedFrameWithoutPublishingMarkers(t *testing.T) {
	editor := text.NewEditor("abc")
	consumer, err := NewConsumer(editor)
	if err != nil {
		t.Fatal(err)
	}
	committed := consumer.tracker
	editor.MoveTo(0, false)
	editor.Insert("x")
	frames, latest, ok := editor.HistorySince(consumer.state.cursor)
	if !ok || len(frames) == 0 || len(frames[0].Steps) == 0 {
		t.Fatal("producer did not publish the fixture edit")
	}
	frames[0].Steps[0].AfterBytes++
	consumer.read = func(text.HistoryCursor) ([]text.HistoryFrame, text.HistoryCursor, bool) {
		return frames, latest, true
	}
	if err := consumer.Sync(); err == nil || consumer.tracker != nil || !consumer.unavailable {
		t.Fatalf("malformed frame remained visible: err=%v metadata=%p", err, consumer.tracker)
	}
	if committed.Length() != len("abc") || committed.EditionAt(0) != 0 {
		t.Fatal("candidate frame partially changed the committed tracker")
	}
	if mask, err := consumer.LineMask(0, 3, 3); err == nil {
		t.Fatalf("unavailable history returned a clean-looking mask %d", mask)
	}
}

func TestConsumerRefusesExpiredFrameWindowAndMetadataBudget(t *testing.T) {
	t.Run("expired producer cursor", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer, err := NewConsumer(editor)
		if err != nil {
			t.Fatal(err)
		}
		for range 257 {
			editor.SetSavePoint()
		}
		if err := consumer.Sync(); err == nil || !consumer.unavailable || consumer.tracker != nil {
			t.Fatalf("expired producer window appeared complete: %v", err)
		}
	})
	t.Run("retained metadata budget", func(t *testing.T) {
		editor := text.NewEditor("abc")
		consumer, err := NewConsumer(editor)
		if err != nil {
			t.Fatal(err)
		}
		consumer.budget = consumer.metadataBytes
		editor.MoveTo(1, false)
		editor.Insert("x")
		if err := consumer.Sync(); err == nil || !consumer.unavailable || consumer.tracker != nil || consumer.metadataBytes != 0 {
			t.Fatalf("over-budget metadata remained available: %v", err)
		}
	})
}
