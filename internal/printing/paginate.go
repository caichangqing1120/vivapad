package printing

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"vivapad/internal/runcommand"
)

type TextMetrics struct {
	LineHeight float64
	TabWidth   int
	Advance    func(grapheme string) float64
}

type PageVariables struct {
	ShortDate, LongDate, Time string
	NPP                       runcommand.Variables
}

type Row struct {
	Text               string
	StartByte, EndByte int
	LineNumber         int
}

type Page struct {
	Number         int
	Bounds         PageBounds
	TextBounds     PageBounds
	Rows           []Row
	Styles         [][]ColorRun
	Header, Footer TextBand
}

const maxPrintPages = 65535

func Paginate(job Job, device DeviceMetrics, metrics TextMetrics, variables PageVariables) ([]Page, error) {
	bounds, err := BodyBounds(job, device)
	if err != nil {
		return nil, err
	}
	if !validAdvance(metrics.LineHeight) || metrics.LineHeight == 0 || metrics.TabWidth < 1 || metrics.TabWidth > 16 || metrics.Advance == nil {
		return nil, fmt.Errorf("printing: invalid text metrics")
	}
	if !utf8.ValidString(job.Text) {
		return nil, fmt.Errorf("printing: source is not UTF-8")
	}
	start, end := 0, len(job.Text)
	switch job.Scope {
	case ScopeAll:
	case ScopeSelection:
		start, end = job.Selection.Start, job.Selection.End
		if !job.Interactive || start < 0 || end > len(job.Text) || start >= end ||
			start < len(job.Text) && !utf8.RuneStart(job.Text[start]) ||
			end < len(job.Text) && !utf8.RuneStart(job.Text[end]) {
			return nil, fmt.Errorf("printing: invalid selected byte range")
		}
	default:
		return nil, fmt.Errorf("printing: invalid print scope")
	}
	if start == end {
		return nil, nil
	}
	rowsPerPage := math.Floor((bounds.Bottom - bounds.Top) / metrics.LineHeight)
	if rowsPerPage < 1 || rowsPerPage > math.MaxInt32 {
		return nil, fmt.Errorf("printing: no printable text row")
	}
	textBounds := bounds
	if job.Settings.LineNumbers {
		number := strconv.Itoa(strings.Count(job.Text, "\n")+1) + " "
		for graphemes := uniseg.NewGraphemes(number); graphemes.Next(); {
			advance := metrics.Advance(graphemes.Str())
			if !validAdvance(advance) || advance == 0 && graphemes.Width() > 0 {
				return nil, fmt.Errorf("printing: invalid line-number advance")
			}
			textBounds.Left += advance
		}
	}
	width := textBounds.Right - textBounds.Left
	if width <= 0 {
		return nil, fmt.Errorf("printing: no room for text after line numbers")
	}
	var pages []Page
	forcePage := false
	header, footer := prepareBand(job.Settings.Header, variables), prepareBand(job.Settings.Footer, variables)
	newPage := func() error {
		if len(pages) >= maxPrintPages {
			return fmt.Errorf("printing: too many pages")
		}
		number := len(pages) + 1
		pages = append(pages, Page{
			Number: number, Bounds: bounds, TextBounds: textBounds,
			Header: numberBand(header, number),
			Footer: numberBand(footer, number),
		})
		return nil
	}
	appendRow := func(row Row) error {
		if len(pages) == 0 || forcePage || len(pages[len(pages)-1].Rows) >= int(rowsPerPage) {
			if err := newPage(); err != nil {
				return err
			}
		}
		forcePage = false
		pages[len(pages)-1].Rows = append(pages[len(pages)-1].Rows, row)
		return nil
	}
	pageBreak := func() error {
		if len(pages) == 0 || forcePage {
			if err := newPage(); err != nil {
				return err
			}
		}
		forcePage = true
		return nil
	}
	appendText := func(value string, sourceStart, sourceLine int) error {
		return wrapLine(value, sourceStart, sourceLine, width, metrics, appendRow)
	}
	selected := job.Text[start:end]
	lineStart, lineNumber := start, strings.Count(job.Text[:start], "\n")+1
	for line := range strings.SplitSeq(strings.TrimSuffix(selected, "\n"), "\n") {
		if job.Settings.FormFeedPageBreak && strings.ContainsRune(line, '\f') {
			segmentStart := 0
			for position, character := range line {
				if character != '\f' {
					continue
				}
				if position > segmentStart {
					if err := appendText(line[segmentStart:position], lineStart+segmentStart, lineNumber); err != nil {
						return nil, err
					}
				}
				if err := pageBreak(); err != nil {
					return nil, err
				}
				segmentStart = position + 1
			}
			if segmentStart < len(line) {
				if err := appendText(line[segmentStart:], lineStart+segmentStart, lineNumber); err != nil {
					return nil, err
				}
			}
		} else {
			if err := appendText(line, lineStart, lineNumber); err != nil {
				return nil, err
			}
		}
		lineStart += len(line) + 1
		lineNumber++
	}
	return pages, nil
}

