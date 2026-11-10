//go:build windows

package printing

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/ui"
)

const (
	pdSelection       = 0x00000001
	pdNoSelection     = 0x00000004
	pdPageNumbers     = 0x00000002
	pdReturnDC        = 0x00000100
	pdReturnDefault   = 0x00000400
	pdUseDeviceCopies = 0x00040000
	pdErrNoDefault    = 0x00001008
	physicalWidth     = 110
	physicalHeight    = 111
	physicalOffsetX   = 112
	physicalOffsetY   = 113
	horizontalPixels  = 8
	verticalPixels    = 10
	horizontalDPI     = 88
	verticalDPI       = 90
)

type printDialog struct {
	size             uint32
	owner            uintptr
	devMode          uintptr
	devNames         uintptr
	hdc              uintptr
	flags            uint32
	fromPage         uint16
	toPage           uint16
	minPage          uint16
	maxPage          uint16
	copies           uint16
	instance         uintptr
	customData       uintptr
	printHook        uintptr
	setupHook        uintptr
	printTemplate    *uint16
	setupTemplate    *uint16
	printTemplateMem uintptr
	setupTemplateMem uintptr
}

type textMetricW struct {
	height, ascent, descent, internalLeading, externalLeading int32
	averageWidth, maxWidth, weight, overhang                  int32
	aspectX, aspectY                                          int32
	firstChar, lastChar, defaultChar, breakChar               uint16
	italic, underlined, struckOut, pitchFamily, charset       uint8
}

type pixelSize struct{ width, height int32 }

type docInfoW struct {
	size     int32
	name     *uint16
	output   *uint16
	datatype *uint16
	kind     uint32
}

var (
	commonDialog = syscall.NewLazyDLL("comdlg32.dll")
	winPrintDlg  = commonDialog.NewProc("PrintDlgW")
	printError   = commonDialog.NewProc("CommDlgExtendedError")
	graphics     = syscall.NewLazyDLL("gdi32.dll")
	getCaps      = graphics.NewProc("GetDeviceCaps")
	createFont   = graphics.NewProc("CreateFontW")
	selectFont   = graphics.NewProc("SelectObject")
	deleteFont   = graphics.NewProc("DeleteObject")
	deleteDC     = graphics.NewProc("DeleteDC")
	getTextSize  = graphics.NewProc("GetTextExtentPoint32W")
	getTextInfo  = graphics.NewProc("GetTextMetricsW")
	startDoc     = graphics.NewProc("StartDocW")
	endDoc       = graphics.NewProc("EndDoc")
	abortDoc     = graphics.NewProc("AbortDoc")
	startPage    = graphics.NewProc("StartPage")
	endPage      = graphics.NewProc("EndPage")
	textOut      = graphics.NewProc("TextOutW")
	setTextColor = graphics.NewProc("SetTextColor")
	setBkMode    = graphics.NewProc("SetBkMode")
	setTextAlign = graphics.NewProc("SetTextAlign")
	moveTo       = graphics.NewProc("MoveToEx")
	lineTo       = graphics.NewProc("LineTo")
	patBlt       = graphics.NewProc("PatBlt")
	globalFree   = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalFree")
	activeWindow = syscall.NewLazyDLL("user32.dll").NewProc("GetActiveWindow")
)

type Printer struct {
	mu         sync.RWMutex
	hdc        uintptr
	devMode    uintptr
	devNames   uintptr
	font       uintptr
	headerFont uintptr
	footerFont uintptr
	oldFont    uintptr
	device     DeviceMetrics
	lineHeight float64
	tabWidth   int
	choice     PrintChoice
	physical   Insets
	dpiX, dpiY float64
}

func invalidGDIObject(handle uintptr) bool {
	return handle == 0 || handle == ^uintptr(0)
}

func failedColorChange(previous uintptr) bool {
	return uint32(previous) == 0xffffffff
}

func newPrintDialog(job Job) printDialog {
	dialog := printDialog{
		size:     uint32(unsafe.Sizeof(printDialog{})),
		flags:    pdUseDeviceCopies | pdReturnDC,
		fromPage: 1, toPage: 1, minPage: 1, maxPage: 0xffff, copies: 1,
	}
	if !job.Interactive {
		dialog.flags |= pdReturnDefault
	} else if job.Selection.End > job.Selection.Start {
		dialog.flags |= pdSelection
	} else {
		dialog.flags |= pdNoSelection
	}
	return dialog
}

