package printing

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestSegmentsMapSourceColorOntoExpandedTabAndUnicode(t *testing.T) {
	job := Job{Text: "a\t猫"}
	row := Row{Text: "a   猫", StartByte: 0, EndByte: len(job.Text), LineNumber: 1}
	red, blue := ui.RGB(172, 77, 64), ui.RGB(49, 92, 167)
	spans := []ColorRun{
		{StartByte: 1, EndByte: 2, Color: red},
		{StartByte: 2, EndByte: len(job.Text), Color: blue},
	}
	got, err := Segments(job, row, spans, 4)
	want := []Segment{
		{Text: "a", Color: ui.RGB(0, 0, 0)},
		{Text: "   ", Color: red},
		{Text: "猫", Color: blue},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %+v, %v; want %+v", got, err, want)
	}
	if job.Text != "a\t猫" || row.Text != "a   猫" {
		t.Fatal("segment conversion changed its source")
	}
}

func TestSegmentsRejectInvalidSpanAndPreserveGrapheme(t *testing.T) {
	job := Job{Text: "🙂b"}
	row := Row{Text: job.Text, StartByte: 0, EndByte: len(job.Text)}
	if _, err := Segments(job, row, []ColorRun{{StartByte: 2, EndByte: 4}}, 4); err == nil {
		t.Fatal("style starts inside an emoji")
	}
	for _, candidate := range []Row{{StartByte: -1, EndByte: 2}, {StartByte: 0, EndByte: 9}} {
		if _, err := Segments(job, candidate, nil, 4); err == nil {
			t.Fatalf("invalid row span %+v was accepted", candidate)
		}
	}
	if _, err := Segments(job, row, nil, 0); err == nil {
		t.Fatal("invalid tab width accepted")
	}
	got, err := Segments(job, row, nil, 4)
	if err != nil || len(got) != 1 || got[0].Text != job.Text {
		t.Fatalf("emoji cluster was split: %+v, %v", got, err)
	}
}

func TestPrintColorModesTransformSourceWithoutChangingText(t *testing.T) {
	row := Row{Text: "ab", StartByte: 0, EndByte: 2}
	red := ui.RGB(172, 77, 64)
	styles := []ColorRun{{StartByte: 1, EndByte: 2, Color: red}}
	for _, mode := range []ColorMode{ColorOnWhite, ColorWYSIWYG, ColorBlackOnWhite, ColorInvertLight} {
		job := Job{Text: "ab", Settings: Settings{ColorMode: mode}}
		got, err := Segments(job, row, styles, 4)
		if err != nil || len(got) != 2 || got[0].Text != "a" || got[1].Text != "b" {
			t.Fatalf("mode %d segments = %+v, %v", mode, got, err)
		}
		switch mode {
		case ColorOnWhite, ColorWYSIWYG:
			if got[0].Color != ui.RGB(0, 0, 0) || got[1].Color != red {
				t.Fatalf("mode %d changed source colors: %+v", mode, got)
			}
		case ColorBlackOnWhite:
			if got[0].Color != ui.RGB(0, 0, 0) || got[1].Color != ui.RGB(0, 0, 0) {
				t.Fatalf("black-on-white colors = %+v", got)
			}
		case ColorInvertLight:
			if red, green, blue := pdfColor(got[1].Color); red != 83 || green != 178 || blue != 191 {
				t.Fatalf("inverted color = %d,%d,%d", red, green, blue)
			}
			if red, green, blue := pdfColor(got[0].Color); red != 255 || green != 255 || blue != 255 {
				t.Fatalf("inverted default color = %d,%d,%d", red, green, blue)
			}
		}
	}
}

func TestPrintBodyBackgroundOnlyForInvertedMode(t *testing.T) {
	for _, mode := range []ColorMode{ColorOnWhite, ColorWYSIWYG, ColorBlackOnWhite, ColorInvertLight} {
		if got := printBodyBackground(Settings{ColorMode: mode}); got != (mode == ColorInvertLight) {
			t.Fatalf("mode %d black body = %v", mode, got)
		}
	}
}
