package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/texttools"
)

const (
	structuredToolsMenu = "JSON / XML"
	encodingToolsMenu   = "Encode / Convert"
	timestampCommand    = "Convert Timestamp"
)

type textTransform func(string) (string, error)

type textTool struct {
	command   string
	transform textTransform
}

var structuredTools = []textTool{
	{"Format JSON (2 Spaces)", func(src string) (string, error) { return texttools.FormatJSON(src, "  ") }},
	{"Format JSON (4 Spaces)", func(src string) (string, error) { return texttools.FormatJSON(src, "    ") }},
	{"Minify JSON", texttools.MinifyJSON},
	{"Sort JSON Keys", func(src string) (string, error) { return texttools.SortJSONKeys(src, "  ") }},
	{"Format XML", func(src string) (string, error) { return texttools.FormatXML(src, "  ") }},
}

var encodingTools = []textTool{
	{"Base64 Encode", plainTextTransform(texttools.Base64Encode)}, {"Base64 Decode", texttools.Base64Decode},
	{"URL Encode", plainTextTransform(texttools.URLEncode)}, {"URL Decode", texttools.URLDecode},
	{"HTML Escape", plainTextTransform(texttools.HTMLEscape)}, {"HTML Unescape", plainTextTransform(texttools.HTMLUnescape)},
	{timestampCommand, func(src string) (string, error) { return texttools.ConvertTimestamp(src, nil) }},
}

var toolLineEndings = strings.NewReplacer("\r\n", "\n", "\r", "\n")

func plainTextTransform(transform func(string) string) textTransform {
	return func(src string) (string, error) { return transform(src), nil }
}

func findTextTool(command string) (textTool, bool) {
	for _, tool := range structuredTools {
		if tool.command == command {
			return tool, true
		}
	}
	for _, tool := range encodingTools {
		if tool.command == command {
			return tool, false
		}
	}
	return textTool{}, false
}

func (state *model) applyTextTool(doc *document.Document, tool textTool, wholeDocument bool) {
	result, err := state.runTextTool(doc, tool, wholeDocument)
	if err != nil {
		state.notice = err.Error()
		return
	}
	state.notice = result
}

func (state *model) runTextTool(doc *document.Document, tool textTool, wholeDocument bool) (string, error) {
	if doc.Lightweight() || int64(len(doc.Editor.String())) >= document.LightweightThreshold {
		return "", fmt.Errorf("%s is unavailable in large documents", tool.command)
	}
	if state.hasMultipleStreamSelections(doc) || state.rectangles[state.activePane].active && state.rectangles[state.activePane].doc == doc {
		return "", fmt.Errorf("%s requires one continuous selection", tool.command)
	}
	editor := doc.Editor
	value := editor.String()
	selection := editor.Selection()
	start, end := editor.ByteOffset(selection.Start), editor.ByteOffset(selection.End)
	fallback := start == end
	if fallback {
		if wholeDocument {
			start, end = 0, len(value)
		} else {
			start = strings.LastIndexByte(value[:start], '\n') + 1
			if next := strings.IndexByte(value[end:], '\n'); next < 0 {
				end = len(value)
			} else {
				end += next
			}
		}
	}
	caret := editor.ByteOffset(editor.Caret())
	if fallback && tool.command == timestampCommand {
		start += len(value[start:end]) - len(strings.TrimLeftFunc(value[start:end], unicode.IsSpace))
		end = start + len(strings.TrimRightFunc(value[start:end], unicode.IsSpace))
	}
	if !wholeDocument && start == end {
		return "", fmt.Errorf("select text or place the caret on a non-empty line")
	}
	src := value[start:end]
	if tool.command == "Base64 Encode" || tool.command == "URL Encode" {
		physical, err := doc.TextRangeWithLineEndings(text.Range{Start: editor.RuneIndex(start), End: editor.RuneIndex(end)})
		if err != nil {
			return "", err
		}
		src = physical
	}
	converted, err := tool.transform(src)
	if err != nil && fallback && tool.command == timestampCommand {
		if tokenStart, tokenEnd, ok := timestampAtCaret(value, start, end, caret); ok {
			start, end = tokenStart, tokenEnd
			converted, err = tool.transform(value[start:end])
		}
	}
	if err != nil {
		return "", err
	}
	if !utf8.ValidString(converted) || strings.ContainsRune(converted, 0) {
		return "", fmt.Errorf("conversion result is binary or not valid UTF-8; the document was not changed")
	}
	converted = toolLineEndings.Replace(converted)
	if converted == value[start:end] {
		return "No changes", nil
	}
	if int64(len(value)-(end-start)+len(converted)) >= document.LightweightThreshold {
		return "", fmt.Errorf("conversion result exceeds the editable tool size limit")
	}
	batch, err := doc.PrepareTargetEdits([]document.ReplacementEdit{{Start: start, End: end, Text: converted}})
	if err != nil {
		return "", err
	}
	if err := batch.Apply(); err != nil {
		return "", err
	}
	return tool.command, nil
}

func timestampAtCaret(value string, start, end, caret int) (int, int, bool) {
	if caret < start || caret > end {
		return 0, 0, false
	}
	left, right := caret, caret
	for left > start && asciiDigit(value[left-1]) {
		left--
	}
	for right < end && asciiDigit(value[right]) {
		right++
	}
	if right-left != 10 && right-left != 13 {
		return 0, 0, false
	}
	if left > start && (value[left-1] == '-' || value[left-1] == '+') {
		left--
	}
	before, _ := utf8.DecodeLastRuneInString(value[start:left])
	after, _ := utf8.DecodeRuneInString(value[right:end])
	return left, right, !timestampWordRune(before) && !timestampWordRune(after)
}

func asciiDigit(value byte) bool { return value >= '0' && value <= '9' }

func timestampWordRune(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsDigit(value) || value == '_'
}
