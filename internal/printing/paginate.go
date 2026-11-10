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
	newPage := func() error {
		if len(pages) >= maxPrintPages {
			return fmt.Errorf("printing: too many pages")
		}
		number := len(pages) + 1
		pages = append(pages, Page{
			Number: number, Bounds: bounds, TextBounds: textBounds,
			Header: expandBand(job.Settings.Header, number, variables),
			Footer: expandBand(job.Settings.Footer, number, variables),
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
		rows, err := wrapLine(value, sourceStart, sourceLine, width, metrics)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := appendRow(row); err != nil {
				return err
			}
		}
		return nil
	}
	selected := job.Text[start:end]
	lineStart, lineNumber := start, strings.Count(job.Text[:start], "\n")+1
	lines := strings.Split(selected, "\n")
	for index, line := range lines {
		if index == len(lines)-1 && line == "" && strings.HasSuffix(selected, "\n") {
			break
		}
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

type printCluster struct {
	start, end int
	text       string
	canBreak   bool
}

func wrapLine(line string, sourceStart, lineNumber int, width float64, metrics TextMetrics) ([]Row, error) {
	if line == "" {
		return []Row{{StartByte: sourceStart, EndByte: sourceStart, LineNumber: lineNumber}}, nil
	}
	var clusters []printCluster
	for graphemes := uniseg.NewGraphemes(line); graphemes.Next(); {
		start, end := graphemes.Positions()
		clusters = append(clusters, printCluster{start: start, end: end, text: graphemes.Str(), canBreak: graphemes.LineBreak() != uniseg.LineDontBreak})
	}
	var rows []Row
	for start := 0; start < len(clusters); {
		advance, columns, lastBreak, end := float64(0), 0, -1, start
		for end < len(clusters) {
			printed, nextColumns := expandCluster(clusters[end].text, columns, metrics.TabWidth)
			step := metrics.Advance(printed)
			if !validAdvance(step) || step == 0 && uniseg.StringWidth(printed) > 0 {
				return nil, fmt.Errorf("printing: invalid glyph advance")
			}
			if advance+step > width {
				if end == start {
					return nil, fmt.Errorf("printing: glyph is wider than the printable text area")
				}
				if lastBreak > start {
					end = lastBreak
				}
				break
			}
			advance += step
			columns = nextColumns
			end++
			if clusters[end-1].canBreak {
				lastBreak = end
			}
		}
		var text strings.Builder
		columns = 0
		for index := start; index < end; index++ {
			printed, nextColumns := expandCluster(clusters[index].text, columns, metrics.TabWidth)
			text.WriteString(printed)
			columns = nextColumns
		}
		row := Row{Text: text.String(), StartByte: sourceStart + clusters[start].start, EndByte: sourceStart + clusters[end-1].end}
		if start == 0 {
			row.LineNumber = lineNumber
		}
		rows = append(rows, row)
		start = end
	}
	return rows, nil
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
		part, err := SelectPages(pages, pageRange.First, pageRange.Last)
		if err != nil {
			return nil, err
		}
		selected = append(selected, part...)
		previous = pageRange.Last
	}
	return selected, nil
}

func validAdvance(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