func wrapLine(line string, sourceStart, lineNumber int, width float64, metrics TextMetrics, appendRow func(Row) error) error {
	if line == "" {
		return appendRow(Row{StartByte: sourceStart, EndByte: sourceStart, LineNumber: lineNumber})
	}
	rest, state := line, -1
	for rest != "" {
		var text strings.Builder
		advance, columns, startLen := float64(0), 0, len(rest)
		var breakRest string
		var breakState, breakLen int
		for rest != "" {
			cluster, next, boundaries, nextState := uniseg.StepString(rest, state)
			printed, nextColumns := expandCluster(cluster, columns, metrics.TabWidth)
			step := metrics.Advance(printed)
			if !validAdvance(step) || step == 0 && uniseg.StringWidth(printed) > 0 {
				return fmt.Errorf("printing: invalid glyph advance")
			}
			if advance+step > width {
				if text.Len() == 0 {
					return fmt.Errorf("printing: glyph is wider than the printable text area")
				}
				if breakLen > 0 {
					// Resume at the legal break with its Unicode boundary state.
					rest, state = breakRest, breakState
				}
				break
			}
			advance += step
			columns = nextColumns
			rest, state = next, nextState
			text.WriteString(printed)
			if boundaries&uniseg.MaskLine != uniseg.LineDontBreak {
				breakRest, breakState, breakLen = rest, state, text.Len()
			}
		}
		output := text.String()
		if rest == breakRest && breakLen > 0 {
			output = output[:breakLen]
		}
		end := sourceStart + startLen - len(rest)
		if err := appendRow(Row{Text: output, StartByte: sourceStart, EndByte: end, LineNumber: lineNumber}); err != nil {
			return err
		}
		sourceStart, lineNumber = end, 0
	}
	return nil
}

func expandCluster(cluster string, columns, tabWidth int) (string, int) {
	if cluster == "\t" {
		spaces := tabWidth - columns%tabWidth
		return strings.Repeat(" ", spaces), columns + spaces
	}
	return cluster, columns + uniseg.StringWidth(cluster)
}

func SelectPages(pages []Page, first, last int) ([]Page, error) {
	if first < 1 || last < first || first > len(pages) {
		return nil, fmt.Errorf("printing: invalid page range")
	}
	return append([]Page(nil), pages[first-1:min(last, len(pages))]...), nil
}

func SelectPageRanges(pages []Page, ranges []PageRange) ([]Page, error) {
	if len(ranges) == 0 {
		return nil, fmt.Errorf("printing: no page ranges")
	}
	selected := make([]Page, 0)
	previous := 0
	for _, pageRange := range ranges {
		if pageRange.First <= previous {
			return nil, fmt.Errorf("printing: page ranges overlap or are out of order")
		}
		last := pageRange.Last
		if last == pageRangeThroughEnd {
			last = len(pages)
		}
		if last > len(pages) {
			return nil, fmt.Errorf("printing: page range ends at %d, but only %d pages are available", last, len(pages))
		}
		part, err := SelectPages(pages, pageRange.First, last)
		if err != nil {
			return nil, err
		}
		selected = append(selected, part...)
		previous = last
	}
	return selected, nil
}

func validAdvance(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
