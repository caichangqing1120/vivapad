package printing

import (
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
	"github.com/rivo/uniseg"
	"github.com/vivaui/viva/gfx"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type pdfFont struct {
	name string
	data []byte
	face *sfnt.Font
}

type pdfFontRun struct {
	text string
	font int
	// fpdf's CID encoding covers the BMP only; other single glyphs use sfnt outlines.
	vector bool
}

type pdfRenderer struct {
	fonts      []pdfFont
	measure    *fpdf.Fpdf
	mu         sync.Mutex
	fontSize   float64
	lineHeight float64
	tabWidth   int
}

func newPDFRenderer(font []byte, fontSize float64, tabWidth int) (*pdfRenderer, error) {
	return newPDFRendererWithFallback(font, fontSize, tabWidth)
}

func newPDFRendererWithFallback(font []byte, fontSize float64, tabWidth int, fallbacks ...[]byte) (*pdfRenderer, error) {
	if len(font) == 0 || math.IsNaN(fontSize) || math.IsInf(fontSize, 0) || fontSize <= 0 || fontSize > 256 || tabWidth < 1 || tabWidth > 16 {
		return nil, fmt.Errorf("printing: invalid PDF font or tab width")
	}
	fonts := make([]pdfFont, 0, len(fallbacks)+1)
	for index, data := range append([][]byte{font}, fallbacks...) {
		face, err := sfnt.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("printing: parse PDF font %d: %w", index, err)
		}
		name := "VivaPad"
		if index > 0 {
			name = fmt.Sprintf("VivaPadFallback%d", index)
		}
		fonts = append(fonts, pdfFont{name: name, data: data, face: face})
	}
	measure := newPDFDocument(fonts, fontSize)
	if err := measure.Error(); err != nil {
		return nil, fmt.Errorf("printing: prepare PDF font: %w", err)
	}
	description := measure.GetFontDesc("", "")
	lineHeight := fontSize * (float64(description.Ascent-description.Descent)/1000 + 0.15)
	if lineHeight <= fontSize || lineHeight > fontSize*3 {
		return nil, fmt.Errorf("printing: invalid PDF font line height")
	}
	return &pdfRenderer{fonts: fonts, measure: measure, fontSize: fontSize, lineHeight: lineHeight, tabWidth: tabWidth}, nil
}

func newPDFDocument(fonts []pdfFont, size float64) *fpdf.Fpdf {
	pdf := fpdf.New("P", "pt", "A4", "")
	for _, font := range fonts {
		pdf.AddUTF8FontFromBytes(font.name, "", font.data)
	}
	pdf.SetFont(fonts[0].name, "", size)
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
			runs, err := renderer.fontRuns(cluster)
			if err != nil {
				return renderer.measure.GetStringWidth(cluster)
			}
			width, err := renderer.runWidth(renderer.measure, runs, renderer.fontSize)
			renderer.measure.SetFont(renderer.fonts[0].name, "", renderer.fontSize)
			if err != nil {
				return renderer.measure.GetStringWidth(cluster)
			}
			return width
		},
	}
}

func (renderer *pdfRenderer) BandHeight(font BandFont) float64 {
	return renderer.lineHeight * float64(font.Points()) / renderer.fontSize
}

func (renderer *pdfRenderer) fontRuns(value string) ([]pdfFontRun, error) {
	var buffer sfnt.Buffer
	var runs []pdfFontRun
	var current strings.Builder
	last := -1
	lastVector := false
	flush := func() {
		if current.Len() > 0 {
			runs = append(runs, pdfFontRun{text: current.String(), font: last, vector: lastVector})
			current.Reset()
		}
	}
	for graphemes := uniseg.NewGraphemes(value); graphemes.Next(); {
		cluster := graphemes.Str()
		selected := -1
		for index, font := range renderer.fonts {
			covered := true
			for _, character := range cluster {
				if character == '\n' || character == '\r' {
					continue
				}
				glyph, err := font.face.GlyphIndex(&buffer, character)
				if err != nil || glyph == 0 {
					covered = false
					break
				}
			}
			if covered {
				selected = index
				break
			}
		}
		if selected < 0 {
			for _, character := range cluster {
				covered := false
				for _, font := range renderer.fonts {
					glyph, err := font.face.GlyphIndex(&buffer, character)
					covered = covered || err == nil && glyph != 0
				}
				if !covered {
					return nil, fmt.Errorf("printing: PDF font lacks glyph U+%04X", character)
				}
			}
			return nil, fmt.Errorf("printing: no PDF font covers grapheme %q", cluster)
		}
		vector := false
		for _, character := range cluster {
			vector = vector || character > 0xffff
		}
		if vector && utf8.RuneCountInString(cluster) != 1 {
			return nil, fmt.Errorf("printing: PDF font cannot shape grapheme %q", cluster)
		}
		if selected != last || vector || lastVector {
			flush()
			last = selected
			lastVector = vector
		}
		current.WriteString(cluster)
	}
	flush()
	return runs, nil
}

