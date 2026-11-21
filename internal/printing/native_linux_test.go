//go:build linux

package printing

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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
	renderer, err := newPDFRenderer(linuxPrintFont, 12, 4)
	if err != nil {
		t.Fatal(err)
	}
	printer := &Printer{
		cups: true, renderer: renderer,
		device: DeviceMetrics{PaperWidth: 595, PaperHeight: 842, HeaderLineHeight: 11, FooterLineHeight: 11},
	}
	t.Cleanup(printer.Close)
	job := Job{Settings: DefaultSettings()}
	pages := []Page{{Number: 1}, {Number: 2}}
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
	assertEmptySpool()
}
