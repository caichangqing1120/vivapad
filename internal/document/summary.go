package document

import (
	"cmp"
	"slices"
	"unicode"

	"github.com/vivaui/viva/text"
)

// SummaryCounts describes the live UTF-8 buffer with its physical line endings.
type SummaryCounts struct {
	Characters, Words, Lines, Bytes, SelectedBytes int
}

// SummaryCounts measures the buffer and a selection given in rune indices.
func (doc *Document) SummaryCounts(startRune, endRune int) SummaryCounts {
	return doc.SummaryCountsForRanges([]text.Range{{Start: startRune, End: endRune}})
}

// SummaryCountsForRanges measures the buffer and disjoint selections in rune
// indices. Selections may be supplied in editor order rather than text order.
func (doc *Document) SummaryCountsForRanges(ranges []text.Range) SummaryCounts {
	value := doc.Editor.String()
	breaks := doc.currentLineEndings()
	defaultExtra := len(doc.newline) - 1
	counts := SummaryCounts{Characters: doc.Editor.RuneCount(), Lines: 1, Bytes: len(value)}
	var single [1]text.Span
	spans := single[:0]
	for _, r := range ranges {
		if r.Start == r.End {
			continue
		}
		start := doc.Editor.ByteOffset(min(r.Start, r.End))
		end := doc.Editor.ByteOffset(max(r.Start, r.End))
		if start < end {
			spans = append(spans, text.Span{Start: start, End: end})
			counts.SelectedBytes += end - start
		}
	}
	if len(spans) > 1 {
		slices.SortFunc(spans, func(a, b text.Span) int { return cmp.Compare(a.Start, b.Start) })
	}
	insideWord := false
	for index, character := range value {
		switch character {
		case '\n':
			counts.Characters--
			extra := defaultExtra
			if line := counts.Lines - 1; line < len(breaks) {
				extra = len(breaks[line]) - 1
			}
			counts.Bytes += extra
			for len(spans) > 0 && spans[0].End <= index {
				spans = spans[1:]
			}
			if len(spans) > 0 && spans[0].Start <= index {
				counts.SelectedBytes += extra
			}
			counts.Lines++
			insideWord = false
		case '\r':
			counts.Characters--
			insideWord = false
		default:
			word := character == '_' || unicode.IsLetter(character) || unicode.IsNumber(character)
			if word && !insideWord {
				counts.Words++
			}
			insideWord = word
		}
	}
	return counts
}
