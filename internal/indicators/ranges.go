package indicators

import "github.com/vivaui/viva/text"

// Replace applies Scintilla's DeleteRange then InsertSpace indicator rules.
// Offsets and lengths are bytes; the input is not changed, including on error.
func Replace(spans []text.Span, length, at, removed, inserted int) ([]text.Span, bool) {
	if length < 0 || at < 0 || at > length || removed < 0 || removed > length-at || inserted < 0 ||
		inserted > int(^uint(0)>>1)-(length-removed) {
		return nil, false
	}
	result := make([]text.Span, 0, len(spans))
	previousEnd := 0
	for _, span := range spans {
		if span.Start < previousEnd || span.Start < 0 || span.End < span.Start || span.End > length {
			return nil, false
		}
		previousEnd = span.End
		if span.Start == span.End {
			continue
		}
		if len(result) > 0 && span.Start == result[len(result)-1].End {
			result[len(result)-1].End = span.End
		} else {
			result = append(result, span)
		}
	}
	mapDeleted := func(position int) int {
		if position <= at {
			return position
		}
		if position < at+removed {
			return at
		}
		return position - removed
	}
	deleted := result[:0]
	for _, span := range result {
		span.Start, span.End = mapDeleted(span.Start), mapDeleted(span.End)
		if span.Start == span.End {
			continue
		}
		if len(deleted) > 0 && span.Start <= deleted[len(deleted)-1].End {
			deleted[len(deleted)-1].End = max(deleted[len(deleted)-1].End, span.End)
		} else {
			deleted = append(deleted, span)
		}
	}
	for index, span := range deleted {
		if inserted > 0 {
			if at <= span.Start {
				span.Start, span.End = span.Start+inserted, span.End+inserted
			} else if at < span.End {
				span.End += inserted
			}
		}
		deleted[index] = span
	}
	return deleted, true
}
