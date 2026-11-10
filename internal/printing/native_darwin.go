//go:build darwin

package printing

/*
#cgo LDFLAGS: -framework AppKit -lcups
#include <stdlib.h>
#include <stddef.h>

typedef struct vivapad_print_context vivapad_print_context;
typedef struct {
    double paper_width, paper_height;
    double margin_left, margin_top, margin_right, margin_bottom;
    double header_line_height, footer_line_height, body_line_height;
    int selection_only, first_page, last_page, copies;
} vivapad_print_metrics;

int vivapad_print_choose(int interactive, int has_selection, double font_size,
    int header_size, int header_bold, int header_italic,
    int footer_size, int footer_bold, int footer_italic,
    vivapad_print_context **context, vivapad_print_metrics *metrics, char **message);
double vivapad_print_advance(vivapad_print_context *context, const char *utf8, size_t length);
int vivapad_print_preview(vivapad_print_context *context, const char *json, size_t length,
    unsigned char **pdf, size_t *pdf_length, char **message);
int vivapad_print_submit(vivapad_print_context *context, const char *json, size_t length,
    const char *save_path, char **message);
void vivapad_print_close(vivapad_print_context *context);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

type Printer struct {
	mu           sync.RWMutex
	native       *C.vivapad_print_context
	metrics      C.vivapad_print_metrics
	tabWidth     int
	interactive  bool
	hasSelection bool
}

func ChoosePrinter(job Job, fontSize float64, tabWidth int) (*Printer, error) {
	if math.IsNaN(fontSize) || math.IsInf(fontSize, 0) || fontSize <= 0 || fontSize > 256 || tabWidth < 1 || tabWidth > 16 {
		return nil, fmt.Errorf("printing: invalid native font or tab width")
	}
	interactive, hasSelection := C.int(0), C.int(0)
	if job.Interactive {
		interactive = 1
		if job.Selection.End > job.Selection.Start {
			hasSelection = 1
		}
	}
	var native *C.vivapad_print_context
	var metrics C.vivapad_print_metrics
	var message *C.char
	flag := func(enabled bool) C.int {
		if enabled {
			return 1
		}
		return 0
	}
	switch C.vivapad_print_choose(interactive, hasSelection, C.double(fontSize),
		C.int(job.Settings.HeaderFont.Points()), flag(job.Settings.HeaderFont.Bold), flag(job.Settings.HeaderFont.Italic),
		C.int(job.Settings.FooterFont.Points()), flag(job.Settings.FooterFont.Bold), flag(job.Settings.FooterFont.Italic),
		&native, &metrics, &message) {
	case 0:
		return &Printer{native: native, metrics: metrics, tabWidth: tabWidth, interactive: job.Interactive, hasSelection: hasSelection != 0}, nil
	case 1:
		return nil, ErrPrintCanceled
	case 2:
		return nil, ErrNoPrinter
	default:
		if message == nil {
			return nil, fmt.Errorf("printing: native printer selection failed")
		}
		defer C.free(unsafe.Pointer(message))
		return nil, fmt.Errorf("printing: native printer selection: %s", C.GoString(message))
	}
}

func (printer *Printer) Metrics() (DeviceMetrics, TextMetrics) {
	if printer == nil {
		return DeviceMetrics{}, TextMetrics{}
	}
	printer.mu.RLock()
	metrics, tabWidth := printer.metrics, printer.tabWidth
	printer.mu.RUnlock()
	device := DeviceMetrics{
		PaperWidth: float64(metrics.paper_width), PaperHeight: float64(metrics.paper_height),
		PhysicalMargins: Insets{
			Left: float64(metrics.margin_left), Top: float64(metrics.margin_top),
			Right: float64(metrics.margin_right), Bottom: float64(metrics.margin_bottom),
		},
		HeaderLineHeight: float64(metrics.header_line_height), FooterLineHeight: float64(metrics.footer_line_height),
	}
	text := TextMetrics{LineHeight: float64(metrics.body_line_height), TabWidth: tabWidth}
	text.Advance = func(cluster string) float64 {
		printer.mu.RLock()
		defer printer.mu.RUnlock()
		if printer.native == nil {
			return math.NaN()
		}
		bytes := []byte(cluster)
		if len(bytes) == 0 {
			return 0
		}
		advance := float64(C.vivapad_print_advance(printer.native, (*C.char)(unsafe.Pointer(&bytes[0])), C.size_t(len(bytes))))
		runtime.KeepAlive(bytes)
		return advance
	}
	return device, text
}

func (printer *Printer) Choice() PrintChoice {
	if printer == nil {
		return PrintChoice{}
	}
	printer.mu.RLock()
	defer printer.mu.RUnlock()
	choice := PrintChoice{
		FirstPage: int(printer.metrics.first_page), LastPage: int(printer.metrics.last_page),
		Copies: int(printer.metrics.copies),
	}
	if printer.interactive && printer.hasSelection && printer.metrics.selection_only != 0 {
		choice.Scope = ScopeSelection
	}
	return choice
}

type printWireRect struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
}

type printWireBand struct {
	Left   string `json:"left"`
	Middle string `json:"middle"`
	Right  string `json:"right"`
	Size   int    `json:"size"`
	Bold   bool   `json:"bold"`
	Italic bool   `json:"italic"`
}

type printWireSegment struct {
	Text  string  `json:"text"`
	Red   float32 `json:"red"`
	Green float32 `json:"green"`
	Blue  float32 `json:"blue"`
}

type printWireRow struct {
	LineNumber int                `json:"lineNumber"`
	Segments   []printWireSegment `json:"segments"`
}

type printWirePage struct {
	Number     int            `json:"number"`
	InvertBody bool           `json:"invertBody"`
	Bounds     printWireRect  `json:"bounds"`
	TextBounds printWireRect  `json:"textBounds"`
	Header     printWireBand  `json:"header"`
	Footer     printWireBand  `json:"footer"`
	Rows       []printWireRow `json:"rows"`
}

type printWireDocument struct {
	PaperWidth  float64         `json:"paperWidth"`
	PaperHeight float64         `json:"paperHeight"`
	LineHeight  float64         `json:"lineHeight"`
	Pages       []printWirePage `json:"pages"`
}

func marshalNativePages(job Job, pages []Page, device DeviceMetrics, text TextMetrics) ([]byte, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("printing: no pages to render")
	}
	wireRect := func(bounds PageBounds) printWireRect {
		return printWireRect{Left: bounds.Left, Top: bounds.Top, Right: bounds.Right, Bottom: bounds.Bottom}
	}
	wireBand := func(band TextBand, font BandFont) printWireBand {
		return printWireBand{Left: band.Left, Middle: band.Middle, Right: band.Right,
			Size: font.Points(), Bold: font.Bold, Italic: font.Italic}
	}
	wire := printWireDocument{PaperWidth: device.PaperWidth, PaperHeight: device.PaperHeight, LineHeight: text.LineHeight}
	for _, page := range pages {
		if len(page.Styles) != 0 && len(page.Styles) != len(page.Rows) {
			return nil, fmt.Errorf("printing: page styles do not match its rows")
		}
		item := printWirePage{
			Number: page.Number, InvertBody: printBodyBackground(job.Settings), Bounds: wireRect(page.Bounds), TextBounds: wireRect(page.TextBounds),
			Header: wireBand(page.Header, job.Settings.HeaderFont), Footer: wireBand(page.Footer, job.Settings.FooterFont),
		}
		for index, row := range page.Rows {
			var styles []ColorRun
			if len(page.Styles) > 0 {
				styles = page.Styles[index]
			}
			segments, err := Segments(job, row, styles, text.TabWidth)
			if err != nil {
				return nil, err
			}
			printed := printWireRow{}
			if job.Settings.LineNumbers {
				printed.LineNumber = row.LineNumber
			}
			for _, segment := range segments {
				printed.Segments = append(printed.Segments, printWireSegment{
					Text: segment.Text, Red: segment.Color.Red, Green: segment.Color.Green, Blue: segment.Color.Blue,
				})
			}
			item.Rows = append(item.Rows, printed)
		}
		wire.Pages = append(wire.Pages, item)
	}
	return json.Marshal(wire)
}

func (printer *Printer) PreviewPDF(job Job, pages []Page) ([]byte, error) {
	if printer == nil {
		return nil, fmt.Errorf("printing: native print context is missing")
	}
	device, text := printer.Metrics()
	encoded, err := marshalNativePages(job, pages, device, text)
	if err != nil {
		return nil, err
	}
	printer.mu.RLock()
	defer printer.mu.RUnlock()
	if printer.native == nil {
		return nil, fmt.Errorf("printing: native print context is closed")
	}
	var output *C.uchar
	var length C.size_t
	var message *C.char
	bytes := C.CBytes(encoded)
	defer C.free(bytes)
	status := C.vivapad_print_preview(printer.native, (*C.char)(bytes), C.size_t(len(encoded)), &output, &length, &message)
	if status != 0 {
		if message == nil {
			return nil, fmt.Errorf("printing: native PDF preview failed")
		}
		defer C.free(unsafe.Pointer(message))
		return nil, fmt.Errorf("printing: native PDF preview: %s", C.GoString(message))
	}
	defer C.free(unsafe.Pointer(output))
	if output == nil || length > 64<<20 || length < 5 {
		return nil, fmt.Errorf("printing: native PDF preview size is invalid")
	}
	return C.GoBytes(unsafe.Pointer(output), C.int(length)), nil
}

// PrintPages submits formatted pages to the printer chosen by ChoosePrinter.
func (printer *Printer) PrintPages(job Job, pages []Page) error {
	return printer.submit(job, pages, "")
}

func (printer *Printer) PrintToPDF(job Job, pages []Page, path string) error {
	if !filepath.IsAbs(path) || strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("printing: PDF destination must be an absolute path")
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("printing: PDF destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parent.IsDir() {
		return fmt.Errorf("printing: PDF destination directory is unavailable: %v", err)
	}
	return printer.submit(job, pages, path)
}

func (printer *Printer) submit(job Job, pages []Page, path string) error {
	if printer == nil {
		return fmt.Errorf("printing: native print context is missing")
	}
	if job.Scope != printer.Choice().Scope {
		return fmt.Errorf("printing: page scope differs from the print dialog selection")
	}
	device, text := printer.Metrics()
	encoded, err := marshalNativePages(job, pages, device, text)
	if err != nil {
		return err
	}
	printer.mu.RLock()
	defer printer.mu.RUnlock()
	if printer.native == nil {
		return fmt.Errorf("printing: native print context is closed")
	}
	bytes := C.CBytes(encoded)
	defer C.free(bytes)
	var destination *C.char
	if path != "" {
		destination = C.CString(path)
		defer C.free(unsafe.Pointer(destination))
	}
	var message *C.char
	if C.vivapad_print_submit(printer.native, (*C.char)(bytes), C.size_t(len(encoded)), destination, &message) != 0 {
		if message == nil {
			return fmt.Errorf("printing: native print operation failed")
		}
		defer C.free(unsafe.Pointer(message))
		return fmt.Errorf("printing: native print operation: %s", C.GoString(message))
	}
	return nil
}

func (printer *Printer) Close() {
	if printer == nil {
		return
	}
	printer.mu.Lock()
	defer printer.mu.Unlock()
	if printer.native != nil {
		C.vivapad_print_close(printer.native)
		printer.native = nil
	}
}
