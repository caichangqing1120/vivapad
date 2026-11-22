package document

import "unicode"

// SummaryCounts describes the live UTF-8 buffer with its physical line endings.
type SummaryCounts struct {
	Characters, Words, Lines, Bytes, SelectedBytes int
}

// SummaryCounts measures the buffer and a selection given in rune indices.
func (doc *Document) SummaryCounts(startRune, endRune int) SummaryCounts {
	if startRune > endRune {
		startRune, endRune = endRune, startRune
	}
	start, end := doc.Editor.ByteOffset(startRune), doc.Editor.ByteOffset(endRune)
	value := doc.Editor.String()
	breaks := doc.currentLineEndings()
	defaultExtra := len(doc.newline) - 1
	counts := SummaryCounts{Characters: doc.Editor.RuneCount(), Lines: 1, Bytes: len(value), SelectedBytes: end - start}
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
			if index >= start && index < end {
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