func printDialogFailure(interactive bool, code uintptr) error {
	if code == pdErrNoDefault || code == 0 && !interactive {
		return ErrNoPrinter
	}
	if code == 0 {
		return ErrPrintCanceled
	}
	return fmt.Errorf("printing: Windows print dialog failed: 0x%04x", code)
}

func ChoosePrinter(job Job, fontSize float64, tabWidth int) (*Printer, error) {
	if math.IsNaN(fontSize) || math.IsInf(fontSize, 0) || fontSize <= 0 || fontSize > 256 || tabWidth < 1 || tabWidth > 16 {
		return nil, fmt.Errorf("printing: invalid native font or tab width")
	}
	dialog := newPrintDialog(job)
	if job.Interactive {
		dialog.owner, _, _ = activeWindow.Call()
	}
	accepted, _, _ := winPrintDlg.Call(uintptr(unsafe.Pointer(&dialog)))
	if accepted == 0 {
		freeDialogHandles(dialog)
		code, _, _ := printError.Call()
		return nil, printDialogFailure(job.Interactive, code)
	}
	if dialog.hdc == 0 {
		freeDialogHandles(dialog)
		return nil, fmt.Errorf("printing: Windows print dialog returned no printer DC")
	}
	printer := &Printer{hdc: dialog.hdc, devMode: dialog.devMode, devNames: dialog.devNames, tabWidth: tabWidth}
	if dialog.flags&pdPageNumbers != 0 {
		printer.choice.FirstPage, printer.choice.LastPage = int(dialog.fromPage), int(dialog.toPage)
	} else {
		printer.choice.FirstPage, printer.choice.LastPage = 1, maxPrintPages
	}
	printer.choice.Copies = max(1, int(dialog.copies))
	if job.Interactive && dialog.flags&pdSelection != 0 && job.Selection.End > job.Selection.Start {
		printer.choice.Scope = ScopeSelection
	}
	if err := printer.openMetrics(fontSize, job.Settings); err != nil {
		printer.Close()
		return nil, err
	}
	return printer, nil
}

func freeDialogHandles(dialog printDialog) {
	if dialog.hdc != 0 {
		deleteDC.Call(dialog.hdc)
	}
	if dialog.devMode != 0 {
		globalFree.Call(dialog.devMode)
	}
	if dialog.devNames != 0 {
		globalFree.Call(dialog.devNames)
	}
}

