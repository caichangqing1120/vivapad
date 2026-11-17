package printing

import (
	"bytes"
	"fmt"
	"math"
	"sync"

	"github.com/go-pdf/fpdf"
	"github.com/vivaui/viva/gfx"
	"golang.org/x/image/font/sfnt"
)

type pdfRenderer struct {
	font       []byte
	face       *sfnt.Font
	measure    *fpdf.Fpdf
	mu         sync.Mutex
	fontSize   float64
	lineHeight float64
	tabWidth   int
}

func newPDFRenderer(font []byte, fontSize float64, tabWidth int) (*pdfRenderer, error) {
	if len(font) == 0 || math.IsNaN(fontSize) || math.IsInf(fontSize, 0) || fontSize <= 0 || fontSize > 256 || tabWidth < 1 || tabWidth > 16 {
		return nil, fmt.Errorf("printing: invalid PDF font or tab width")
	}
	face, err := sfnt.Parse(font)
	if err != nil {
		return nil, fmt.Errorf("printing: parse PDF font: %w", err)
	}
	measure := newPDFDocument(font, fontSize)
	if err := measure.Error(); err != nil {
		return nil, fmt.Errorf("printing: prepare PDF font: %w", err)
	}
	description := measure.GetFontDesc("", "")
	lineHeight := fontSize * (float64(description.Ascent-description.Descent)/1000 + 0.15)
	if lineHeight <= fontSize || lineHeight > fontSize*3 {
		return nil, fmt.Errorf("printing: invalid PDF font line height")
	}
	return &pdfRenderer{font: font, face: face, measure: measure, fontSize: fontSize, lineHeight: lineHeight, tabWidth: tabWidth}, nil
}

func newPDFDocument(font []byte, size float64) *fpdf.Fpdf {
	pdf := fpdf.New("P", "pt", "A4", "")
	pdf.AddUTF8FontFromBytes("VivaPad", "", font)
	pdf.SetFont("VivaPad", "", size)
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	return pdf
}

func (renderer *pdfRenderer) Metrics() TextMetrics {
	return TextMetrics{
		LineHeight: renderer.lineHeight, TabWidth: renderer.tabWidth,
		Advance: func(cluster string) float64 {
			renderer.mu.Lock()
			defer renderer.mu.Unlock()
			return renderer.measure.GetStringWidth(cluster)
		},
	}
}

func (renderer *pdfRenderer) BandHeight(font BandFont) float64 {
	return renderer.lineHeight * float64(font.Points()) / renderer.fontSize
}

func (renderer *pdfRenderer) checkGlyphs(value string) error {
	var buffer sfnt.Buffer
	for _, character := range value {
		if character == '\n' || character == '\r' {
			continue
		}
		glyph, err := renderer.face.GlyphIndex(&buffer, character)
		if err != nil || glyph == 0 {
			return fmt.Errorf("printing: PDF font lacks glyph U+%04X", character)
		}
	}
	return nil
}

