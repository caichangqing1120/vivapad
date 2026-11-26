package printing

import (
	"fmt"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/sfnt"
)

type pdfFontStyle uint8

const (
	pdfBold pdfFontStyle = 1 << iota
	pdfItalic
)

type pdfBandFonts struct {
	bold, italic, boldItalic []byte
}

func newPDFRendererWithBandFonts(font []byte, fontSize float64, tabWidth int, band pdfBandFonts, fallbacks ...[]byte) (*pdfRenderer, error) {
	renderer, err := newPDFRendererWithFallback(font, fontSize, tabWidth, fallbacks...)
	if err != nil {
		return nil, err
	}
	for _, variant := range []struct {
		name  string
		style pdfFontStyle
		data  []byte
	}{
		{"VivaPadBandBold", pdfBold, band.bold},
		{"VivaPadBandItalic", pdfItalic, band.italic},
		{"VivaPadBandBoldItalic", pdfBold | pdfItalic, band.boldItalic},
	} {
		if len(variant.data) == 0 {
			continue
		}
		face, err := sfnt.Parse(variant.data)
		if err != nil {
			return nil, fmt.Errorf("printing: parse PDF band font %s: %w", variant.name, err)
		}
		renderer.fonts = append(renderer.fonts, pdfFont{name: variant.name, data: variant.data, face: face, style: variant.style})
	}
	renderer.prepareFontOrder()
	return renderer, nil
}

func pdfBandStyle(font BandFont) pdfFontStyle {
	var style pdfFontStyle
	if font.Bold {
		style |= pdfBold
	}
	if font.Italic {
		style |= pdfItalic
	}
	return style
}

func (renderer *pdfRenderer) prepareFontOrder() {
	for style := range renderer.fontOrder {
		renderer.fontOrder[style] = renderer.fontCandidates(pdfFontStyle(style))
	}
}

func (renderer *pdfRenderer) fontCandidates(style pdfFontStyle) []int {
	candidates := make([]int, 0, len(renderer.fonts))
	if style != 0 {
		// Prefer an exact face. If both styles are requested, try real bold
		// before synthesizing weight on an italic outline.
		order := []pdfFontStyle{style}
		if style == pdfBold|pdfItalic {
			order = append(order, pdfBold, pdfItalic)
		}
		for _, target := range order {
			for index := renderer.bodyCount; index < len(renderer.fonts); index++ {
				if renderer.fonts[index].style == target {
					candidates = append(candidates, index)
				}
			}
		}
	}
	for index := range renderer.bodyCount {
		candidates = append(candidates, index)
	}
	return candidates
}

func (renderer *pdfRenderer) paintBandRun(pdf *fpdf.Fpdf, run pdfFontRun, x, y, size float64, style pdfFontStyle) (float64, error) {
	missing := style &^ renderer.fonts[run.font].style
	bold := missing&pdfBold != 0
	if bold {
		width := pdf.GetLineWidth()
		pdf.SetLineWidth(0.25)
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetTextRenderingMode(2)
		defer func() {
			pdf.SetTextRenderingMode(0)
			pdf.SetLineWidth(width)
		}()
	}
	if missing&pdfItalic != 0 {
		red, green, blue := pdf.GetFillColor()
		pdf.TransformBegin()
		pdf.TransformSkewX(12, x, y)
		defer func() {
			pdf.TransformEnd()
			if run.vector {
				// Q restores the PDF fill but not fpdf's cached fill.
				pdf.SetFillColor(red, green, blue)
			}
		}()
	}
	return renderer.paintRun(pdf, run, x, y, size, bold)
}
