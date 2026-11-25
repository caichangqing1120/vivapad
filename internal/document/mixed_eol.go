package document

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/vivaui/viva/text"
)

func normalizeLineEndings(value string) (string, string, []string) {
	var normalized strings.Builder
	normalized.Grow(len(value))
	breaks := make([]string, 0, strings.Count(value, "\n")+strings.Count(value, "\r"))
	defaultNewline := "\n"
	for index := 0; index < len(value); index++ {
		ending := ""
		switch value[index] {
		case '\r':
			ending = "\r"
			if index+1 < len(value) && value[index+1] == '\n' {
				ending = "\r\n"
				index++
			}
		case '\n':
			ending = "\n"
		}
		if ending != "" {
			if len(breaks) == 0 {
				defaultNewline = ending
			}
			breaks = append(breaks, ending)
			normalized.WriteByte('\n')
		} else {
			normalized.WriteByte(value[index])
		}
	}
	if !hasMixedBreaks(breaks) {
		breaks = nil
	}
	return normalized.String(), defaultNewline, breaks
}

func hasMixedBreaks(breaks []string) bool {
	if len(breaks) < 2 {
		return false
	}
	for _, ending := range breaks[1:] {
		if ending != breaks[0] {
			return true
		}
	}
	return false
}

func (doc *Document) MixedLineEndings() bool {
	return hasMixedBreaks(doc.currentLineEndings())
}

func (doc *Document) currentLineEndings() []string {
	_, err := doc.currentLFLabels()
	if err != nil || doc.eolUniform {
		return nil
	}
	return doc.eolCache
}

func (doc *Document) currentLFLabels() ([]byte, error) {
	if doc == nil || doc.Editor == nil || doc.Editor != doc.eolEditor {
		return nil, fmt.Errorf("document has no live EOL label source")
	}
	revision, ok := doc.Editor.LFLabelRevision()
	if !ok {
		return nil, fmt.Errorf("document EOL labels are unavailable")
	}
	if !doc.eolCacheValid || doc.eolRevision != revision || doc.eolCacheEOL != doc.newline {
		doc.eolLabels, _ = doc.Editor.LFLabels(doc.eolLabels[:0])
		if cap(doc.eolLabels) > 64 && len(doc.eolLabels) < cap(doc.eolLabels)/4 {
			doc.eolLabels = append([]byte(nil), doc.eolLabels...)
		}
		if cap(doc.eolCache) < len(doc.eolLabels) || cap(doc.eolCache) > 64 && len(doc.eolLabels) < cap(doc.eolCache)/4 {
			doc.eolCache = make([]string, len(doc.eolLabels))
		} else {
			doc.eolCache = doc.eolCache[:len(doc.eolLabels)]
		}
		for index, code := range doc.eolLabels {
			doc.eolCache[index] = lineEndingFromCode(code)
		}
		doc.eolRevision, doc.eolCacheValid = revision, true
		doc.eolCacheEOL, doc.eolUniform = doc.newline, uniformLFLabels(doc.eolLabels, lineEndingCode(doc.newline))
	}
	return doc.eolLabels, nil
}

// CaptureMovedLineEndings validates the live source before a line-move
// transaction and keeps its original labels independent of the draft.
func (doc *Document) CaptureMovedLineEndings() ([]byte, error) {
	labels, err := doc.currentLFLabels()
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), labels...), nil
}

// PreserveMovedLineEndings assigns a moved block its original LF labels after
// its text transaction. The inserted LF identities keep these labels in Undo.
func (doc *Document) PreserveMovedLineEndings(original []byte, sourceLine, targetLine, count int) error {
	if count == 0 {
		return nil
	}
	current, err := doc.currentLFLabels()
	if err != nil {
		return err
	}
	if sourceLine < 0 || targetLine < 0 || count < 0 || sourceLine > len(original)-count || targetLine > len(current)-count {
		return fmt.Errorf("moved line ending range is unavailable")
	}
	updated := append([]byte(nil), current...)
	copy(updated[targetLine:targetLine+count], original[sourceLine:sourceLine+count])
	if bytes.Equal(updated, current) {
		return nil
	}
	if !doc.Editor.SetLFLabels(updated, lineEndingCode(doc.newline)) {
		return fmt.Errorf("cannot preserve moved line endings")
	}
	return nil
}

func lineEndingCode(ending string) byte {
	switch ending {
	case "\r\n":
		return 'W'
	case "\r":
		return 'M'
	default:
		return 'L'
	}
}

func lineEndingsFromLabels(labels []byte) []string {
	breaks := make([]string, len(labels))
	for index, code := range labels {
		breaks[index] = lineEndingFromCode(code)
	}
	return breaks
}