func (printer *Printer) openMetrics(fontSize float64, settings Settings) error {
	capability := func(index uintptr) float64 {
		value, _, _ := getCaps.Call(printer.hdc, index)
		return float64(int32(value))
	}
	printer.dpiX, printer.dpiY = capability(horizontalDPI), capability(verticalDPI)
	if printer.dpiX <= 0 || printer.dpiY <= 0 {
		return fmt.Errorf("printing: Windows printer has no resolution")
	}
	toX, toY := 72/printer.dpiX, 72/printer.dpiY
	paperWidth, paperHeight := capability(physicalWidth), capability(physicalHeight)
	left, top := capability(physicalOffsetX), capability(physicalOffsetY)
	printer.physical = Insets{Left: left, Top: top,
		Right:  paperWidth - capability(horizontalPixels) - left,
		Bottom: paperHeight - capability(verticalPixels) - top}
	printer.device = DeviceMetrics{
		PaperWidth: paperWidth * toX, PaperHeight: paperHeight * toY,
		PhysicalMargins: Insets{Left: printer.physical.Left * toX, Top: printer.physical.Top * toY,
			Right: printer.physical.Right * toX, Bottom: printer.physical.Bottom * toY},
	}
	var err error
	printer.font, err = printer.makeFont(fontSize, "Consolas", false, false)
	if err != nil {
		return err
	}
	printer.headerFont, err = printer.makeFont(float64(settings.HeaderFont.Points()), "Arial", settings.HeaderFont.Bold, settings.HeaderFont.Italic)
	if err != nil {
		return err
	}
	printer.footerFont, err = printer.makeFont(float64(settings.FooterFont.Points()), "Arial", settings.FooterFont.Bold, settings.FooterFont.Italic)
	if err != nil {
		return err
	}
	printer.oldFont, _, _ = selectFont.Call(printer.hdc, printer.font)
	if invalidGDIObject(printer.oldFont) {
		printer.oldFont = 0
		return fmt.Errorf("printing: select Windows printer font")
	}
	var body textMetricW
	if ok, _, _ := getTextInfo.Call(printer.hdc, uintptr(unsafe.Pointer(&body))); ok == 0 {
		return fmt.Errorf("printing: read Windows printer font metrics")
	}
	printer.lineHeight = float64(body.height+body.externalLeading) * toY
	if previous, _, _ := selectFont.Call(printer.hdc, printer.headerFont); invalidGDIObject(previous) {
		return fmt.Errorf("printing: select Windows print header font")
	}
	var header textMetricW
	if ok, _, _ := getTextInfo.Call(printer.hdc, uintptr(unsafe.Pointer(&header))); ok == 0 {
		return fmt.Errorf("printing: read Windows print header metrics")
	}
	if previous, _, _ := selectFont.Call(printer.hdc, printer.footerFont); invalidGDIObject(previous) {
		return fmt.Errorf("printing: select Windows print footer font")
	}
	var footer textMetricW
	if ok, _, _ := getTextInfo.Call(printer.hdc, uintptr(unsafe.Pointer(&footer))); ok == 0 {
		return fmt.Errorf("printing: read Windows print footer metrics")
	}
	if previous, _, _ := selectFont.Call(printer.hdc, printer.font); invalidGDIObject(previous) {
		return fmt.Errorf("printing: restore Windows printer font")
	}
	printer.device.HeaderLineHeight = float64(header.height+header.externalLeading) * toY
	printer.device.FooterLineHeight = float64(footer.height+footer.externalLeading) * toY
	_, err = BodyBounds(Job{}, printer.device)
	return err
}

func (printer *Printer) makeFont(size float64, family string, bold, italic bool) (uintptr, error) {
	face, _ := syscall.UTF16PtrFromString(family)
	height := -int32(math.Round(size * printer.dpiY / 72))
	weight, slant := uintptr(400), uintptr(0)
	if bold {
		weight = 700
	}
	if italic {
		slant = 1
	}
	font, _, _ := createFont.Call(uintptr(height), 0, 0, 0, weight, slant, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(face)))
	runtime.KeepAlive(face)
	if font == 0 {
		return 0, fmt.Errorf("printing: create Windows printer font")
	}
	return font, nil
}

func (printer *Printer) Metrics() (DeviceMetrics, TextMetrics) {
	if printer == nil {
		return DeviceMetrics{}, TextMetrics{}
	}
	printer.mu.RLock()
	device, height, tab := printer.device, printer.lineHeight, printer.tabWidth
	printer.mu.RUnlock()
	text := TextMetrics{LineHeight: height, TabWidth: tab}
	text.Advance = func(grapheme string) float64 {
		printer.mu.RLock()
		defer printer.mu.RUnlock()
		if printer.hdc == 0 {
			return math.NaN()
		}
		value, err := syscall.UTF16FromString(grapheme)
		if err != nil {
			return math.NaN()
		}
		var size pixelSize
		if ok, _, _ := getTextSize.Call(printer.hdc, uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)-1), uintptr(unsafe.Pointer(&size))); ok == 0 {
			return math.NaN()
		}
		runtime.KeepAlive(value)
		return float64(size.width) * 72 / printer.dpiX
	}
	return device, text
}

func (printer *Printer) Choice() PrintChoice {
	if printer == nil {
		return PrintChoice{}
	}
	return printer.choice
}

func (printer *Printer) toX(points float64) int32 {
	return int32(math.Round(points*printer.dpiX/72 - printer.physical.Left))
}

func (printer *Printer) toY(points float64) int32 {
	return int32(math.Round(points*printer.dpiY/72 - printer.physical.Top))
}

