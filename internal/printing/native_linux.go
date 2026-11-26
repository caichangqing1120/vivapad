//go:build linux

package printing

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

//go:embed fonts/NotoSansSC.ttf
var linuxPrintFont []byte

//go:embed fonts/NotoMusic-Regular.ttf
var linuxMusicFont []byte

//go:embed fonts/NotoSansSC-Bold.ttf
var linuxPrintBold []byte

//go:embed fonts/NotoSans-Italic.ttf
var linuxPrintItalic []byte

//go:embed fonts/NotoSans-BoldItalic.ttf
var linuxPrintBoldItalic []byte

const portalName = "org.freedesktop.portal.Desktop"
const portalPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")

type Printer struct {
	mu       sync.Mutex
	bus      *dbus.Conn
	renderer *pdfRenderer
	device   DeviceMetrics
	choice   PrintChoice
	token    uint32
	cups     bool
}

func newLinuxPDFRenderer(fontSize float64, tabWidth int) (*pdfRenderer, error) {
	return newPDFRendererWithBandFonts(linuxPrintFont, fontSize, tabWidth, pdfBandFonts{
		bold: linuxPrintBold, italic: linuxPrintItalic, boldItalic: linuxPrintBoldItalic,
	}, linuxMusicFont)
}

func ChoosePrinter(job Job, fontSize float64, tabWidth int) (*Printer, error) {
	renderer, err := newLinuxPDFRenderer(fontSize, tabWidth)
	if err != nil {
		return nil, err
	}
	if !job.Interactive {
		device, err := chooseCUPSDefault()
		if err != nil {
			return nil, err
		}
		device.HeaderLineHeight = renderer.BandHeight(job.Settings.HeaderFont)
		device.FooterLineHeight = renderer.BandHeight(job.Settings.FooterFont)
		if _, err := BodyBounds(job, device); err != nil {
			return nil, err
		}
		return &Printer{renderer: renderer, device: device, choice: PrintChoice{FirstPage: 1, LastPage: maxPrintPages, Copies: 1}, cups: true}, nil
	}
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("%w: Linux print portal: %v", ErrUnsupported, err)
	}
	if !bus.SupportsUnixFDs() {
		bus.Close()
		return nil, fmt.Errorf("%w: print portal cannot receive PDF files", ErrUnsupported)
	}
	printer := &Printer{bus: bus, renderer: renderer}
	settings := map[string]dbus.Variant{"print-pages": dbus.MakeVariant("all")}
	options := map[string]dbus.Variant{
		"supported_output_file_formats": dbus.MakeVariant([]string{"pdf"}),
		"has_selected_pages":            dbus.MakeVariant(job.Selection.End > job.Selection.Start),
	}
	if job.Selection.End > job.Selection.Start {
		settings["print-pages"] = dbus.MakeVariant("selection")
	}
	result, err := printer.portalRequest("PreparePrint", job.Title, settings, map[string]dbus.Variant{}, options)
	if err != nil {
		printer.Close()
		return nil, err
	}
	parsed, err := parsePortalPrintResult(job, portalValues(result))
	if err != nil {
		printer.Close()
		return nil, err
	}
	printer.device, printer.choice, printer.token = parsed.device, parsed.choice, parsed.token
	printer.device.HeaderLineHeight = renderer.BandHeight(job.Settings.HeaderFont)
	printer.device.FooterLineHeight = renderer.BandHeight(job.Settings.FooterFont)
	if _, err := BodyBounds(job, printer.device); err != nil {
		printer.Close()
		return nil, err
	}
	return printer, nil
}

