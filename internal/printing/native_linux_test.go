//go:build linux

package printing

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLinuxPDFBandStylesCoverMixedText(t *testing.T) {
	renderer, err := newLinuxPDFRenderer(12, 4)
	if err != nil {
		t.Fatal(err)
	}
	text := "A\u4e2d\U0001d11eB"
	for _, tc := range []struct {
		name    string
		style   pdfFontStyle
		faces   []string
		missing []pdfFontStyle
	}{
		{"bold", pdfBold, []string{"VivaPadBandBold", "VivaPadFallback1", "VivaPadBandBold"}, []pdfFontStyle{0, pdfBold, 0}},
		{"italic", pdfItalic, []string{"VivaPadBandItalic", "VivaPad", "VivaPadFallback1", "VivaPadBandItalic"}, []pdfFontStyle{0, pdfItalic, pdfItalic, 0}},
		{"bold italic", pdfBold | pdfItalic, []string{"VivaPadBandBoldItalic", "VivaPadBandBold", "VivaPadFallback1", "VivaPadBandBoldItalic"}, []pdfFontStyle{0, pdfItalic, pdfBold | pdfItalic, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runs, err := renderer.fontRunsForStyle(text, tc.style)
			if err != nil {
				t.Fatal(err)
			}
			var faces []string
			var missing []pdfFontStyle
			var joined string
			for _, run := range runs {
				font := renderer.fonts[run.font]
				faces = append(faces, font.name)
				missing = append(missing, tc.style&^font.style)
				joined += run.text
			}
			if joined != text || !reflect.DeepEqual(faces, tc.faces) || !reflect.DeepEqual(missing, tc.missing) {
				t.Fatalf("text=%q faces=%v missing=%v", joined, faces, missing)
			}
		})
	}
	plain := newPDFDocument(renderer.fonts[:renderer.bodyCount], 12)
	plain.AddPage()
	if err := renderer.drawPDFBand(plain, TextBand{Left: text}, BandFont{}, PageBounds{Left: 20, Top: 50, Right: 500, Bottom: 750}, true); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := plain.Output(&output); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output.Bytes(), []byte("/BaseFont /utf8vivapadband")) {
		t.Fatal("unstyled PDF registered unused band fonts")
	}
}

