package main

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

func (state *model) documentMapWidth(width float32) float32 {
	if !state.documentMapVisible {
		return 0
	}
	return min(160, max(110, width*0.16))
}

func documentMapViewport(bounds gfx.Rect, scroll, maxScroll, viewHeight float32) gfx.Rect {
	result := gfx.Rect{X: bounds.X + 1, Y: bounds.Y, Width: max(0, bounds.Width-2), Height: bounds.Height}
	if bounds.Height <= 0 || maxScroll <= 0 || viewHeight <= 0 {
		return result
	}
	result.Height = min(bounds.Height, max(18, bounds.Height*viewHeight/(maxScroll+viewHeight)))
	result.Y += min(max(0, scroll/maxScroll), 1) * (bounds.Height - result.Height)
	return result
}

func documentMapPreview(value string, offset int) string {
	if offset < 0 || offset >= len(value) {
		return ""
	}
	preview := value[offset:min(len(value), offset+256)]
	if newline := strings.IndexByte(preview, '\n'); newline >= 0 {
		preview = preview[:newline]
	}
	return preview
}

func (state *model) scrollFromDocumentMap(y float32) {
	content := state.documentMapContent
	editor := state.codeEditor()
	maxScroll := editor.MaxScrollY()
	if content.Height <= 0 || maxScroll <= 0 {
		return
	}
	viewport := documentMapViewport(content, editor.ScrollY(), maxScroll, state.documentMapViewHeight)
	track := content.Height - viewport.Height
	if track <= 0 {
		return
	}
	position := min(max(0, (y-content.Y-viewport.Height/2)/track), 1)
	editor.SetScrollY(position * maxScroll)
	state.invalidator.Invalidate()
}

func (state *model) paintDocumentMap(canvas *gfx.Canvas, bounds gfx.Rect, editorHeight float32) {
	canvas.Fill(bounds, gfx.RGB(247, 248, 249))
	canvas.Fill(gfx.Rect{X: bounds.X, Y: bounds.Y, Width: 1, Height: bounds.Height}, palette.border)
	canvas.Fill(gfx.Rect{X: bounds.X, Y: bounds.Y + 30, Width: bounds.Width, Height: 1}, palette.border)
	title := "Document Map"
	if bounds.Width < 130 {
		title = "Map"
	}
	smallLabel(canvas, title, gfx.Rect{X: bounds.X + 10, Y: bounds.Y + 3, Width: bounds.Width - 42, Height: 26}, palette.text, 11)
	closeBounds := gfx.Rect{X: bounds.X + bounds.Width - 26, Y: bounds.Y + 6, Width: 18, Height: 18}
	if canvas.Hovered() == documentMapCloseID {
		canvas.Fill(closeBounds, palette.hover)
	}
	canvas.DrawIcon(closeIcon, closeBounds.Inset(gfx.All(3)), palette.muted)
	if hitButton(canvas, documentMapCloseID, closeBounds, "Close Document Map") {
		state.documentMapVisible = false
		state.documentMapDragging = false
	}
	content := gfx.Rect{X: bounds.X + 6, Y: bounds.Y + 35, Width: bounds.Width - 12, Height: max(0, bounds.Height-41)}
	state.documentMapContent, state.documentMapViewHeight = content, editorHeight
	editor := state.codeEditor()
	doc := state.space.Current()
	if state.splitEnabled {
		doc = state.paneDocument(state.activePane)
	}
	value := doc.Editor.String()
	canvas.Clip(content)
	viewport := documentMapViewport(content, editor.ScrollY(), editor.MaxScrollY(), editorHeight)
	canvas.RoundedBorder(viewport, 2, 1, palette.blue, gfx.RGB(255, 246, 247))
	if content.Height > 0 && len(value) != 0 {
		samples := max(1, int(content.Height/3))
		step := content.Height / float32(samples)
		visualHeight := editor.MaxScrollY() + editorHeight
		previousOffset := -1
		for index := 0; index < samples; index++ {
			offset, ok := editor.OffsetAtY(float32(index) / float32(samples) * visualHeight)
			if !ok || offset >= len(value) {
				break
			}
			if offset == previousOffset {
				continue
			}
			previousOffset = offset
			line := documentMapPreview(value, offset)
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := 0
			for _, character := range line {
				if character != ' ' && character != '\t' {
					break
				}
				indent++
			}
			x := content.X + 4 + min(float32(indent)*0.8, 18)
			width := min(max(3, float32(utf8.RuneCountInString(line))*0.85), max(3, content.X+content.Width-x-4))
			if doc.RTL {
				x = content.X + content.Width - (x - content.X) - width
			}
			canvas.Fill(gfx.Rect{X: x, Y: content.Y + float32(index)*step + 1, Width: width, Height: 1.5}, gfx.RGB(158, 165, 172))
		}
	}
	canvas.Unclip()
	canvas.HitWithOptions(documentMapID, content, "Scroll Document Map", gfx.HitOptions{Role: gfx.RoleButton, Hint: "Scroll the active editor view"})
}
