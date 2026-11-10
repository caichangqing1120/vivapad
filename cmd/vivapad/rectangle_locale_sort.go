package main

import (
	"runtime"

	"github.com/vivaui/viva/app"
	"golang.org/x/text/language"
	"vivapad/internal/document"
)

func preferredSortLanguage() language.Tag {
	return sortLanguageForPlatform(runtime.GOOS, app.PreferredLanguages)
}

func sortLanguageForPlatform(platform string, preferred func() []string) language.Tag {
	if platform == "windows" {
		return language.Und
	}
	if languages := preferred(); len(languages) != 0 {
		return language.Make(languages[0])
	}
	return language.Und
}

func (state *model) sortLocaleRectangleCommand(doc *document.Document, command string, previous rectangleSelection) bool {
	if previous.composing {
		state.notice = "Finish column composition before sorting"
		return true
	}
	pane := state.activePane
	var after document.Rectangle
	var accepted bool
	var sortError error
	state.editLineTargetsWithBookmarks(doc, "replace", func() {
		after, accepted, sortError = doc.SortRectangleLocale(state.rectangleForPane(pane), preferredSortLanguage(), command == "Sort In Locale Order Ascending")
	})
	if sortError != nil {
		state.notice = sortError.Error()
	} else if accepted {
		value := doc.Editor.String()
		state.rectangles[pane] = rectangleSelection{
			doc: doc, anchor: after.Anchor, caret: after.Caret,
			anchorVirtual: after.AnchorVirtual, caretVirtual: after.CaretVirtual, value: value, active: true,
		}
		state.recordRectangleTransition(pane, doc, previous.value, value, previous)
		state.notice = command
	} else {
		state.notice = "Nothing to sort"
	}
	return true
}