func (renderer *pdfRenderer) Render(job Job, pages []Page, device DeviceMetrics) ([]byte, error) {
	if renderer == nil || len(pages) == 0 || len(pages) > maxPrintPages {
		return nil, ErrNoPages
	}
	if _, err := BodyBounds(job, device); err != nil {
		return nil, err
	}
	pdf := newPDFDocument(renderer.font, renderer.fontSize)
	if err := pdf.Error(); err != nil {
		return nil, err
	}
	baseAscent := float64(pdf.GetFontDesc("", "").Ascent) * renderer.fontSize / 1000
	for _, page := range pages {
		if page.Number < 1 || len(page.Styles) != 0 && len(page.Styles) != len(page.Rows) {
			return nil, fmt.Errorf("printing: invalid PDF page rows or styles")
		}
		pdf.AddPageFormat("P", fpdf.SizeType{Wd: device.PaperWidth, Ht: device.PaperHeight})
		if err := renderer.drawPDFBand(pdf, page.Header, job.Settings.HeaderFont, page.Bounds, true); err != nil {
			return nil, err
		}
		if err := renderer.drawPDFBand(pdf, page.Footer, job.Settings.FooterFont, page.Bounds, false); err != nil {
			return nil, err
		}
		for index, row := range page.Rows {
			if printBodyBackground(job.Settings) {
				pdf.SetFillColor(0, 0, 0)
				pdf.Rect(page.TextBounds.Left, page.Bounds.Top+float64(index)*renderer.lineHeight,
					page.TextBounds.Right-page.TextBounds.Left, renderer.lineHeight, "F")
			}
			var styles []ColorRun
			if len(page.Styles) != 0 {
				styles = page.Styles[index]
			}
			segments, err := Segments(job, row, styles, renderer.tabWidth)
			if err != nil {
				return nil, err
			}
			y := page.Bounds.Top + float64(index)*renderer.lineHeight + baseAscent
			if row.LineNumber > 0 && job.Settings.LineNumbers {
				pdf.SetTextColor(112, 116, 123)
				pdf.Text(page.Bounds.Left, y, fmt.Sprint(row.LineNumber))
			}
			x := page.TextBounds.Left
			for _, segment := range segments {
				if err := renderer.checkGlyphs(segment.Text); err != nil {
					return nil, err
				}
				red, green, blue := pdfColor(segment.Color)
				pdf.SetTextColor(red, green, blue)
				pdf.Text(x, y, segment.Text)
				x += pdf.GetStringWidth(segment.Text)
			}
		}
	}
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, fmt.Errorf("printing: render PDF: %w", err)
	}
	return output.Bytes(), nil
}

func (renderer *pdfRenderer) drawPDFBand(pdf *fpdf.Fpdf, band TextBand, font BandFont, bounds PageBounds, top bool) error {
	if band == (TextBand{}) {
		return nil
	}
	size := float64(font.Points())
	pdf.SetFontSize(size)
	defer pdf.SetFontSize(renderer.fontSize)
	for _, value := range []string{band.Left, band.Middle, band.Right} {
		if err := renderer.checkGlyphs(value); err != nil {
			return err
		}
	}
	lineHeight := renderer.BandHeight(font)
	baseline := bounds.Bottom + lineHeight*0.5
	if top {
		baseline = bounds.Top - lineHeight*1.5
	}
	baseline += float64(pdf.GetFontDesc("", "").Ascent) * size / 1000
	pdf.SetTextColor(0, 0, 0)
	if font.Bold {
		pdf.SetLineWidth(0.25)
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetTextRenderingMode(2)
	}
	for _, item := range []struct {
		text string
		x    float64
	}{
		{band.Left, bounds.Left},
		{band.Middle, (bounds.Left + bounds.Right - pdf.GetStringWidth(band.Middle)) / 2},
		{band.Right, bounds.Right - pdf.GetStringWidth(band.Right)},
	} {
		if font.Italic && item.text != "" {
			pdf.TransformBegin()
			pdf.TransformSkewX(12, item.x, baseline)
		}
		pdf.Text(item.x, baseline, item.text)
		if font.Italic && item.text != "" {
			pdf.TransformEnd()
		}
	}
	if font.Bold {
		pdf.SetTextRenderingMode(0)
	}
	pdf.SetDrawColor(0, 0, 0)
	pdf.Line(bounds.Left, pdfBandRuleY(bounds, top), bounds.Right, pdfBandRuleY(bounds, top))
	return pdf.Error()
}

func pdfBandRuleY(bounds PageBounds, top bool) float64 {
	if top {
		return bounds.Top - 4
	}
	return bounds.Bottom + 4
}

func pdfColor(color gfx.Color) (int, int, int) {
	channel := func(value float32) int { return int(math.Round(float64(min(max(value, 0), 1)) * 255)) }
	return channel(color.Red), channel(color.Green), channel(color.Blue)
}
