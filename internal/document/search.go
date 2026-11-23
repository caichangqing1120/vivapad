package document

import (
	"cmp"
	"context"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
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
		if searchWholeWord(value, start, end) {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

func wordRune(value rune) bool {
	return value == '_' || unicode.IsLetter(value) || unicode.IsDigit(value)
}

func searchWholeWord(value string, start, end int) bool {
	before, _ := utf8.DecodeLastRuneInString(value[:start])
	after, _ := utf8.DecodeRuneInString(value[end:])
	return !wordRune(before) && !wordRune(after)
}

type selectionCharacterClass uint8

const (
	selectionOther selectionCharacterClass = iota
	selectionWord
	selectionPunctuation
)

// Scintilla treats UTF-8 marks and all numeric categories as word characters.
func multiSelectionClass(r rune) selectionCharacterClass {
	switch {
	case r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r):
		return selectionWord
	case unicode.IsPunct(r) || unicode.IsSymbol(r):
		return selectionPunctuation
	default:
		return selectionOther
	}
}

func multiSelectionWholeWord(value string, start, end int) bool {
	match := value[start:end]
	first, _ := utf8.DecodeRuneInString(match)
	last, _ := utf8.DecodeLastRuneInString(match)
	firstClass, lastClass := multiSelectionClass(first), multiSelectionClass(last)
	if firstClass == selectionOther || lastClass == selectionOther {
		return false
	}
	before, beforeWidth := utf8.DecodeLastRuneInString(value[:start])
	after, afterWidth := utf8.DecodeRuneInString(value[end:])
	return (beforeWidth == 0 || multiSelectionClass(before) != firstClass) &&
		(afterWidth == 0 || multiSelectionClass(after) != lastClass)
}

func (doc *Document) MultiSelectionWordAtCaret() (text.Range, bool) {
	value := doc.Editor.String()
	caret := doc.Editor.Caret()
	start, end := caret, caret
	offset := doc.Editor.ByteOffset(caret)
	for left := offset; left > 0; {
		r, width := utf8.DecodeLastRuneInString(value[:left])
		if multiSelectionClass(r) != selectionWord {
			break
		}
		left -= width
		start--
	}
	for right := offset; right < len(value); {
		r, width := utf8.DecodeRuneInString(value[right:])
		if multiSelectionClass(r) != selectionWord {
			break
		}
		right += width
		end++
	}
	return text.Range{Start: start, End: end}, start < end
}

// NextLiteralSelection finds the next non-overlapping occurrence of the
// current selection, wrapping to the first available occurrence if needed.
func (doc *Document) NextLiteralSelection(current text.Range, occupied []text.Range, options SearchOptions) (text.Range, bool) {
	value := doc.Editor.String()
	from, to := doc.Editor.ByteOffset(current.Start), doc.Editor.ByteOffset(current.End)
	if from >= to {
		return text.Range{}, false
	}
	var found text.Range
	visitLiteralMatches(value, value[from:to], options.MatchCase, func(match text.Span) bool {
		if match.Start < to && match.End > from ||
			match.End <= from && found.End != 0 ||
			options.WholeWord && !multiSelectionWholeWord(value, match.Start, match.End) {
			return true
		}
		candidate := text.Range{Start: doc.Editor.RuneIndex(match.Start), End: doc.Editor.RuneIndex(match.End)}
		for _, selection := range occupied {
			if candidate.Start < selection.End && candidate.End > selection.Start {
				return true
			}
		}
		found = candidate
		return match.Start < to
	})
	return found, found.End != 0
}

// AppendLiteralSelections adds unoccupied matches after the current selection,
// then wraps to those before it. The existing selection order is preserved.
func (doc *Document) AppendLiteralSelections(current text.Range, selections []text.Range, options SearchOptions) []text.Range {
	value := doc.Editor.String()
	from, to := doc.Editor.ByteOffset(current.Start), doc.Editor.ByteOffset(current.End)
	if from >= to {
		return selections
	}
	occupied := make([]text.Span, len(selections))
	for i, selection := range selections {
		occupied[i] = text.Span{Start: doc.Editor.ByteOffset(selection.Start), End: doc.Editor.ByteOffset(selection.End)}
	}
	slices.SortFunc(occupied, func(a, b text.Span) int { return cmp.Compare(a.Start, b.Start) })
	firstNew, before := len(selections), 0
	visitLiteralMatches(value, value[from:to], options.MatchCase, func(match text.Span) bool {
		if match.Start < to && match.End > from ||
			options.WholeWord && !multiSelectionWholeWord(value, match.Start, match.End) {
			return true
		}
		for len(occupied) > 0 && occupied[0].End <= match.Start {
			occupied = occupied[1:]
		}
		if len(occupied) > 0 && occupied[0].Start < match.End {
			return true
		}
		selections = append(selections, text.Range{Start: doc.Editor.RuneIndex(match.Start), End: doc.Editor.RuneIndex(match.End)})
		if match.End <= from {
			before++
		}
		return true
	})
	// The stream arrives in document order. Move only the new wrap ranges
	// behind the new matches after the current selection.
	added := selections[firstNew:]
	if before > 0 && before < len(added) {
		slices.Reverse(added[:before])
		slices.Reverse(added[before:])
		slices.Reverse(added)
	}
	return selections
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
	var match text.Span
	if options.Regex {
		match = findRegexSpan(searchMatches(value, expression, options), byteStart, backward)
	} else {
		match = findLiteralSpan(value, query, options, byteStart, backward)
	}
	if match.End == 0 {
		return false, nil
	}
	from := utf8.RuneCountInString(value[:match.Start])
	to := from + utf8.RuneCountInString(value[match.Start:match.End])
	if backward {
		doc.Editor.SetSelection(text.Range{Start: to, End: from})
	} else {
		doc.Editor.SetSelection(text.Range{Start: from, End: to})
	}
	return true, nil
}

func findRegexSpan(matches [][]int, byteStart int, backward bool) text.Span {
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
		return text.Span{}
	}
	match := matches[chosen]
	return text.Span{Start: match[0], End: match[1]}
}