func printerColor(color ui.Color) uintptr {
	channel := func(value float32) uint32 {
		return uint32(math.Round(float64(min(max(value, 0), 1)) * 255))
	}
	return uintptr(channel(color.Red) | channel(color.Green)<<8 | channel(color.Blue)<<16)
}

func (printer *Printer) measureText(value string) (int32, error) {
	if value == "" {
		return 0, nil
	}
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		return 0, err
	}
	var size pixelSize
	if ok, _, _ := getTextSize.Call(printer.hdc, uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)-1), uintptr(unsafe.Pointer(&size))); ok == 0 {
		return 0, fmt.Errorf("printing: measure Windows printer text")
	}
	runtime.KeepAlive(encoded)
	return size.width, nil
}

func (printer *Printer) drawText(value string, x, y int32, color ui.Color) (int32, error) {
	if value == "" {
		return 0, nil
	}
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		return 0, err
	}
	if previous, _, _ := setTextColor.Call(printer.hdc, printerColor(color)); failedColorChange(previous) {
		return 0, fmt.Errorf("printing: set Windows printer text color")
	}
	if ok, _, _ := textOut.Call(printer.hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)-1)); ok == 0 {
		return 0, fmt.Errorf("printing: draw Windows printer text")
	}
	runtime.KeepAlive(encoded)
	return printer.measureText(value)
}

func (printer *Printer) drawBand(band TextBand, page Page, top bool) (err error) {
	if band == (TextBand{}) {
		return nil
	}
	font, lineHeight := printer.footerFont, printer.device.FooterLineHeight
	if top {
		font, lineHeight = printer.headerFont, printer.device.HeaderLineHeight
	}
	if previous, _, _ := selectFont.Call(printer.hdc, font); invalidGDIObject(previous) {
		return fmt.Errorf("printing: select Windows print band font")
	}
	defer func() {
		if previous, _, _ := selectFont.Call(printer.hdc, printer.font); invalidGDIObject(previous) && err == nil {
			err = fmt.Errorf("printing: restore Windows printer font")
		}
	}()
	y := page.Bounds.Bottom + lineHeight*0.5
	if top {
		y = page.Bounds.Top - lineHeight*1.5
	}
	left, right := printer.toX(page.Bounds.Left), printer.toX(page.Bounds.Right)
	black := ui.RGB(0, 0, 0)
	if _, err := printer.drawText(band.Left, left, printer.toY(y), black); err != nil {
		return err
	}
	for _, item := range []struct {
		value  string
		center bool
	}{{band.Middle, true}, {band.Right, false}} {
		width, err := printer.measureText(item.value)
		if err != nil {
			return err
		}
		x := right - width
		if item.center {
			x = left + (right-left-width)/2
		}
		if _, err := printer.drawText(item.value, x, printer.toY(y), black); err != nil {
			return err
		}
	}
	ruleY := y - lineHeight*0.25
	if top {
		ruleY = y + lineHeight*1.25
	}
	if ok, _, _ := moveTo.Call(printer.hdc, uintptr(left), uintptr(printer.toY(ruleY)), 0); ok == 0 {
		return fmt.Errorf("printing: move Windows page rule")
	}
	if ok, _, _ := lineTo.Call(printer.hdc, uintptr(right), uintptr(printer.toY(ruleY))); ok == 0 {
		return fmt.Errorf("printing: draw Windows page rule")
	}
	return nil
}

func (printer *Printer) drawPage(job Job, page Page, rows [][]Segment) error {
	if err := printer.drawBand(page.Header, page, true); err != nil {
		return err
	}
	if err := printer.drawBand(page.Footer, page, false); err != nil {
		return err
	}
	for index, row := range page.Rows {
		y := printer.toY(page.Bounds.Top + float64(index)*printer.lineHeight)
		if printBodyBackground(job.Settings) {
			left := printer.toX(page.TextBounds.Left)
			width := printer.toX(page.TextBounds.Right) - left
			height := int32(math.Ceil(printer.lineHeight * printer.dpiY / 72))
			if ok, _, _ := patBlt.Call(printer.hdc, uintptr(left), uintptr(y), uintptr(width), uintptr(height), 0x00000042); ok == 0 {
				return fmt.Errorf("printing: fill Windows inverted print row")
			}
		}
		if row.LineNumber > 0 && job.Settings.LineNumbers {
			if _, err := printer.drawText(strconv.Itoa(row.LineNumber), printer.toX(page.Bounds.Left), y, ui.RGB(112, 116, 123)); err != nil {
				return err
			}
		}
		x := printer.toX(page.TextBounds.Left)
		for _, segment := range rows[index] {
			width, err := printer.drawText(segment.Text, x, y, segment.Color)
			if err != nil {
				return err
			}
			x += width
		}
	}
	return nil
}

