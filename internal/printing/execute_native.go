//go:build darwin || windows || linux

package printing

import "github.com/vivaui/viva/components"

func Execute(job Job, fontSize float64, tabWidth int, variables PageVariables,
	highlight func(string, string, []components.CodeEditorRun) []components.CodeEditorRun) (int, error) {
	if err := job.Settings.Validate(); err != nil {
		return 0, err
	}
	printer, err := ChoosePrinter(job, fontSize, tabWidth)
	if err != nil {
		return 0, err
	}
	defer printer.Close()
	choice := printer.Choice()
	job.Scope = choice.Scope
	device, text := printer.Metrics()
	pages, err := Paginate(job, device, text, variables)
	if err != nil {
		return 0, err
	}
	if len(pages) == 0 {
		return 0, ErrNoPages
	}
	var selected []Page
	if len(choice.Ranges) != 0 {
		selected, err = SelectPageRanges(pages, choice.Ranges)
	} else {
		selected, err = SelectPages(pages, choice.FirstPage, choice.LastPage)
	}
	if err != nil {
		return 0, err
	}
	styled, err := Colorize(job, pages, highlight)
	if err != nil {
		return 0, err
	}
	if err := printer.PrintPages(job, styled); err != nil {
		return 0, err
	}
	return len(selected), nil
}