func (renderer *pdfRenderer) vectorAdvance(run pdfFontRun, size float64) (float64, error) {
	character, _ := utf8.DecodeRuneInString(run.text)
	var buffer sfnt.Buffer
	glyph, err := renderer.fonts[run.font].face.GlyphIndex(&buffer, character)
	if err != nil || glyph == 0 {
		return 0, fmt.Errorf("printing: PDF font lacks glyph U+%04X: %v", character, err)
	}
	advance, err := renderer.fonts[run.font].face.GlyphAdvance(&buffer, glyph,
		fixed.Int26_6(math.Round(size*64)), xfont.HintingNone)
	if err != nil {
		return 0, fmt.Errorf("printing: measure PDF glyph U+%04X: %w", character, err)
	}
	return float64(advance) / 64, nil
}

func (renderer *pdfRenderer) runWidth(pdf *fpdf.Fpdf, runs []pdfFontRun, size float64) (float64, error) {
	var width float64
	for _, run := range runs {
		if run.vector {
			advance, err := renderer.vectorAdvance(run, size)
			if err != nil {
				return 0, err
			}
			width += advance
			continue
		}
		pdf.SetFont(renderer.fonts[run.font].name, "", size)
		width += pdf.GetStringWidth(run.text)
	}
	return width, nil
}

func (renderer *pdfRenderer) paintRun(pdf *fpdf.Fpdf, run pdfFontRun, x, y, size float64, bold bool) (float64, error) {
	if !run.vector {
		pdf.SetFont(renderer.fonts[run.font].name, "", size)
		pdf.Text(x, y, run.text)
		return pdf.GetStringWidth(run.text), nil
	}
	character, _ := utf8.DecodeRuneInString(run.text)
	advance, err := renderer.vectorAdvance(run, size)
	if err != nil {
		return 0, err
	}
	var buffer sfnt.Buffer
	face := renderer.fonts[run.font].face
	glyph, err := face.GlyphIndex(&buffer, character)
	if err != nil || glyph == 0 {
		return 0, fmt.Errorf("printing: PDF font lacks glyph U+%04X: %v", character, err)
	}
	segments, err := face.LoadGlyph(&buffer, glyph, fixed.Int26_6(math.Round(size*64)), nil)
	if err != nil {
		return 0, fmt.Errorf("printing: outline PDF glyph U+%04X: %w", character, err)
	}
	if len(segments) == 0 {
		return 0, fmt.Errorf("printing: PDF glyph U+%04X has no printable outline", character)
	}
	red, green, blue := pdf.GetTextColor()
	pdf.SetFillColor(red, green, blue)
	if bold {
		pdf.SetDrawColor(red, green, blue)
	}
	point := func(value fixed.Point26_6) (float64, float64) {
		return x + float64(value.X)/64, y + float64(value.Y)/64
	}
	var currentX, currentY float64
	open := false
	for _, segment := range segments {
		switch segment.Op {
		case sfnt.SegmentOpMoveTo:
			if open {
				pdf.ClosePath()
			}
			currentX, currentY = point(segment.Args[0])
			pdf.MoveTo(currentX, currentY)
			open = true
		case sfnt.SegmentOpLineTo:
			currentX, currentY = point(segment.Args[0])
			pdf.LineTo(currentX, currentY)
		case sfnt.SegmentOpQuadTo:
			controlX, controlY := point(segment.Args[0])
			endX, endY := point(segment.Args[1])
			pdf.CurveBezierCubicTo(currentX+2*(controlX-currentX)/3, currentY+2*(controlY-currentY)/3,
				endX+2*(controlX-endX)/3, endY+2*(controlY-endY)/3, endX, endY)
			currentX, currentY = endX, endY
		case sfnt.SegmentOpCubeTo:
			firstX, firstY := point(segment.Args[0])
			secondX, secondY := point(segment.Args[1])
			currentX, currentY = point(segment.Args[2])
			pdf.CurveBezierCubicTo(firstX, firstY, secondX, secondY, currentX, currentY)
		default:
			return 0, fmt.Errorf("printing: unknown PDF glyph segment for U+%04X", character)
		}
	}
	pdf.ClosePath()
	style := "F"
	if bold {
		style = "DF"
	}
	pdf.DrawPath(style)
	return advance, nil
}

