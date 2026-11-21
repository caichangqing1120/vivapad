package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestANSIStylePreservesMarkupAndByteRanges(t *testing.T) {
	const line = "\u754c \x1b[31m\u00e9x\x1b[Kmore\x1b[0m plain \x1b[2Junchanged \x1b["
	var h Highlighter
	got := h.Highlight(".ans", line, nil)
	for _, expected := range []struct {
		value string
		color gfx.Color
	}{
		{"\u00e9x", gfx.RGB(205, 0, 0)},
		{"more", gfx.RGB(205, 0, 0)},
		{"\x1b[31m", gfx.RGB(113, 76, 144)},
		{"\x1b[K", gfx.RGB(113, 76, 144)},
		{"\x1b[2J", gfx.RGB(177, 39, 58)},
	} {
		found := false
		for _, run := range got {
			if line[run.Start:run.End] == expected.value && run.Color == expected.color {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no run for %q in %+v", expected.value, got)
		}
	}
	if len(got) == 0 || got[len(got)-1].End != len(line) || got[len(got)-1].Color != gfx.RGB(177, 39, 58) {
		t.Fatalf("unterminated sequence was not marked: %+v", got)
	}
	if runs := h.Highlight(".ans", "next line", nil); len(runs) != 0 {
		t.Fatalf("color leaked to another line: %+v", runs)
	}
	lexer := lexers.Get("Escape Sequence (ANSI)")
	if lexer == nil {
		t.Fatal("ANSI lexer is missing")
	}
	const mixed = line + "\r\n\x1b[32mgreen\r\x1b[0mplain"
	it, err := lexer.Tokenise(nil, mixed)
	if err != nil {
		t.Fatal(err)
	}
	var joined strings.Builder
	for token := it(); token != chroma.EOF; token = it() {
		joined.WriteString(token.Value)
	}
	if joined.String() != mixed {
		t.Fatal("lexer changed source bytes")
	}
}

func TestANSIBackgroundUsesDocumentByteOffsetsAfterEdit(t *testing.T) {
	const value = "\u754c\x1b[31;44m\u00e9\r\nplain\x1b[45mend\x1b[mnormal"
	var background ANSIBackground
	layers := background.AppendLayers(value, nil)
	if len(layers) != 2 {
		t.Fatalf("backgrounds = %+v", layers)
	}
	for i, want := range []struct {
		color gfx.Color
		text  string
	}{
		{gfx.RGB(0, 0, 255), "\u00e9"},
		{gfx.RGB(205, 0, 205), "end"},
	} {
		start := strings.Index(value, want.text)
		if layer := layers[i]; layer.Color != want.color || len(layer.Spans) != 1 || layer.Spans[0] != (text.Span{Start: start, End: start + len(want.text)}) {
			t.Fatalf("layer %d = %+v", i, layer)
		}
	}
	updated := strings.Replace(value, ";44", "", 1)
	layers = background.AppendLayers(updated, nil)
	start := strings.Index(updated, "end")
	if len(layers) != 1 || len(layers[0].Spans) != 1 || layers[0].Spans[0] != (text.Span{Start: start, End: start + 3}) {
		t.Fatalf("stale backgrounds after edit: %+v", layers)
	}
}

func TestANSIIgnoresExtendedColorOperandsAndKeepsBrightColors(t *testing.T) {
	const line = "\x1b[38;5;31mplain\x1b[48;2;40;0;42mdefault\x1b[1;31mbright\x1b[31mred"
	var h Highlighter
	styles := make(map[string]gfx.Color)
	for _, run := range h.Highlight(".ans", line, nil) {
		styles[line[run.Start:run.End]] = run.Color
	}
	if styles["bright"] != gfx.RGB(255, 0, 0) || styles["red"] != gfx.RGB(205, 0, 0) ||
		styles["plain"].Alpha != 0 || styles["default"].Alpha != 0 {
		t.Fatalf("unexpected colors: %v", styles)
	}
	var backgrounds ANSIBackground
	if layers := backgrounds.AppendLayers(line, nil); len(layers) != 0 {
		t.Fatalf("extended operands became basic background commands: %+v", layers)
	}
}
