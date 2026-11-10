package document

import (
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

type SearchOptions struct {
	MatchCase bool
	WholeWord bool
	Regex     bool
}

type ReplacementEdit struct {
	Start, End int
	Text       string
}

func compileSearch(query string, options SearchOptions) (*regexp.Regexp, error) {
	if query == "" {
		return nil, nil
	}
	pattern := query
	if !options.Regex {
		pattern = regexp.QuoteMeta(query)
	}
	if !options.MatchCase {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

func searchMatches(value string, expression *regexp.Regexp, options SearchOptions) [][]int {
	if expression == nil {
		return nil
	}
	all := expression.FindAllStringSubmatchIndex(value, -1)
	filtered := all[:0]
	for _, match := range all {
		start, end := match[0], match[1]
		if start == end {
			continue
		}
		if !options.WholeWord {
			filtered = append(filtered, match)
			continue
		}
		before, after := rune(0), rune(0)
		if start > 0 {
			before, _ = utf8.DecodeLastRuneInString(value[:start])
		}
		if end < len(value) {
			after, _ = utf8.DecodeRuneInString(value[end:])
		}
		if !wordRune(before) && !wordRune(after) {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

func wordRune(value rune) bool {
	return value == '_' || unicode.IsLetter(value) || unicode.IsDigit(value)
}

func (doc *Document) WordAtCaret() (text.Range, bool) {
	characters := []rune(doc.Editor.String())
	caret := doc.Editor.Caret()
	start, end := caret, caret
	for start > 0 && wordRune(characters[start-1]) {
		start--
	}
	for end < len(characters) && wordRune(characters[end]) {
		end++
	}
	return text.Range{Start: start, End: end}, start < end
}

func byteAtRune(value string, offset int) int {
	if offset <= 0 {
		return 0
	}
	for index := range value {
		if offset == 0 {
			return index
		}
		offset--
	}
	return len(value)
}

func (doc *Document) FindWith(query string, options SearchOptions, backward bool) (bool, error) {
	expression, err := compileSearch(query, options)
	if err != nil || expression == nil {
		return false, err
	}
	value := doc.Editor.String()
	matches := searchMatches(value, expression, options)
	selection := doc.Editor.Selection()
	start := doc.Editor.Caret()
	if selection.Start != selection.End {
		if backward {
			start = selection.Start
		} else {
			start = selection.End
		}
	}
	byteStart := byteAtRune(value, start)
	chosen := -1
	if backward {
		for index, match := range matches {
			if match[1] <= byteStart {
				chosen = index
			}
		}
		if chosen < 0 && len(matches) != 0 {
			chosen = len(matches) - 1
		}
	} else {
		for index, match := range matches {
			if match[0] >= byteStart {
				chosen = index
				break
			}
		}
		if chosen < 0 && len(matches) != 0 {
			chosen = 0
		}
	}
	if chosen < 0 {
		return false, nil
	}
	match := matches[chosen]
	from := utf8.RuneCountInString(value[:match[0]])
	to := from + utf8.RuneCountInString(value[match[0]:match[1]])
	if backward {
		doc.Editor.SetSelection(text.Range{Start: to, End: from})
	} else {
		doc.Editor.SetSelection(text.Range{Start: from, End: to})
	}
	return true, nil
}

func (doc *Document) MatchesWith(query string, options SearchOptions) ([]text.Span, error) {
	expression, err := compileSearch(query, options)
	if err != nil || expression == nil {
		return nil, err
	}
	matches := searchMatches(doc.Editor.String(), expression, options)
	spans := make([]text.Span, 0, len(matches))
	for _, match := range matches {
		spans = append(spans, text.Span{Start: match[0], End: match[1]})
	}
	return spans, nil
}

func (doc *Document) ReplaceCurrentWith(query, replacement string, options SearchOptions) (bool, error) {
	replaced, _, err := doc.ReplaceCurrentWithEdit(query, replacement, options)
	return replaced, err
}

func (doc *Document) ReplaceCurrentWithEdit(query, replacement string, options SearchOptions) (bool, ReplacementEdit, error) {
	expression, err := compileSearch(query, options)
	if err != nil || expression == nil {
		return false, ReplacementEdit{}, err
	}
	value := doc.Editor.String()
	selection := doc.Editor.Selection()
	if selection.Start == selection.End {
		return false, ReplacementEdit{}, nil
	}
	from, to := byteAtRune(value, selection.Start), byteAtRune(value, selection.End)
	for _, match := range searchMatches(value, expression, options) {
		if match[0] == from && match[1] == to {
			insert := replacement
			if options.Regex {
				insert = string(expression.ExpandString(nil, replacement, value, match))
			}
			doc.Editor.Insert(insert)
			return true, ReplacementEdit{Start: from, End: to, Text: insert}, nil
		}
	}
	return false, ReplacementEdit{}, nil
}

func (doc *Document) ReplaceAllWith(query, replacement string, options SearchOptions) (int, error) {
	count, _, err := doc.ReplaceAllWithEdits(query, replacement, options)
	return count, err
}

func (doc *Document) ReplaceAllWithEdits(query, replacement string, options SearchOptions) (int, []ReplacementEdit, error) {
	expression, err := compileSearch(query, options)
	if err != nil || expression == nil {
		return 0, nil, err
	}
	value := doc.Editor.String()
	matches := searchMatches(value, expression, options)
	edits := make([]ReplacementEdit, 0, len(matches))
	for _, match := range matches {
		if match[0] == match[1] {
			continue
		}
		insert := replacement
		if options.Regex {
			insert = string(expression.ExpandString(nil, replacement, value, match))
		}
		edits = append(edits, ReplacementEdit{Start: match[0], End: match[1], Text: insert})
	}
	if err := doc.applyTargetEdits(edits); err != nil {
		return 0, nil, err
	}
	return len(edits), edits, nil
}