func (printer *Printer) portalRequest(method, title string, arguments ...any) (map[string]dbus.Variant, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	bus := printer.bus
	match := []dbus.MatchOption{dbus.WithMatchInterface("org.freedesktop.portal.Request"), dbus.WithMatchMember("Response")}
	if err := bus.AddMatchSignalContext(ctx, match...); err != nil {
		return nil, fmt.Errorf("printing: watch Linux print portal: %w", err)
	}
	defer func() {
		removeContext, removeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer removeCancel()
		bus.RemoveMatchSignalContext(removeContext, match...)
	}()
	signals := make(chan *dbus.Signal, 8)
	bus.Signal(signals)
	defer bus.RemoveSignal(signals)
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	options, ok := arguments[len(arguments)-1].(map[string]dbus.Variant)
	if !ok {
		return nil, fmt.Errorf("printing: invalid portal request options")
	}
	options["handle_token"] = dbus.MakeVariant("vivapad_" + hex.EncodeToString(nonce[:]))
	args := make([]any, 0, len(arguments)+2)
	args = append(args, "", title)
	args = append(args, arguments...)
	var handle dbus.ObjectPath
	if err := bus.Object(portalName, portalPath).CallWithContext(ctx, "org.freedesktop.portal.Print."+method, 0, args...).Store(&handle); err != nil {
		return nil, fmt.Errorf("printing: Linux print portal %s: %w", method, err)
	}
	if !handle.IsValid() {
		return nil, fmt.Errorf("printing: Linux print portal returned an invalid request")
	}
	for {
		select {
		case signal, open := <-signals:
			if !open {
				return nil, fmt.Errorf("printing: Linux print portal disconnected")
			}
			if signal.Path != handle || signal.Name != "org.freedesktop.portal.Request.Response" || len(signal.Body) != 2 {
				continue
			}
			status, ok := signal.Body[0].(uint32)
			if !ok {
				return nil, fmt.Errorf("printing: invalid Linux print portal response")
			}
			if status == 1 {
				return nil, ErrPrintCanceled
			}
			if status != 0 {
				return nil, fmt.Errorf("printing: Linux print portal rejected %s (%d)", method, status)
			}
			result, ok := signal.Body[1].(map[string]dbus.Variant)
			if !ok {
				return nil, fmt.Errorf("printing: invalid Linux print portal result")
			}
			return result, nil
		case <-ctx.Done():
			closeContext, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
			bus.Object(portalName, handle).CallWithContext(closeContext, "org.freedesktop.portal.Request.Close", 0)
			closeCancel()
			return nil, fmt.Errorf("printing: Linux print portal %s: %w", method, ctx.Err())
		}
	}
}

func (printer *Printer) Metrics() (DeviceMetrics, TextMetrics) {
	if printer == nil {
		return DeviceMetrics{}, TextMetrics{}
	}
	return printer.device, printer.renderer.Metrics()
}

func (printer *Printer) Choice() PrintChoice {
	if printer == nil {
		return PrintChoice{}
	}
	return printer.choice
}

func (printer *Printer) PrintPages(job Job, pages []Page) error {
	if printer == nil {
		return fmt.Errorf("printing: Linux print portal is missing")
	}
	printer.mu.Lock()
	defer printer.mu.Unlock()
	if printer.bus == nil && !printer.cups || job.Scope != printer.choice.Scope {
		return fmt.Errorf("printing: Linux print portal is closed or page scope changed")
	}
	file, err := os.CreateTemp("", "vivapad-print-*.pdf")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := printer.writePDF(file, job, pages); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if printer.cups {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		title := job.Title
		if title == "" {
			title = "VivaPad"
		}
		output, err := exec.CommandContext(ctx, "lp", "-t", title, file.Name()).CombinedOutput()
		if err != nil {
			return fmt.Errorf("printing: CUPS rejected the PDF: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	_, err = printer.portalRequest("Print", job.Title, dbus.UnixFD(file.Fd()), map[string]dbus.Variant{
		"token":                         dbus.MakeVariant(printer.token),
		"supported_output_file_formats": dbus.MakeVariant([]string{"pdf"}),
	})
	return err
}

func (printer *Printer) writePDF(output io.Writer, job Job, pages []Page) error {
	if len(printer.choice.Ranges) != 0 {
		// Portal submission sends a ready-to-print PDF. Apply the range before
		// rendering, leaving copy count and other spool settings with the portal.
		selected, err := SelectPageRanges(pages, printer.choice.Ranges)
		if err != nil {
			return err
		}
		pages = selected
	}
	return printer.renderer.writePDF(output, job, pages, printer.device)
}

func (printer *Printer) Close() {
	if printer == nil {
		return
	}
	printer.mu.Lock()
	defer printer.mu.Unlock()
	if printer.bus != nil {
		printer.bus.Close()
		printer.bus = nil
	}
	printer.cups = false
}