func (printer *Printer) PrintPages(job Job, pages []Page) error {
	if printer == nil {
		return fmt.Errorf("printing: native printer is missing")
	}
	if job.Scope != printer.choice.Scope {
		return fmt.Errorf("printing: page scope differs from the print dialog selection")
	}
	selected, err := SelectPages(pages, printer.choice.FirstPage, printer.choice.LastPage)
	if err != nil {
		return err
	}
	prepared := make([][][]Segment, len(selected))
	for pageIndex, page := range selected {
		if len(page.Styles) != 0 && len(page.Styles) != len(page.Rows) {
			return fmt.Errorf("printing: page styles do not match its rows")
		}
		prepared[pageIndex] = make([][]Segment, len(page.Rows))
		for rowIndex, row := range page.Rows {
			var styles []ColorRun
			if len(page.Styles) > 0 {
				styles = page.Styles[rowIndex]
			}
			prepared[pageIndex][rowIndex], err = Segments(job, row, styles, printer.tabWidth)
			if err != nil {
				return err
			}
		}
	}
	name := job.Title
	if name == "" {
		name = "VivaPad"
	}
	label, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	printer.mu.Lock()
	defer printer.mu.Unlock()
	if printer.hdc == 0 {
		return fmt.Errorf("printing: native printer is closed")
	}
	if previous, _, _ := setBkMode.Call(printer.hdc, 1); previous == 0 {
		return fmt.Errorf("printing: set Windows printer background mode")
	}
	if previous, _, _ := setTextAlign.Call(printer.hdc, 0); uint32(previous) == 0xffffffff {
		return fmt.Errorf("printing: set Windows printer text alignment")
	}
	document := docInfoW{size: int32(unsafe.Sizeof(docInfoW{})), name: label}
	started, _, _ := startDoc.Call(printer.hdc, uintptr(unsafe.Pointer(&document)))
	runtime.KeepAlive(label)
	if int32(started) <= 0 {
		return fmt.Errorf("printing: Windows StartDoc rejected the job")
	}
	finished := false
	defer func() {
		if !finished {
			abortDoc.Call(printer.hdc)
		}
	}()
	for index, page := range selected {
		if started, _, _ := startPage.Call(printer.hdc); int32(started) <= 0 {
			return fmt.Errorf("printing: Windows StartPage rejected page %d", page.Number)
		}
		if err := printer.drawPage(job, page, prepared[index]); err != nil {
			return err
		}
		if ended, _, _ := endPage.Call(printer.hdc); int32(ended) <= 0 {
			return fmt.Errorf("printing: Windows EndPage failed for page %d", page.Number)
		}
	}
	if ended, _, _ := endDoc.Call(printer.hdc); int32(ended) <= 0 {
		return fmt.Errorf("printing: Windows EndDoc failed")
	}
	finished = true
	return nil
}

func (printer *Printer) Close() {
	if printer == nil {
		return
	}
	printer.mu.Lock()
	defer printer.mu.Unlock()
	if printer.hdc != 0 {
		if printer.oldFont != 0 {
			selectFont.Call(printer.hdc, printer.oldFont)
		}
		if printer.font != 0 {
			deleteFont.Call(printer.font)
		}
		if printer.headerFont != 0 {
			deleteFont.Call(printer.headerFont)
		}
		if printer.footerFont != 0 {
			deleteFont.Call(printer.footerFont)
		}
		deleteDC.Call(printer.hdc)
		printer.hdc = 0
	}
	if printer.devMode != 0 {
		globalFree.Call(printer.devMode)
		printer.devMode = 0
	}
	if printer.devNames != 0 {
		globalFree.Call(printer.devNames)
		printer.devNames = 0
	}
}