func TestLinuxPortalPDFContainsOnlyRequestedPages(t *testing.T) {
	renderer, err := newPDFRenderer(linuxPrintFont, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	printer := &Printer{
		renderer: renderer,
		device:   DeviceMetrics{PaperWidth: 595, PaperHeight: 842, HeaderLineHeight: 11, FooterLineHeight: 11},
		choice:   PrintChoice{Ranges: []PageRange{{First: 2, Last: 2}, {First: 4, Last: 4}}, Copies: 2},
	}
	pages := []Page{{Number: 1}, {Number: 2}, {Number: 3}, {Number: 4}}
	var spool bytes.Buffer
	err = printer.writePDF(&spool, Job{Interactive: true, Settings: DefaultSettings()}, pages)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(spool.Bytes(), []byte("/Count 2\n")) {
		t.Fatal("the submitted PDF must contain exactly the two selected pages, before the portal applies copies")
	}
}

func TestLinuxPortalDoesNotRenderPDFForOutOfBoundsRange(t *testing.T) {
	renderer, err := newPDFRenderer(linuxPrintFont, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	printer := &Printer{
		renderer: renderer,
		device:   DeviceMetrics{PaperWidth: 595, PaperHeight: 842, HeaderLineHeight: 11, FooterLineHeight: 11},
		choice:   PrintChoice{Ranges: []PageRange{{First: 2, Last: 3}}},
	}
	pages := []Page{{Number: 1}, {Number: 2}}
	var spool bytes.Buffer
	err = printer.writePDF(&spool, Job{Interactive: true, Settings: DefaultSettings()}, pages)
	if err == nil || spool.Len() != 0 {
		t.Fatalf("portal built a partial PDF for an out-of-bounds range: %d bytes, err=%v", spool.Len(), err)
	}
}

func TestLinuxPortalPDFRendersDisjointRangesThroughLastPage(t *testing.T) {
	ranges, err := parsePortalRanges("3--1,1")
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := newPDFRenderer(linuxPrintFont, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	printer := &Printer{
		renderer: renderer,
		device:   DeviceMetrics{PaperWidth: 595, PaperHeight: 842, HeaderLineHeight: 11, FooterLineHeight: 11},
		choice:   PrintChoice{Ranges: ranges, Copies: 2},
	}
	pages := []Page{{Number: 1}, {Number: 2}, {Number: 3}, {Number: 4}, {Number: 5}}
	var spool bytes.Buffer
	err = printer.writePDF(&spool, Job{Interactive: true, Settings: DefaultSettings()}, pages)
	if err != nil || !bytes.Contains(spool.Bytes(), []byte("/Count 3\n")) {
		t.Fatalf("PDF must contain pages 2, 4 and 5 once before portal copies: bytes=%d, err=%v", spool.Len(), err)
	}
}

func TestClosedLinuxPrinterCannotSubmit(t *testing.T) {
	printer := &Printer{cups: true, choice: PrintChoice{Scope: ScopeAll}}
	printer.Close()
	if printer.cups {
		t.Fatal("closed CUPS printer still owns a submission path")
	}
	if err := printer.PrintPages(Job{Scope: ScopeAll}, []Page{{Number: 1}}); err == nil {
		t.Fatal("closed CUPS printer could submit another job")
	}
}

func TestLinuxPrintPagesPassesCompleteSpoolAndRemovesIt(t *testing.T) {
	tmp := t.TempDir()
	bin := t.TempDir()
	capture := filepath.Join(bin, "submitted.pdf")
	t.Setenv("TMPDIR", tmp)
	t.Setenv("PATH", bin)
	t.Setenv("VIVAPAD_TEST_PRINT_CAPTURE", capture)
	const fakeLP = `#!/bin/sh
set -eu
[ "$#" -eq 3 ]
[ "$1" = -t ]
[ "$2" = "VivaPad" ]
/bin/cp "$3" "$VIVAPAD_TEST_PRINT_CAPTURE"
`
	if err := os.WriteFile(filepath.Join(bin, "lp"), []byte(fakeLP), 0700); err != nil {
		t.Fatal(err)
	}
	renderer, err := newLinuxPDFRenderer(12, 4)
	if err != nil {
		t.Fatal(err)
	}
	printer := &Printer{
		cups: true, renderer: renderer,
		device: DeviceMetrics{PaperWidth: 595, PaperHeight: 842, HeaderLineHeight: 11, FooterLineHeight: 11},
	}
	t.Cleanup(printer.Close)
	job := Job{Settings: DefaultSettings()}
	job.Settings.HeaderFont = BandFont{Bold: true, Italic: true, Size: 14}
	job.Settings.FooterFont = BandFont{Italic: true}
	pages := []Page{{Number: 1, Header: TextBand{Right: "Report \u4e2d \U0001d11e"}, Footer: TextBand{Middle: "Footer"},
		Bounds: PageBounds{Left: 20, Top: 50, Right: 500, Bottom: 750}}, {Number: 2}}
	assertEmptySpool := func() {
		t.Helper()
		if files, err := os.ReadDir(tmp); err != nil || len(files) != 0 {
			t.Fatalf("temporary spool was not removed: %v, %v", files, err)
		}
	}
	// A rejected range must not reach the submission program.
	printer.choice.Ranges = []PageRange{{First: 3, Last: 3}}
	if err := printer.PrintPages(job, pages); err == nil {
		t.Fatal("invalid page range was accepted")
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatalf("invalid range reached lp: %v", err)
	}
	assertEmptySpool()

	printer.choice.Ranges = nil
	if err := printer.PrintPages(job, pages); err != nil {
		t.Fatal(err)
	}
	spooled, err := os.ReadFile(capture)
	if err != nil || !bytes.HasPrefix(spooled, []byte("%PDF-")) || !bytes.Contains(spooled, []byte("/Count 2\n")) ||
		!bytes.HasSuffix(bytes.TrimSpace(spooled), []byte("%%EOF")) {
		t.Fatalf("lp did not receive a complete two-page PDF: %d bytes, %v", len(spooled), err)
	}
	for _, resource := range []string{"vivapadbandbolditalic", "vivapadbandbold", "vivapadbanditalic"} {
		if !bytes.Contains(spooled, []byte("/BaseFont /utf8"+resource+"\n")) {
			t.Errorf("submitted PDF lacks %s", resource)
		}
	}
	assertEmptySpool()
}