func (renderer *pdfRenderer) writePDF(output io.Writer, job Job, pages []Page, device DeviceMetrics) error {
	if renderer == nil || len(pages) == 0 || len(pages) > maxPrintPages {
		return ErrNoPages
	}
	if _, err := BodyBounds(job, device); err != nil {
		return err
	}
	pdf := newPDFDocument(renderer.fonts, renderer.fontSize)
	if err := pdf.Error(); err != nil {
		return err
	}
	baseAscent := float64(pdf.GetFontDesc("", "").Ascent) * renderer.fontSize / 1000
	for _, page := range pages {
		if page.Number < 1 || len(page.Styles) != 0 && len(page.Styles) != len(page.Rows) {
			return fmt.Errorf("printing: invalid PDF page rows or styles")
		}
		pdf.AddPageFormat("P", fpdf.SizeType{Wd: device.PaperWidth, Ht: device.PaperHeight})
		if err := renderer.drawPDFBand(pdf, page.Header, job.Settings.HeaderFont, page.Bounds, true); err != nil {
			return err
		}
		if err := renderer.drawPDFBand(pdf, page.Footer, job.Settings.FooterFont, page.Bounds, false); err != nil {
			return err
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
				return err
			}
			y := page.Bounds.Top + float64(index)*renderer.lineHeight + baseAscent
			if row.LineNumber > 0 && job.Settings.LineNumbers {
				pdf.SetFont(renderer.fonts[0].name, "", renderer.fontSize)
				pdf.SetTextColor(112, 116, 123)
				pdf.Text(page.Bounds.Left, y, fmt.Sprint(row.LineNumber))
			}
			x := page.TextBounds.Left
			for _, segment := range segments {
				runs, err := renderer.fontRuns(segment.Text)
				if err != nil {
					return err
				}
				red, green, blue := pdfColor(segment.Color)
				pdf.SetTextColor(red, green, blue)
				for _, run := range runs {
					advance, err := renderer.paintRun(pdf, run, x, y, renderer.fontSize, false)
					if err != nil {
						return err
					}
					x += advance
				}
			}
		}
	}
	if err := pdf.Output(output); err != nil {
		return fmt.Errorf("printing: write PDF: %w", err)
	}
	return nil
}

func (renderer *pdfRenderer) drawPDFBand(pdf *fpdf.Fpdf, band TextBand, font BandFont, bounds PageBounds, top bool) error {
	if band == (TextBand{}) {
		return nil
	}
	size := float64(font.Points())
	pdf.SetFontSize(size)
	defer pdf.SetFont(renderer.fonts[0].name, "", renderer.fontSize)
	values := []string{band.Left, band.Middle, band.Right}
	var runs [3][]pdfFontRun
	var widths [3]float64
	for index, value := range values {
		var err error
		runs[index], err = renderer.fontRuns(value)
		if err != nil {
			return err
		}
		widths[index], err = renderer.runWidth(pdf, runs[index], size)
		if err != nil {
			return err
		}
	}
	lineHeight := renderer.BandHeight(font)
	baseline := bounds.Bottom + lineHeight*0.5
	if top {
		baseline = bounds.Top - lineHeight*1.5
	}
	pdf.SetFont(renderer.fonts[0].name, "", size)
	baseline += float64(pdf.GetFontDesc("", "").Ascent) * size / 1000
	pdf.SetTextColor(0, 0, 0)
	if font.Bold {
		pdf.SetLineWidth(0.25)
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetTextRenderingMode(2)
	}
	for _, item := range []struct {
		runs []pdfFontRun
		text string
		x    float64
	}{
		{runs[0], band.Left, bounds.Left},
		{runs[1], band.Middle, (bounds.Left + bounds.Right - widths[1]) / 2},
		{runs[2], band.Right, bounds.Right - widths[2]},
	} {
		if font.Italic && item.text != "" {
			pdf.TransformBegin()
			pdf.TransformSkewX(12, item.x, baseline)
		}
		x := item.x
		for _, run := range item.runs {
			advance, err := renderer.paintRun(pdf, run, x, baseline, size, font.Bold)
			if err != nil {
				return err
			}
			x += advance
		}
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