func lineEndingFromCode(code byte) string {
	switch code {
	case 'W':
		return "\r\n"
	case 'M':
		return "\r"
	default:
		return "\n"
	}
}

func uniformLFLabels(labels []byte, code byte) bool {
	for _, label := range labels {
		if label != code {
			return false
		}
	}
	return true
}

func mapLFLineEndings(change text.LFLabelReplacement) []byte {
	breaks := mapLineEndings(change.Removed, change.Inserted, lineEndingsFromLabels(change.Labels), lineEndingFromCode(change.Default))
	labels := make([]byte, len(breaks))
	for index, ending := range breaks {
		labels[index] = lineEndingCode(ending)
	}
	return labels
}

func (doc *Document) LineEndingAt(line int) string {
	if breaks := doc.currentLineEndings(); line >= 0 && line < len(breaks) {
		return breaks[line]
	}
	return doc.newline
}

func (doc *Document) EOLLabels(dst []string) []string {
	dst = dst[:0]
	for _, ending := range doc.currentLineEndings() {
		switch ending {
		case "\r\n":
			dst = append(dst, "CRLF")
		case "\r":
			dst = append(dst, "CR")
		default:
			dst = append(dst, "LF")
		}
	}
	return dst
}

func (doc *Document) EOLSequence() string {
	labels, err := doc.currentLFLabels()
	if err != nil || doc.eolUniform {
		return ""
	}
	return string(labels)
}

func (doc *Document) RestoreEOLSequence(sequence string) error {
	if sequence == "" {
		return fmt.Errorf("recovered EOL sequence does not match text")
	}
	return doc.restoreLineEndings(doc.newline, sequence)
}

// RestoreLineEndings installs recovery metadata on current LF identities.
// Unlike an explicit conversion, it preserves replaced disk identities in Undo.
// An empty sequence means every current LF uses the recovered default format.
func (doc *Document) RestoreLineEndings(ending LineEnding, sequence string) error {
	newline := "\n"
	switch ending {
	case LF:
	case CRLF:
		newline = "\r\n"
	case CR:
		newline = "\r"
	default:
		return fmt.Errorf("invalid recovered line ending %d", ending)
	}
	return doc.restoreLineEndings(newline, sequence)
}

func (doc *Document) restoreLineEndings(newline, sequence string) error {
	current, err := doc.currentLFLabels()
	if err != nil {
		return err
	}
	if sequence != "" && len(sequence) != len(current) {
		return fmt.Errorf("recovered EOL sequence does not match text")
	}
	for _, code := range []byte(sequence) {
		switch code {
		case 'W', 'M', 'L':
		default:
			return fmt.Errorf("invalid recovered EOL code %q", code)
		}
	}
	labels := []byte(sequence)
	if sequence == "" {
		labels = make([]byte, len(current))
		for index := range labels {
			labels[index] = lineEndingCode(newline)
		}
	}
	if !doc.Editor.SetLFLabels(labels, lineEndingCode(newline)) {
		return fmt.Errorf("cannot restore EOL labels on the active source")
	}
	doc.newline, doc.uniformEOL = newline, false
	return nil
}

// TextRangeWithLineEndings returns UTF-8 buffer text with the selected line
// separators restored, without applying the file encoding or adding a BOM.
func (doc *Document) TextRangeWithLineEndings(selection text.Range) (string, error) {
	start, end := min(selection.Start, selection.End), max(selection.Start, selection.End)
	if start < 0 || end > doc.Editor.RuneCount() {
		return "", fmt.Errorf("selection is outside the document")
	}
	value := doc.Editor.String()
	startByte, endByte := doc.Editor.ByteOffset(start), doc.Editor.ByteOffset(end)
	selected := value[startByte:endByte]
	firstBreak := strings.IndexByte(selected, '\n')
	if firstBreak < 0 {
		return selected, nil
	}
	labels, err := doc.currentLFLabels()
	if err != nil {
		return "", err
	}
	if doc.eolUniform {
		return strings.ReplaceAll(selected, "\n", doc.newline), nil
	}
	line := strings.Count(value[:startByte], "\n")
	var physical strings.Builder
	physical.Grow(len(selected))
	for next := firstBreak; next >= 0; next = strings.IndexByte(selected, '\n') {
		if line >= len(labels) {
			return "", fmt.Errorf("document EOL labels do not match selection")
		}
		physical.WriteString(selected[:next])
		physical.WriteString(lineEndingFromCode(labels[line]))
		line++
		selected = selected[next+1:]
	}
	physical.WriteString(selected)
	return physical.String(), nil
}

