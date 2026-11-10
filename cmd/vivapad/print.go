package main

import (
	"errors"
	"fmt"

	"vivapad/internal/printing"
)

func (state *model) printCurrent(interactive bool) {
	if state.printJob == nil {
		state.notice = "Printing unavailable"
		return
	}
	doc := state.paneDocument(state.activePane)
	job, err := printing.Prepare(doc, state.languageFor(doc), interactive)
	if err != nil {
		state.notice = "Printing unavailable: " + err.Error()
		return
	}
	job.Settings = state.printSettings
	pages, err := state.printJob(job)
	if errors.Is(err, printing.ErrPrintCanceled) {
		return
	}
	if err != nil {
		state.notice = "Printing failed: " + err.Error()
		return
	}
	state.notice = fmt.Sprintf("Submitted print job (%d pages)", pages)
}