func findLiteralSpan(value, query string, options SearchOptions, byteStart int, backward bool) text.Span {
	var chosen, wrap text.Span
	visitLiteralMatches(value, query, options.MatchCase, func(match text.Span) bool {
		if options.WholeWord && !searchWholeWord(value, match.Start, match.End) {
			return true
		}
		if backward {
			if match.End <= byteStart {
				chosen = match
			} else if chosen.End != 0 {
				return false
			}
			wrap = match
		} else {
			if match.Start >= byteStart {
				chosen = match
				return false
			}
			if wrap.End == 0 {
				wrap = match
			}
		}
		return true
	})
	if chosen.End == 0 {
		return wrap
	}
	return chosen
}

func visitLiteralMatches(value, query string, matchCase bool, visit func(text.Span) bool) {
	if matchCase {
		for offset := 0; offset < len(value); {
			index := strings.Index(value[offset:], query)
			if index < 0 {
				return
			}
			start := offset + index
			offset = start + len(query)
			if !visit(text.Span{Start: start, End: offset}) {
				return
			}
		}
		return
	}
	pattern := []rune(query)
	prefix := make([]int, len(pattern))
	for index, r := range pattern {
		pattern[index] = simpleFoldKey(r)
	}
	// The prefix table keeps case-folded matching linear even when the
	// document repeatedly contains almost the entire search term.
	for index, length := 1, 0; index < len(pattern); index++ {
		for length > 0 && pattern[index] != pattern[length] {
			length = prefix[length-1]
		}
		if pattern[index] == pattern[length] {
			length++
		}
		prefix[index] = length
	}
	length := 0
	for index, r := range value {
		key := simpleFoldKey(r)
		for length > 0 && key != pattern[length] {
			length = prefix[length-1]
		}
		if key == pattern[length] {
			length++
		}
		if length == len(pattern) {
			end := index + utf8.RuneLen(r)
			start := end
			for range pattern {
				_, width := utf8.DecodeLastRuneInString(value[:start])
				start -= width
			}
			if !visit(text.Span{Start: start, End: end}) {
				return
			}
			length = 0 // Match FindAll's non-overlapping sequence.
		}
	}
}

func simpleFoldKey(r rune) rune {
	if r < utf8.RuneSelf {
		if 'a' <= r && r <= 'z' {
			return r - ('a' - 'A')
		}
		return r
	}
	key := r
	for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
		key = min(key, folded)
	}
	return key
}

func (doc *Document) MatchesWith(query string, options SearchOptions) ([]text.Span, error) {
	expression, err := compileSearch(query, options)
	if err != nil || expression == nil {
		return nil, err
	}
	return searchSpans(doc.Editor.String(), expression, options), nil
}

