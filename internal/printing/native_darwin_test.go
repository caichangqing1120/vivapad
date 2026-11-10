//go:build darwin

package printing

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
	"vivapad/internal/syntax"
)

func TestNativePrintWireHonorsLineNumbersAndInvertedBody(t *testing.T) {
	job := Job{Text: "abc", Settings: Settings{ColorMode: ColorInvertLight, LineNumbers: false,
		HeaderFont: BandFont{Size: 12, Bold: true}, FooterFont: BandFont{Size: 8, Italic: true}}}
	page := Page{Number: 1, Rows: []Row{{Text: "abc", StartByte: 0, EndByte: 3, LineNumber: 1}}}
	encoded, err := marshalNativePages(job, []Page{page}, testDevice(), testTextMetrics())
	if err != nil {
		t.Fatal(err)
	}
	var document printWireDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Pages) != 1 || document.Pages[0].Rows[0].LineNumber != 0 || !document.Pages[0].InvertBody ||
		document.Pages[0].Header.Size != 12 || !document.Pages[0].Header.Bold || document.Pages[0].Footer.Size != 8 || !document.Pages[0].Footer.Italic {
		t.Fatalf("native print wire = %+v", document.Pages)
	}
}

func TestNativePrintChoiceReadsMetricsWithoutSubmittingPages(t *testing.T) {
	printer, err := ChoosePrinter(Job{Title: "VivaPad test", Interactive: false}, 13, 4)
	if os.Getenv("VIVAPAD_REQUIRE_NO_DEFAULT_PRINTER") == "1" && !errors.Is(err, ErrNoPrinter) {
		if printer != nil {
			printer.Close()
		}
		t.Fatalf("Print Now picked a printer without a system default: %v", err)
	}
	if errors.Is(err, ErrNoPrinter) {
		t.Log("no default printer; no print job submitted")
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer printer.Close()
	device, text := printer.Metrics()
	if device.PaperWidth <= 0 || device.PaperHeight <= 0 || text.LineHeight <= 0 || text.TabWidth != 4 ||
		text.Advance == nil || math.IsNaN(text.Advance("猫")) || text.Advance("猫") <= 0 {
		t.Fatalf("selected native metrics = %+v, %+v", device, text)
	}
	if _, err := BodyBounds(Job{}, device); err != nil {
		t.Fatalf("native page has no body: %v", err)
	}
	choice := printer.Choice()
	if choice.Scope != ScopeAll || choice.FirstPage < 1 || choice.LastPage < choice.FirstPage || choice.Copies < 1 {
		t.Fatalf("Print Now choices = %+v", choice)
	}
	printer.Close()
	if !math.IsNaN(text.Advance("x")) {
		t.Fatal("closed native font was still measured")
	}
}

func TestNativePrintChoiceRejectsInvalidGeometryBeforeDialog(t *testing.T) {
	for _, size := range []float64{0, -1, math.NaN()} {
		if _, err := ChoosePrinter(Job{Interactive: true}, size, 4); err == nil {
			t.Fatalf("invalid font size %v reached the print panel", size)
		}
	}
	if _, err := ChoosePrinter(Job{Interactive: true}, 13, 0); err == nil {
		t.Fatal("invalid tab width reached the print panel")
	}
}

func TestNativePrintMetricsWithProcessScopedDestination(t *testing.T) {
	name := os.Getenv("VIVAPAD_TEST_PRINT_METRICS")
	if name == "" {
		t.Skip("set VIVAPAD_TEST_PRINT_METRICS to inspect an installed printer without spooling")
	}
	t.Setenv("LPDEST", name)
	printer, err := ChoosePrinter(Job{Interactive: false}, 13, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer printer.Close()
	device, text := printer.Metrics()
	if device.PaperWidth <= 0 || device.PaperHeight <= 0 || text.LineHeight <= 0 || text.Advance("猫") <= 0 {
		t.Fatalf("native font or paper metrics = %+v, %+v", device, text)
	}
	if bounds, err := BodyBounds(Job{}, device); err != nil || bounds.Left >= bounds.Right || bounds.Top >= bounds.Bottom {
		t.Fatalf("native imageable page = %+v, %v", bounds, err)
	}
	if choice := printer.Choice(); choice.Scope != ScopeAll || choice.Copies < 1 || choice.FirstPage < 1 {
		t.Fatalf("Print Now choice = %+v", choice)
	}
	printer.Close()
	if !math.IsNaN(text.Advance("x")) {
		t.Fatal("closed print context was still usable")
	}
}

func TestNativePDFPreviewRendersStyledPageWithoutSpooling(t *testing.T) {
	name := os.Getenv("VIVAPAD_TEST_PRINT_METRICS")
	if name == "" {
		t.Skip("set VIVAPAD_TEST_PRINT_METRICS for a process-scoped PDF-only preview")
	}
	t.Setenv("LPDEST", name)
	doc := document.New("print-preview.go")
	doc.Editor.Insert("func main() {\n\tprintln(\"猫\")\n}\n")
	job, err := Prepare(doc, ".go", false)
	if err != nil {
		t.Fatal(err)
	}
	printer, err := ChoosePrinter(job, 13, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer printer.Close()
	device, text := printer.Metrics()
	pages, err := Paginate(job, device, text, PageVariables{})
	if err != nil || len(pages) != 1 {
		t.Fatalf("native pages = %d, %v", len(pages), err)
	}
	var highlighter syntax.Highlighter
	pages, err = Colorize(job, pages, highlighter.Highlight)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := printer.PreviewPDF(job, pages)
	if err != nil || len(pdf) < 100 || !strings.HasPrefix(string(pdf[:min(len(pdf), 5)]), "%PDF-") {
		t.Fatalf("native PDF preview = %d bytes, %v", len(pdf), err)
	}
	if output := os.Getenv("VIVAPAD_PRINT_PREVIEW_OUTPUT"); output != "" {
		if err := os.WriteFile(output, pdf, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeBandFontsMeasureAndPreviewWithoutSpooling(t *testing.T) {
	name := os.Getenv("VIVAPAD_TEST_PRINT_METRICS")
	if name == "" {
		t.Skip("set VIVAPAD_TEST_PRINT_METRICS for a process-scoped printer metrics probe")
	}
	t.Setenv("LPDEST", name)
	baseline, err := ChoosePrinter(Job{Interactive: false}, 13, 4)
	if err != nil {
		t.Fatal(err)
	}
	defaultDevice, _ := baseline.Metrics()
	baseline.Close()
	job := Job{Title: "Font preview", Text: "alpha\nbeta", Settings: DefaultSettings()}
	job.Settings.Header = TextBand{Left: "Header alpha"}
	job.Settings.Footer = TextBand{Right: "Page 1"}
	job.Settings.HeaderFont = BandFont{Size: 14, Bold: true, Italic: true}
	job.Settings.FooterFont = BandFont{Size: 6, Italic: true}
	printer, err := ChoosePrinter(job, 13, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer printer.Close()
	device, text := printer.Metrics()
	if device.HeaderLineHeight <= defaultDevice.HeaderLineHeight || device.FooterLineHeight >= defaultDevice.FooterLineHeight {
		t.Fatalf("native band metrics = header %.1f vs %.1f, footer %.1f vs %.1f", device.HeaderLineHeight, defaultDevice.HeaderLineHeight, device.FooterLineHeight, defaultDevice.FooterLineHeight)
	}
	pages, err := Paginate(job, device, text, PageVariables{})
	if err != nil || len(pages) != 1 {
		t.Fatalf("font preview pagination = %d, %v", len(pages), err)
	}
	pdf, err := printer.PreviewPDF(job, pages)
	if err != nil || len(pdf) < 100 || !strings.HasPrefix(string(pdf[:min(len(pdf), 5)]), "%PDF-") {
		t.Fatalf("native font preview = %d bytes, %v", len(pdf), err)
	}
	if output := os.Getenv("VIVAPAD_PRINT_BAND_PREVIEW_OUTPUT"); output != "" {
		if err := os.WriteFile(output, pdf, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeSystemPrintOperationSavesToTaskPDFWithoutSpooling(t *testing.T) {
	if os.Getenv("VIVAPAD_TEST_PRINT_SAVE_JOB") != "1" {
		t.Skip("explicit opt-in required before running the native Save PDF print operation")
	}
	name := os.Getenv("VIVAPAD_TEST_PRINT_METRICS")
	if name == "" {
		t.Skip("set VIVAPAD_TEST_PRINT_METRICS to test a process-scoped PDF print destination")
	}
	t.Setenv("LPDEST", name)
	doc := document.New("system-print.go")
	doc.Editor.Insert("func main() {\n\tprintln(\"猫\")\n}\n")
	job, err := Prepare(doc, ".go", false)
	if err != nil {
		t.Fatal(err)
	}
	printer, err := ChoosePrinter(job, 13, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer printer.Close()
	device, text := printer.Metrics()
	pages, err := Paginate(job, device, text, PageVariables{})
	if err != nil || len(pages) != 1 {
		t.Fatalf("print pages = %d, %v", len(pages), err)
	}
	var highlighter syntax.Highlighter
	pages, err = Colorize(job, pages, highlighter.Highlight)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "system-print.pdf")
	if err := printer.PrintToPDF(job, pages, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 100 || !strings.HasPrefix(string(data[:min(len(data), 5)]), "%PDF-") {
		t.Fatalf("native system print output = %d bytes, %v", len(data), err)
	}
	if output := os.Getenv("VIVAPAD_PRINT_SYSTEM_OUTPUT"); output != "" {
		if err := os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativePDFDestinationRefusesOverwriteBeforeSubmission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.pdf")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	var printer *Printer
	if err := printer.PrintToPDF(Job{}, nil, path); err == nil {
		t.Fatal("existing PDF destination was accepted")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("existing PDF was changed: %q, %v", data, err)
	}
	if err := printer.PrintPages(Job{}, nil); err == nil {
		t.Fatal("nil native printer accepted a physical print")
	}
}
