package document

import (
	"regexp"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

func (doc *Document) FindIncremental(query string, matchCase, backward, advance bool) (bool, bool) {
	expression, err := compileSearch(query, SearchOptions{MatchCase: matchCase})
	if err != nil || expression == nil {
		return false, false
	}
	value := doc.Editor.String()
	selection := doc.Editor.Selection()
	position := selection.Start
	if backward {
		position = selection.End
		if advance {
			position--
		}
	} else if advance {
		position++
	}
	boundary := byteAtRune(value, position)
	var first, last, chosen [2]int
	found, chosenInDirection := false, false
	walkIncrementalMatches(value, expression, func(start, end int) {
		if !found {
			first = [2]int{start, end}
		}
		last, found = [2]int{start, end}, true
		if backward {
			if end <= boundary {
				chosen, chosenInDirection = last, true
			}
		} else if !chosenInDirection && start >= boundary {
			chosen, chosenInDirection = last, true
		}
	})
	if !found {
		return false, false
	}
	wrapped := !chosenInDirection
	if wrapped {
		if backward {
			chosen = last
		} else {
			chosen = first
		}
	}
	from := utf8.RuneCountInString(value[:chosen[0]])
	to := from + utf8.RuneCountInString(value[chosen[0]:chosen[1]])
	if backward {
		doc.Editor.SetSelection(text.Range{Start: to, End: from})
	} else {
		doc.Editor.SetSelection(text.Range{Start: from, End: to})
	}
	return true, wrapped
}

func (doc *Document) IncrementalMatches(query string, matchCase bool) []text.Span {
	expression, err := compileSearch(query, SearchOptions{MatchCase: matchCase})
	if err != nil || expression == nil {
		return nil
	}
	value := doc.Editor.String()
	var spans []text.Span
	walkIncrementalMatches(value, expression, func(start, end int) {
		spans = append(spans, text.Span{Start: start, End: end})
	})
	return spans
}

func walkIncrementalMatches(value string, expression *regexp.Regexp, visit func(start, end int)) {
	for offset := 0; offset < len(value); {
		match := expression.FindStringIndex(value[offset:])
		if match == nil {
			return
		}
		start, end := offset+match[0], offset+match[1]
		visit(start, end)
		_, width := utf8.DecodeRuneInString(value[start:])
		offset = start + max(1, width)
	}
}
