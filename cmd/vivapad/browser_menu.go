package main

import (
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/external"
)

const namedBrowserMenuCommand = "View Current File in"

var viewNamedBrowsers = [...]external.NamedBrowser{external.Firefox, external.Chrome, external.Edge, external.InternetExplorer}

func (state *model) openCurrentFileInNamedBrowser(browser external.NamedBrowser) {
	state.menuOpen, state.namedBrowserMenuOpen = -1, false
	state.requestEditorCommandFocus()
	doc := state.paneDocument(state.activePane)
	path := doc.Path
	if path == "" {
		state.notice = "Save the document before opening it in a browser"
		return
	}
	opening := "Opening saved file in " + string(browser)
	state.notice = opening
	state.namedBrowserRequest++
	request := state.namedBrowserRequest
	finish := func(err error) {
		if state.namedBrowserRequest != request || state.notice != opening {
			return
		}
		if err != nil {
			state.notice = string(browser) + ": " + err.Error()
		} else {
			state.notice = "Open request sent to " + string(browser)
		}
		state.invalidator.Invalidate()
	}
	open := state.openNamedBrowser
	if post := state.postBrowserResult; post != nil {
		go func() {
			err := open(browser, path)
			post(func() { finish(err) })
		}()
		return
	}
	finish(open(browser, path))
}

func (state *model) paintNamedBrowserMenu(canvas *gfx.Canvas, size gfx.Size, parentRight, parentY float32) {
	palette := state.palette()
	const rowHeight = 27
	width := min(float32(190), size.Width-8)
	height := float32(len(viewNamedBrowsers)*rowHeight + 8)
	x := min(parentRight, size.Width-width-4)
	y := max(menuHeight, min(parentY, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	for index, browser := range viewNamedBrowsers {
		bounds := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*rowHeight, Width: width - 8, Height: rowHeight}
		id := namedBrowserScope.At(index)
		available := external.NamedBrowserSupported(browser)
		color := palette.muted
		if available {
			color = palette.text
			if canvas.Hovered() == id {
				canvas.Fill(bounds, palette.hover)
			}
		}
		label := canvas.FitLine(string(browser), 11, gfx.WeightRegular, bounds.Width-18)
		smallLabel(canvas, label, bounds.Inset(gfx.All(6)), color, 11)
		canvas.HitWithOptions(id, bounds, string(browser), gfx.HitOptions{Role: gfx.RoleMenuItem, Disabled: !available})
		if available && canvas.Clicked(id) {
			state.openCurrentFileInNamedBrowser(browser)
		}
	}
	canvas.PopLayer()
}
