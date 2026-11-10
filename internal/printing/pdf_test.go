package printing

import (
	"bytes"
	"os"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestPDFRendererUsesEmbeddedFontMetricsAndPrinterPage(t *testing.T) {
	font, err := os.ReadFile("fonts/NotoSansSC.ttf")
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := newPDFRenderer(font, 13, 4)
	if err != nil {
		t.Fatal(err)
	}
	text := renderer.Metrics()
	if text.LineHeight <= 13 || text.TabWidth != 4 || text.Advance("中文") <= text.Advance("a") {
		t.Fatalf("PDF font metrics = %+v, Chinese=%v", text, text.Advance("中文"))
	}
	if renderer.BandHeight(BandFont{Size: 14, Bold: true, Italic: true}) <= renderer.BandHeight(BandFont{}) {
		t.Fatal("larger header font did not reserve a taller band")
	}
	device := DeviceMetrics{
		PaperWidth: 595.28, PaperHeight: 841.89,
		PhysicalMargins:  Insets{Left: 18, Top: 18, Right: 18, Bottom: 18},
		HeaderLineHeight: 11, FooterLineHeight: 11,
	}
	job := Job{Text: "alpha\t中文\nbeta", Title: "draft.txt", Settings: Settings{LineNumbers: true, ColorMode: ColorOnWhite, Header: TextBand{Left: "draft.txt"}, HeaderFont: BandFont{Size: 14, Bold: true, Italic: true}}}
	pages, err := Paginate(job, device, text, PageVariables{})
	if err != nil || len(pages) != 1 {
		t.Fatalf("PDF pagination = %d pages, %v", len(pages), err)
	}
	pages[0].Styles = [][]ColorRun{{{StartByte: 0, EndByte: 5, Color: ui.RGB(64, 102, 191)}}, nil}
	output, err := renderer.Render(job, pages, device)
	if err != nil || len(output) < 1000 || !bytes.HasPrefix(output, []byte("%PDF-")) {
		t.Fatalf("rendered PDF = %d bytes, %v", len(output), err)
	}
	if !bytes.Contains(output, []byte("/Type /Page")) {
		t.Fatal("PDF lacks a printable page")
	}
	if path := os.Getenv("VIVAPAD_PDF_PREVIEW_OUTPUT"); path != "" {
		if err := os.WriteFile(path, output, 0600); err != nil {
			t.Fatal(err)
		}
	}
	inverted := job
	inverted.Settings.ColorMode = ColorInvertLight
	invertedPDF, err := renderer.Render(inverted, pages, device)
	if err != nil || !bytes.HasPrefix(invertedPDF, []byte("%PDF-")) {
		t.Fatalf("inverted PDF = %d bytes, %v", len(invertedPDF), err)
	}
	if path := os.Getenv("VIVAPAD_PDF_INVERT_PREVIEW_OUTPUT"); path != "" {
		if err := os.WriteFile(path, invertedPDF, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPDFRendererRejectsInvalidFontAndPage(t *testing.T) {
	if _, err := newPDFRenderer([]byte("not a font"), 13, 4); err == nil {
		t.Fatal("invalid font accepted")
	}
	font, err := os.ReadFile("fonts/NotoSansSC.ttf")
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := newPDFRenderer(font, 13, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderer.Render(Job{Text: "abc"}, []Page{{Number: 1, Rows: []Row{{StartByte: 0, EndByte: 3, Text: "wrong"}}}}, DeviceMetrics{PaperWidth: 595, PaperHeight: 842}); err == nil {
		t.Fatal("inconsistent page text accepted")
	}
	if _, err := renderer.Render(Job{Text: "🙂"}, []Page{{Number: 1, Rows: []Row{{StartByte: 0, EndByte: len("🙂"), Text: "🙂"}}}}, DeviceMetrics{PaperWidth: 595, PaperHeight: 842}); err == nil {
		t.Fatal("missing font glyph was silently omitted")
	}
}

func TestPDFBandRulesStayOutsideTextBody(t *testing.T) {
	bounds := PageBounds{Top: 50, Bottom: 800}
	if top := pdfBandRuleY(bounds, true); top > 46 {
		t.Fatalf("header rule %v enters first row", top)
	}
	if bottom := pdfBandRuleY(bounds, false); bottom < 804 {
		t.Fatalf("footer rule %v enters last row", bottom)
	}
}
