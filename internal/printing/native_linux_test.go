//go:build linux

package printing

import (
	"bytes"
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
	data, err := printer.renderPDF(Job{Interactive: true, Settings: DefaultSettings()}, pages)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("/Count 2\n")) {
		t.Fatal("the submitted PDF must contain exactly the two selected pages, before the portal applies copies")
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