func (doc *Document) physicalValue(value string) (string, error) {
	labels, err := doc.currentLFLabels()
	if err != nil {
		return "", err
	}
	if len(labels) != strings.Count(value, "\n") {
		return "", fmt.Errorf("document EOL labels do not match text")
	}
	if doc.eolUniform {
		return strings.ReplaceAll(value, "\n", doc.newline), nil
	}
	breaks := lineEndingsFromLabels(labels)
	var physical strings.Builder
	physical.Grow(len(value) + len(breaks))
	line, start := 0, 0
	for index := 0; index < len(value); index++ {
		if value[index] == '\n' {
			if index == start && line > 0 && breaks[line-1] == "\r" && breaks[line] == "\n" {
				return "", fmt.Errorf("adjacent CR and LF would merge; convert line endings before saving")
			}
			physical.WriteString(value[start:index])
			physical.WriteString(breaks[line])
			line++
			start = index + 1
		}
	}
	physical.WriteString(value[start:])
	return physical.String(), nil
}

func mapLineEndings(before, after string, breaks []string, defaultNewline string) []string {
	if before == after {
		return append([]string(nil), breaks...)
	}
	if len(breaks) != strings.Count(before, "\n") {
		return nil
	}
	if len(breaks)+strings.Count(after, "\n") > 256 {
		return mapLineEndingsAnchored(before, after, breaks, defaultNewline)
	}
	edits := myers.ComputeEdits(span.URI("mixed-eol"), before, after)
	result := make([]string, 0, strings.Count(after, "\n"))
	oldLine := 0
	for index := 0; index < len(edits); {
		start := edits[index].Span.Start().Line() - 1
		end := edits[index].Span.End().Line() - 1
		inserted := edits[index].NewText
		index++
		for index < len(edits) && edits[index].Span.Start().Line()-1 <= end {
			end = max(end, edits[index].Span.End().Line()-1)
			inserted += edits[index].NewText
			index++
		}
		result = append(result, breaks[oldLine:min(start, len(breaks))]...)
		deleted := breaks[min(start, len(breaks)):min(end, len(breaks))]
		added := strings.Count(inserted, "\n")
		for line := range added {
			oldIndex := len(deleted) - added + line
			if oldIndex >= 0 {
				result = append(result, deleted[oldIndex])
			} else {
				result = append(result, defaultNewline)
			}
		}
		oldLine = end
	}
	result = append(result, breaks[min(oldLine, len(breaks)):]...)
	return result
}

func mapLineEndingsAnchored(before, after string, breaks []string, defaultNewline string) []string {
	oldLines := strings.SplitAfter(before, "\n")
	newLines := strings.SplitAfter(after, "\n")
	oldPositions := make(map[string]int, len(oldLines))
	oldCounts := make(map[string]int, len(oldLines))
	newCounts := make(map[string]int, len(newLines))
	for index, line := range oldLines {
		oldPositions[line] = index
		oldCounts[line]++
	}
	for _, line := range newLines {
		newCounts[line]++
	}
	result := make([]string, 0, strings.Count(after, "\n"))
	oldStart, newStart := 0, 0
	appendGap := func(oldEnd, newEnd int) {
		oldSegment := strings.Join(oldLines[oldStart:oldEnd], "")
		newSegment := strings.Join(newLines[newStart:newEnd], "")
		oldBreakStart, oldBreakEnd := min(oldStart, len(breaks)), min(oldEnd, len(breaks))
		result = append(result, mapLineEndingsByPosition(oldSegment, newSegment, breaks[oldBreakStart:oldBreakEnd], defaultNewline)...)
	}
	for newIndex, line := range newLines {
		oldIndex, exists := oldPositions[line]
		if !exists || oldCounts[line] != 1 || newCounts[line] != 1 || oldIndex < oldStart {
			continue
		}
		appendGap(oldIndex, newIndex)
		if oldIndex < len(breaks) {
			result = append(result, breaks[oldIndex])
		}
		oldStart, newStart = oldIndex+1, newIndex+1
	}
	appendGap(len(oldLines), len(newLines))
	return result
}

func mapLineEndingsByPosition(before, after string, breaks []string, defaultNewline string) []string {
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(before)-prefix && suffix < len(after)-prefix && before[len(before)-suffix-1] == after[len(after)-suffix-1] {
		suffix++
	}
	oldPrefix := strings.Count(before[:prefix], "\n")
	oldSuffix := strings.Count(before[len(before)-suffix:], "\n")
	newMiddle := strings.Count(after[prefix:len(after)-suffix], "\n")
	result := make([]string, 0, strings.Count(after, "\n"))
	result = append(result, breaks[:oldPrefix]...)
	oldMiddle := breaks[oldPrefix : len(breaks)-oldSuffix]
	for index := range newMiddle {
		if index < len(oldMiddle) {
			result = append(result, oldMiddle[index])
		} else {
			result = append(result, defaultNewline)
		}
	}
	result = append(result, breaks[len(breaks)-oldSuffix:]...)
	return result
}
