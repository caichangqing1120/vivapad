//go:build windows

package printing

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

var _ [120 - unsafe.Sizeof(printDialog{})]byte
var _ [unsafe.Sizeof(printDialog{}) - 120]byte
var _ [60 - unsafe.Sizeof(textMetricW{})]byte
var _ [unsafe.Sizeof(textMetricW{}) - 60]byte
var _ [40 - unsafe.Sizeof(docInfoW{})]byte
var _ [unsafe.Sizeof(docInfoW{}) - 40]byte

func TestWindowsPrintDialogFlagsFollowSourceSelectionAndDefault(t *testing.T) {
	selected := Job{Interactive: true, Scope: ScopeSelection, Selection: text.Span{Start: 1, End: 4}}
	dialog := newPrintDialog(selected)
	if dialog.flags&pdSelection == 0 || dialog.flags&pdReturnDC == 0 || dialog.flags&pdReturnDefault != 0 ||
		dialog.fromPage != 1 || dialog.maxPage != 0xffff || dialog.copies != 1 {
		t.Fatalf("selected print dialog = %+v", dialog)
	}
	now := newPrintDialog(Job{Interactive: false, Scope: ScopeAll, Selection: text.Span{Start: 1, End: 4}})
	if now.flags&pdReturnDefault == 0 || now.flags&pdReturnDC == 0 || now.flags&pdSelection != 0 {
		t.Fatalf("Print Now would open a dialog or use selection: %+v", now)
	}
}

func TestWindowsPrinterCoordinatesAndColorFollowGDI(t *testing.T) {
	printer := &Printer{dpiX: 300, dpiY: 300, physical: Insets{Left: 75, Top: 50}}
	if printer.toX(18) != 0 || printer.toY(12) != 0 || printer.toX(90) != 300 {
		t.Fatalf("printer page origin = %d,%d; x(90)=%d", printer.toX(18), printer.toY(12), printer.toX(90))
	}
	if got := printerColor(gfx.RGB(49, 92, 167)); got != 0xa75c31 {
		t.Fatalf("Windows COLORREF = 0x%06x", got)
	}
}

func TestWindowsGDIErrorSentinels(t *testing.T) {
	if !invalidGDIObject(0) || !invalidGDIObject(^uintptr(0)) || invalidGDIObject(1) {
		t.Fatal("SelectObject must reject NULL and HGDI_ERROR handles")
	}
	if !failedColorChange(0xffffffff) || failedColorChange(0x00ffffff) || failedColorChange(0) {
		t.Fatal("SetTextColor must compare the 32-bit CLR_INVALID value")
	}
}

func TestWindowsPrintDialogDistinguishesCancelAndMissingPrinter(t *testing.T) {
	if !errors.Is(printDialogFailure(true, 0), ErrPrintCanceled) {
		t.Fatal("interactive dialog cancel must not print")
	}
	if !errors.Is(printDialogFailure(false, 0), ErrNoPrinter) ||
		!errors.Is(printDialogFailure(false, 0x1008), ErrNoPrinter) ||
		!errors.Is(printDialogFailure(true, 0x1008), ErrNoPrinter) {
		t.Fatal("missing default printer must not enqueue a print job")
	}
	if err := printDialogFailure(true, 0x1002); err == nil || errors.Is(err, ErrPrintCanceled) || errors.Is(err, ErrNoPrinter) {
		t.Fatalf("unexpected dialog failure: %v", err)
	}
}
