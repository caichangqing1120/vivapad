package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vivaui/viva/components"
	"vivapad/internal/document"
	"vivapad/internal/session"
	"vivapad/internal/syntax"
)

type foldCache struct {
	value   string
	mode    string
	regions []syntax.FoldRegion
}

func (state *model) foldRegions(doc *document.Document) []syntax.FoldRegion {
	value, mode := doc.Editor.String(), state.languageFor(doc)
	if cached, ok := state.foldCache[doc]; ok && cached.value == value && cached.mode == mode {
		return cached.regions
	}
	delete(state.folded, doc)
	regions := syntax.FoldRegions(mode, value)
	state.foldCache[doc] = foldCache{value: value, mode: mode, regions: regions}
	return regions
}

func (state *model) foldRanges(doc *document.Document) []components.CodeEditorFoldRange {
	var ranges []components.CodeEditorFoldRange
	lastEnd := -1
	for _, region := range state.foldRegions(doc) {
		if state.folded[doc][region.Header] && region.Header > lastEnd {
			ranges = append(ranges, components.CodeEditorFoldRange{Header: region.Header, LastLine: region.LastLine})
			lastEnd = region.LastLine
		}
	}
	return ranges
}

func (state *model) foldHeaders(doc *document.Document) []int {
	var headers []int
	for _, region := range state.foldRegions(doc) {
		if state.folded[doc][region.Header] {
			headers = append(headers, region.Header)
		}
	}
	return headers
}

func (state *model) applyFoldHeaders(doc *document.Document, headers []int) error {
	regions := state.foldRegions(doc)
	valid := make(map[int]bool, len(regions))
	for _, region := range regions {
		valid[region.Header] = true
	}
	for _, line := range headers {
		if !valid[line] {
			return fmt.Errorf("fold header line %d is unavailable", line)
		}
	}
	if len(headers) != 0 {
		if state.folded[doc] == nil {
			state.folded[doc] = make(map[int]bool, len(headers))
		}
		for _, line := range headers {
			state.folded[doc][line] = true
		}
		state.moveCaretOutOfFold(doc)
	}
	return nil
}

func (state *model) restoreSessionFolds(flags map[string]session.FoldState) {
	for _, doc := range state.space.Documents {
		fold, saved := flags[doc.Path]
		if !saved {
			continue
		}
		if fold.Digest != doc.DiskDigest() {
			state.notice = "Fold state skipped for changed file: " + doc.Name
			continue
		}
		if err := state.applyFoldHeaders(doc, fold.Headers); err != nil {
			state.notice = "Fold state skipped: " + err.Error()
		}
	}
}

func (state *model) foldLine(doc *document.Document) int {
	return foldLineAt(doc, doc.Editor.Caret())
}

func foldLineAt(doc *document.Document, index int) int {
	value := doc.Editor.String()
	return strings.Count(value[:doc.Editor.ByteOffset(index)], "\n")
}

func (state *model) moveCaretOutOfFold(doc *document.Document) {
	line := state.foldLine(doc)
	for _, fold := range state.foldRanges(doc) {
		if line > fold.Header && line <= fold.LastLine {
			doc.GoToLine(fold.Header + 1)
			return
		}
	}
}

func (state *model) revealHiddenCaret(doc *document.Document) {
	selection := doc.Editor.Selection()
	first, last := state.foldLine(doc), state.foldLine(doc)
	if selection.Start != selection.End {
		first = foldLineAt(doc, selection.Start)
		last = foldLineAt(doc, selection.End-1)
	}
	for _, fold := range state.foldRanges(doc) {
		if first <= fold.LastLine && last > fold.Header {
			delete(state.folded[doc], fold.Header)
		}
	}
}

func (state *model) toggleFold(doc *document.Document, header int) {
	for _, region := range state.foldRegions(doc) {
		if region.Header != header {
			continue
		}
		if state.folded[doc] == nil {
			state.folded[doc] = make(map[int]bool)
		}
		if state.folded[doc][header] {
			delete(state.folded[doc], header)
		} else {
			state.folded[doc][header] = true
			state.moveCaretOutOfFold(doc)
		}
		state.invalidator.Invalidate()
		return
	}
}

func (state *model) foldCommand(doc *document.Document, command string) {
	regions := state.foldRegions(doc)
	if len(regions) == 0 {
		state.notice = "No foldable regions"
		return
	}
	if state.folded[doc] == nil {
		state.folded[doc] = make(map[int]bool)
	}
	switch command {
	case "Fold All":
		for _, region := range regions {
			state.folded[doc][region.Header] = true
		}
	case "Unfold All":
		clear(state.folded[doc])
	case "Fold Current Level", "Unfold Current Level":
		line := state.foldLine(doc)
		var selected *syntax.FoldRegion
		for index := range regions {
			region := &regions[index]
			if region.Header <= line && line <= region.LastLine && (selected == nil || region.Level > selected.Level) {
				selected = region
			}
		}
		if selected == nil {
			state.notice = "No fold at caret"
			return
		}
		if command == "Fold Current Level" {
			state.folded[doc][selected.Header] = true
		} else {
			delete(state.folded[doc], selected.Header)
		}
	default:
		level, err := strconv.Atoi(command[len(command)-1:])
		if err != nil || level < 1 || level > 8 {
			return
		}
		fold := strings.HasPrefix(command, "Fold Level ")
		for _, region := range regions {
			if region.Level == level {
				if fold {
					state.folded[doc][region.Header] = true
				} else {
					delete(state.folded[doc], region.Header)
				}
			}
		}
	}
	state.moveCaretOutOfFold(doc)
	state.notice = command
}

func foldLevelForCommand(command string) bool {
	return strings.HasPrefix(command, "Fold Level ") || strings.HasPrefix(command, "Unfold Level ")
}