func searchSpans(value string, expression *regexp.Regexp, options SearchOptions) []text.Span {
	matches := searchMatches(value, expression, options)
	spans := make([]text.Span, 0, len(matches))
	for _, match := range matches {
		spans = append(spans, text.Span{Start: match[0], End: match[1]})
	}
	return spans
}

// MatchesWithLimit collects up to limit matches and reports whether more exist.
// Literal searches and regexes safe to bound stop at the first extra match.
// Other regexes retain the full matching path so zero-width and context-
// sensitive anchors keep their existing semantics.
func (doc *Document) MatchesWithLimit(query string, options SearchOptions, limit int) ([]text.Span, bool, error) {
	return doc.MatchesWithLimitContext(context.Background(), query, options, limit)
}

// MatchesWithLimitContext also checks cancellation while scanning rejected
// literal candidates, which is important for directory searches with no hits.
func (doc *Document) MatchesWithLimitContext(ctx context.Context, query string, options SearchOptions, limit int) ([]text.Span, bool, error) {
	return MatchesInTextWithLimit(ctx, doc.Editor.String(), query, options, limit)
}

// MatchesInTextWithLimit searches one text snapshot so callers can build previews
// and byte offsets from the same value.
func MatchesInTextWithLimit(ctx context.Context, value, query string, options SearchOptions, limit int) ([]text.Span, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	expression, err := compileSearch(query, options)
	if err != nil || expression == nil {
		return nil, false, err
	}
	var resumed *regexp.Regexp
	if options.Regex {
		if !options.WholeWord && regexAlwaysConsumes(expression) {
			matches := expression.FindAllStringIndex(value, limit+1)
			if err := ctx.Err(); err != nil {
				return nil, false, err
			}
			truncated := len(matches) > limit
			spans := make([]text.Span, 0, min(len(matches), limit))
			for index, match := range matches {
				if index == limit {
					break
				}
				spans = append(spans, text.Span{Start: match[0], End: match[1]})
			}
			return spans, truncated, nil
		}
		if !options.WholeWord || !regexCanResumeOnSuffix(expression) {
			resumed = compileResumedRegex(expression)
			if resumed == nil {
				// A pattern at Go's compilation limits may not fit the wrapper.
				spans := searchSpans(value, expression, options)
				if canceled := ctx.Err(); canceled != nil {
					return nil, false, canceled
				}
				return spans[:min(len(spans), limit)], len(spans) > limit, nil
			}
		}
	}
	spans := make([]text.Span, 0, min(limit, 64))
	inspected := 0
	collect := func(start, end int) (bool, error) {
		inspected++
		if inspected%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		if start == end {
			return false, nil
		}
		if options.WholeWord {
			before, after := rune(0), rune(0)
			if start > 0 {
				before, _ = utf8.DecodeLastRuneInString(value[:start])
			}
			if end < len(value) {
				after, _ = utf8.DecodeRuneInString(value[end:])
			}
			if wordRune(before) || wordRune(after) {
				return false, nil
			}
		}
		if len(spans) == limit {
			return true, nil
		}
		spans = append(spans, text.Span{Start: start, End: end})
		return false, nil
	}
	for pos := 0; pos < len(value); {
		if resumed != nil {
			start, end, found := resumedRegexMatch(value, pos, expression, resumed)
			if !found {
				break
			}
			truncated, err := collect(start, end)
			if err != nil {
				return nil, false, err
			}
			if truncated {
				return spans, true, nil
			}
			pos = end
			if start == end {
				if pos == len(value) {
					break
				}
				_, width := utf8.DecodeRuneInString(value[pos:])
				pos += width
			}
			continue
		}
		if options.Regex {
			const batchSize = 256
			matches := expression.FindAllStringIndex(value[pos:], batchSize)
			if len(matches) == 0 {
				break
			}
			origin := pos
			for _, match := range matches {
				truncated, err := collect(origin+match[0], origin+match[1])
				if err != nil {
					return nil, false, err
				}
				if truncated {
					return spans, true, nil
				}
			}
			pos = origin + matches[len(matches)-1][1]
			continue
		}
		match := expression.FindStringIndex(value[pos:])
		if match == nil {
			break
		}
		start, end := pos+match[0], pos+match[1]
		pos = end
		truncated, err := collect(start, end)
		if err != nil {
			return nil, false, err
		}
		if truncated {
			return spans, true, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return spans, false, nil
}

func compileResumedRegex(expression *regexp.Regexp) *regexp.Regexp {
	parsed, err := syntax.Parse(expression.String(), syntax.Perl)
	if err != nil {
		return nil
	}
	// Consume the previous rune, then find the earliest match. The single
	// prefix rune preserves line and word context without creating a new \A
	// position for the user's expression. Reprint the syntax so a trailing
	// \Q in the original pattern cannot swallow the wrapper's parenthesis.
	resumed, _ := regexp.Compile(`\A(?s:..*?)(` + parsed.String() + `)`)
	return resumed
}

func resumedRegexMatch(value string, pos int, initial, resumed *regexp.Regexp) (int, int, bool) {
	if pos == 0 {
		match := initial.FindStringIndex(value)
		if match == nil {
			return 0, 0, false
		}
		return match[0], match[1], true
	}
	_, width := utf8.DecodeLastRuneInString(value[:pos])
	origin := pos - width
	match := resumed.FindStringSubmatchIndex(value[origin:])
	if match == nil {
		return 0, 0, false
	}
	return origin + match[2], origin + match[3], true
}

// A minimum positive width proves that FindAll's first limit+1 raw matches
// are also its first limit+1 nonempty results. Unknown syntax stays on the
// original full search path.
func regexAlwaysConsumes(expression *regexp.Regexp) bool {
	parsed, err := syntax.Parse(expression.String(), syntax.Perl)
	if err != nil {
		return false
	}
	var consumes func(*syntax.Regexp) bool
	consumes = func(node *syntax.Regexp) bool {
		switch node.Op {
		case syntax.OpLiteral:
			return len(node.Rune) > 0
		case syntax.OpCharClass, syntax.OpAnyChar, syntax.OpAnyCharNotNL:
			return true
		case syntax.OpCapture, syntax.OpPlus:
			return len(node.Sub) == 1 && consumes(node.Sub[0])
		case syntax.OpRepeat:
			return node.Min > 0 && len(node.Sub) == 1 && consumes(node.Sub[0])
		case syntax.OpConcat:
			for _, child := range node.Sub {
				if consumes(child) {
					return true
				}
			}
		case syntax.OpAlternate:
			if len(node.Sub) == 0 {
				return false
			}
			for _, child := range node.Sub {
				if !consumes(child) {
					return false
				}
			}
			return true
		}
		return false
	}
	return consumes(parsed)
}

// A suffix is equivalent to a search from that byte offset in the original
// input only when the regex never inspects its surrounding text.
func regexCanResumeOnSuffix(expression *regexp.Regexp) bool {
	if !regexAlwaysConsumes(expression) {
		return false
	}
	parsed, err := syntax.Parse(expression.String(), syntax.Perl)
	if err != nil {
		return false
	}
	var independent func(*syntax.Regexp) bool
	independent = func(node *syntax.Regexp) bool {
		switch node.Op {
		case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText,
			syntax.OpWordBoundary, syntax.OpNoWordBoundary:
			return false
		}
		for _, child := range node.Sub {
			if !independent(child) {
				return false
			}
		}
		return true
	}
	return independent(parsed)
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
	if !options.Regex {
		var matched bool
		visitLiteralMatches(value, query, options.MatchCase, func(match text.Span) bool {
			if match.Start < from {
				return true
			}
			matched = match.Start == from && match.End == to &&
				(!options.WholeWord || searchWholeWord(value, from, to))
			return false
		})
		if !matched {
			return false, ReplacementEdit{}, nil
		}
		doc.Editor.Insert(replacement)
		return true, ReplacementEdit{Start: from, End: to, Text: replacement}, nil
	}
	for _, match := range searchMatches(value, expression, options) {
		if match[0] == from && match[1] == to {
			insert := string(expression.ExpandString(nil, replacement, value, match))
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
	edits, err := ReplacementEditsInText(doc.Editor.String(), query, replacement, options)
	if err != nil {
		return 0, nil, err
	}
	if err := doc.applyTargetEdits(edits); err != nil {
		return 0, nil, err
	}
	return len(edits), edits, nil
}

// ReplacementEditsInText computes replacement targets against an immutable UTF-8 snapshot.
func ReplacementEditsInText(value, query, replacement string, options SearchOptions) ([]ReplacementEdit, error) {
	expression, err := compileSearch(query, options)
	if err != nil || expression == nil {
		return nil, err
	}
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
	return edits, nil
}
