package printing

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
)

type failingPDFWriter struct {
	err error
}

func (writer failingPDFWriter) Write([]byte) (int, error) { return 0, writer.err }

func (renderer *pdfRenderer) renderBytesForTest(job Job, pages []Page, device DeviceMetrics) ([]byte, error) {
	var output bytes.Buffer
	if err := renderer.writePDF(&output, job, pages, device); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func TestPDFRendererReportsSpoolWriteFailure(t *testing.T) {
	font, err := os.ReadFile("fonts/NotoSansSC.ttf")
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := newPDFRenderer(font, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	writeErr := errors.New("spool write failed")
	device := DeviceMetrics{PaperWidth: 595, PaperHeight: 842, HeaderLineHeight: 11, FooterLineHeight: 11}
	job := Job{Settings: DefaultSettings()}
	if err := renderer.writePDF(failingPDFWriter{writeErr}, job, []Page{{Number: 1}}, device); !errors.Is(err, writeErr) {
		t.Fatalf("PDF write error = %v, want %v", err, writeErr)
	}
}

func TestPDFBandUsesNaturalFontVariants(t *testing.T) {
	renderer, err := newPDFRendererWithBandFonts(goregular.TTF, 12, 4, pdfBandFonts{bold: gobold.TTF, italic: goitalic.TTF, boldItalic: gobolditalic.TTF})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		font BandFont
	}{
		{"VivaPadBandBold", BandFont{Bold: true}},
		{"VivaPadBandItalic", BandFont{Italic: true}},
		{"VivaPadBandBoldItalic", BandFont{Bold: true, Italic: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pdf := newPDFDocument(renderer.fonts[:renderer.bodyCount], 12)
			pdf.SetCompression(false)
			pdf.AddPage()
			if err := renderer.drawPDFBand(pdf, TextBand{Right: "Styled band"}, tc.font, PageBounds{Left: 20, Top: 50, Right: 500, Bottom: 750}, true); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := pdf.Output(&output); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(output.Bytes(), []byte("/BaseFont /utf8"+strings.ToLower(tc.name))) || bytes.Contains(output.Bytes(), []byte("2 Tr")) || bytes.Contains(output.Bytes(), []byte(" cm\n")) {
				t.Fatal("the band should use a real styled face without synthetic stroke or skew")
			}
		})
	}
}

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
	pages[0].Styles = [][]ColorRun{{{StartByte: 0, EndByte: 5, Color: gfx.RGB(64, 102, 191)}}, nil}
	output, err := renderer.renderBytesForTest(job, pages, device)
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
	invertedPDF, err := renderer.renderBytesForTest(inverted, pages, device)
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
	if _, err := renderer.renderBytesForTest(Job{Text: "abc"}, []Page{{Number: 1, Rows: []Row{{StartByte: 0, EndByte: 3, Text: "wrong"}}}}, DeviceMetrics{PaperWidth: 595, PaperHeight: 842}); err == nil {
		t.Fatal("inconsistent page text accepted")
	}
	if _, err := renderer.renderBytesForTest(Job{Text: "🙂"}, []Page{{Number: 1, Rows: []Row{{StartByte: 0, EndByte: len("🙂"), Text: "🙂"}}}}, DeviceMetrics{PaperWidth: 595, PaperHeight: 842}); err == nil {
		t.Fatal("missing font glyph was silently omitted")
	}
}

func TestPDFRendererUsesCoveredFallbackForStyledMusicAndBand(t *testing.T) {
	primary, err := os.ReadFile("fonts/NotoSansSC.ttf")
	if err != nil {
		t.Fatal(err)
	}
	music, err := os.ReadFile("fonts/NotoMusic-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	const value = "alpha \U0001d11e \u4f60\u597d\nendPRINTJOB"
	job := Job{Title: "music.txt", Text: value, Settings: DefaultSettings()}
	job.Settings.Header = TextBand{Left: "Score \U0001d11e"}
	device := DeviceMetrics{PaperWidth: 595.28, PaperHeight: 841.89,
		PhysicalMargins: Insets{Left: 18, Top: 18, Right: 18, Bottom: 18}}

	old, err := newPDFRenderer(primary, 13, 4)
	if err != nil {
		t.Fatal(err)
	}
	device.HeaderLineHeight = old.BandHeight(job.Settings.HeaderFont)
	device.FooterLineHeight = old.BandHeight(job.Settings.FooterFont)
	pages, err := Paginate(job, device, old.Metrics(), PageVariables{})
	if err != nil || len(pages) != 1 {
		t.Fatalf("old pagination = %d pages, %v", len(pages), err)
	}
	if _, err := old.renderBytesForTest(job, pages, device); err == nil || !strings.Contains(err.Error(), "U+1D11E") {
		t.Fatalf("precondition: primary font unexpectedly printed the missing glyph: %v", err)
	}

	renderer, err := newPDFRendererWithFallback(primary, 13, 4, music)
	if err != nil {
		t.Fatalf("prepare licensed fallback: %v", err)
	}
	if renderer.Metrics().Advance("\U0001d11e") <= 0 ||
		renderer.Metrics().Advance("A\U0001d11e") <= renderer.Metrics().Advance("A") {
		t.Fatal("fallback glyph did not contribute to pagination width")
	}
	device.HeaderLineHeight = renderer.BandHeight(job.Settings.HeaderFont)
	device.FooterLineHeight = renderer.BandHeight(job.Settings.FooterFont)
	pages, err = Paginate(job, device, renderer.Metrics(), PageVariables{})
	if err != nil || len(pages) != 1 || len(pages[0].Rows) != 2 {
		t.Fatalf("fallback pagination = %+v, %v", pages, err)
	}
	start := strings.Index(value, "\U0001d11e")
	pages[0].Styles = make([][]ColorRun, len(pages[0].Rows))
	pages[0].Styles[0] = []ColorRun{{StartByte: start, EndByte: start + len("\U0001d11e"), Color: gfx.RGB(64, 102, 191)}}
	output, err := renderer.renderBytesForTest(job, pages, device)
	if err != nil || len(output) < 1000 || !bytes.HasPrefix(output, []byte("%PDF-")) {
		t.Fatalf("fallback PDF = %d bytes, %v", len(output), err)
	}
	if path := os.Getenv("VIVAPAD_MUSIC_PDF_PREVIEW_OUTPUT"); path != "" {
		if err := os.WriteFile(path, output, 0600); err != nil {
			t.Fatal(err)
		}
	}
	job.Text = "\U0001f642"
	if _, err := renderer.renderBytesForTest(job, []Page{{Number: 1, Bounds: pages[0].Bounds,
		TextBounds: pages[0].TextBounds, Rows: []Row{{StartByte: 0, EndByte: len(job.Text), Text: job.Text}}}}, device); err == nil || !strings.Contains(err.Error(), "U+1F642") {
		t.Fatalf("unsupported glyph was accepted or silently omitted: %v", err)
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
