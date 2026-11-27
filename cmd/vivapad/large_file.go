package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/largefile"
)

type largeView struct {
	top, rows, loadedTop, loadedRows, loadedLines int
	loadedRevision                                uint64
	loadedBytes                                   int64
	fraction, rowHeight                           float32
	bar, thumb                                    gfx.Rect
	grab                                          float32
	value                                         string
	lines                                         []largefile.Line
	starts                                        []int
	anchor, caret                                 int
	columnBase                                    int
	clipped                                       bool
	targetLine                                    int
	jump                                          bool
	end                                           bool
	hit                                           largefile.Match
	selectHit                                     bool
}

func (state *model) largeView(doc *document.Document, pane int) *largeView {
	views := state.largeViews[doc]
	if views[pane] == nil {
		views[pane] = &largeView{top: 1, rows: 1, columnBase: 1}
		if state.largeViews == nil {
			state.largeViews = make(map[*document.Document][2]*largeView)
		}
		state.largeViews[doc] = views
		doc.LargeFile().SetWake(state.invalidator.Invalidate)
	}
	return views[pane]
}

func (view *largeView) selectedText() string {
	return view.value[min(view.anchor, view.caret):max(view.anchor, view.caret)]
}

func (view *largeView) position() (int, int, int64) {
	if len(view.starts) == 0 || view.loadedTop != view.top {
		return view.top, 1, 0
	}
	index := strings.Count(view.value[:view.caret], "\n")
	start := view.starts[index]
	column := utf8.RuneCountInString(view.value[start:view.caret]) + 1
	if view.lines[index].DisplayOffset > view.lines[index].Offset && view.hit.Line == view.lines[index].Number {
		column += view.columnBase - 1
	}
	return view.lines[index].Number, column, view.lines[index].DisplayOffset + int64(view.caret-start)
}

func (view *largeView) bottom(lines int) int { return max(1, lines-view.rows+1) }

func (view *largeView) move(top int) {
	view.top, view.fraction, view.jump, view.end = top, 0, false, false
	view.selectHit = false
	view.hit = largefile.Match{}
}

func (state *model) jumpLarge(doc *document.Document, line int, hit largefile.Match) {
	view := state.largeView(doc, state.activePane)
	view.targetLine, view.jump, view.end = line, true, false
	view.hit, view.selectHit = hit, hit.Line != 0
	state.invalidator.Invalidate()
}

func (state *model) paintLargePane(canvas *gfx.Canvas, bounds gfx.Rect, pane int, doc *document.Document) {
	file := doc.LargeFile()
	view, progress := state.largeView(doc, pane), file.Progress()
	palette, theme := state.palette(), state.editorTheme()
	font := float32(state.paneFontSize(pane))
	cell, height := canvas.MonoCellSize(font, 0)
	view.rowHeight = max(height, float32(int(font*1.5+0.5)), 1)
	view.rows = min(396, max(1, int((bounds.Height-28)/view.rowHeight)))
	lastTop := view.bottom(progress.Lines)
	if view.end {
		view.top, view.fraction = lastTop, 0
		if progress.Done {
			view.end = false
		}
	}
	if view.jump && (progress.Lines >= view.targetLine || progress.Done || progress.Err != nil) {
		if view.targetLine > progress.Lines {
			state.notice = fmt.Sprintf("Enter a line from 1 to %d", progress.Lines)
		} else if strings.HasPrefix(state.notice, "Indexing to line ") {
			state.notice = ""
		}
		view.targetLine = min(view.targetLine, progress.Lines)
		view.top = min(lastTop, max(1, view.targetLine-view.rows/3))
		view.fraction = 0
	}
	view.top = min(max(1, view.top), lastTop)
	rows := view.rows + 3
	lineCount := min(rows, progress.Lines-view.top+1)
	firstLoaded := view.loadedTop != view.top || view.loadedRows != rows || view.loadedLines != lineCount
	updatedTail := view.loadedRevision != progress.Revision || view.loadedLines+view.loadedTop-1 >= progress.Lines && view.loadedBytes != progress.IndexedBytes
	jumpReady := view.jump && view.targetLine <= progress.Lines
	if firstLoaded || updatedTail || jumpReady {
		lines, err := file.Window(view.top, rows, view.hit.Line, view.hit.Offset)
		if err != nil {
			state.notice = "Large file read failed: " + err.Error()
		} else {
			old, oldTop := view.value, view.loadedTop
			var value strings.Builder
			view.starts = view.starts[:0]
			view.clipped = false
			for index, line := range lines {
				if index != 0 {
					value.WriteByte('\n')
				}
				view.starts = append(view.starts, value.Len())
				value.WriteString(line.Text)
				view.clipped = view.clipped || line.Clipped
			}
			view.lines, view.value = lines, value.String()
			view.loadedTop, view.loadedRows, view.loadedLines = view.top, rows, len(lines)
			view.loadedRevision, view.loadedBytes = progress.Revision, progress.IndexedBytes
			if oldTop != view.top || old != view.value {
				view.anchor, view.caret = 0, 0
			}
		}
	}
	code := state.codeEditorForPane(pane)
	id, barID, horizontalID := editorID, scrollbarID, horizontalBarID
	if pane == 1 {
		id, barID, horizontalID = rightEditorID, rightScrollbarID, rightHorizontalID
	}
	if jumpReady {
		index := view.targetLine - view.loadedTop
		if index >= 0 && index < len(view.lines) {
			view.anchor, view.caret = view.starts[index], view.starts[index]
			view.columnBase = 1
			if view.selectHit {
				line := view.lines[index]
				offset := min(len(line.Text), int(max(0, view.hit.Offset-line.DisplayOffset)))
				end := min(len(line.Text), int(max(0, view.hit.End-line.DisplayOffset)))
				view.anchor, view.caret = view.starts[index]+offset, view.starts[index]+end
				view.columnBase = view.hit.Column - utf8.RuneCountInString(line.Text[:offset])
				code.Reveal(view.anchor)
			} else {
				code.SetScrollX(0)
			}
			view.jump, view.selectHit = false, false
		}
	}
	if view.jump && (progress.Done || progress.Err != nil) && view.targetLine > progress.Lines {
		view.jump = false
	}
	const scrollbarWidth = 12
	const pad = 6
	numberWidth := float32(0)
	if state.showLineNumbers {
		numberWidth = float32(max(4, len(strconv.Itoa(progress.Lines))+1))*cell + pad
	}
	body := gfx.Rect{X: bounds.X + numberWidth, Y: bounds.Y, Width: max(1, bounds.Width-numberWidth-scrollbarWidth), Height: bounds.Height}
	code.SetScrollY(view.fraction)
	caret := view.caret
	if view.anchor != view.caret && view.hit.Line != 0 && view.caret >= view.anchor {
		caret = view.anchor
	}
	code.Paint(canvas, theme, body, components.CodeEditorSnapshot{
		ID: id, Label: "Large File (Read-Only)", Value: view.value, ReadOnly: true,
		ShowCaret: true, Caret: caret, SelectionStart: min(view.anchor, view.caret), SelectionEnd: max(view.anchor, view.caret),
		FontSize: font, TabWidth: state.tabWidth, HorizontalScrollbarID: horizontalID,
	})
	view.fraction = code.ScrollY()
	if numberWidth > 0 {
		gutter := gfx.Rect{X: bounds.X, Y: bounds.Y, Width: numberWidth, Height: bounds.Height}
		canvas.Fill(gutter, palette.gutter)
		canvas.Clip(gfx.Rect{X: gutter.X, Y: gutter.Y + 8, Width: gutter.Width, Height: max(0, gutter.Height-28)})
		for index, line := range view.lines {
			y := bounds.Y + 8 + float32(index)*view.rowHeight - view.fraction
			if y >= bounds.Y+bounds.Height-12 {
				continue
			}
			label := strconv.Itoa(line.Number)
			if line.Clipped {
				label += "*"
			}
			smallLabel(canvas, label, gfx.Rect{X: gutter.X + pad, Y: y, Width: numberWidth - pad, Height: view.rowHeight}, palette.muted, font-1)
		}
		canvas.Unclip()
	}
	view.bar = gfx.Rect{X: bounds.X + bounds.Width - scrollbarWidth, Y: bounds.Y, Width: scrollbarWidth, Height: max(1, bounds.Height-12)}
	thumbHeight := min(view.bar.Height, max(18, view.bar.Height*float32(float64(view.rows)/float64(max(1, progress.Lines)))))
	fraction := float64(view.top-1) / float64(max(1, lastTop-1))
	view.thumb = gfx.Rect{X: view.bar.X + 2, Y: view.bar.Y + float32(fraction*float64(view.bar.Height-thumbHeight)), Width: scrollbarWidth - 4, Height: thumbHeight}
	canvas.Fill(view.bar, palette.gutter)
	canvas.Rounded(view.thumb, 3, palette.muted)
	canvas.HitWithOptions(barID, view.bar, "Large file scrollbar", gfx.HitOptions{Role: gfx.RoleSlider, SkipFocus: true})
}

func (state *model) largePointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	pane := paneForTarget(event.Target)
	if state.largeDrag != 0 {
		pane = state.largeDragPane
	}
	if pane < 0 || pane == 1 && !state.splitEnabled {
		return gfx.EventResult{}, false
	}
	doc := state.paneDocument(pane)
	if doc.LargeFile() == nil || state.largeDrag != 0 && state.largeDragDoc != doc {
		state.largeDrag, state.largeDragDoc = 0, nil
		return gfx.EventResult{}, false
	}
	view := state.largeView(doc, pane)
	code := state.codeEditorForPane(pane)
	if event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonPrimary {
		switch event.Target {
		case editorID, rightEditorID:
			state.activatePane(pane)
			if offset, ok := code.OffsetAt(gfx.Point{X: event.X, Y: event.Y}); ok {
				if event.Modifiers&gfx.ModifierShift == 0 {
					view.anchor = offset
				}
				view.caret = offset
				state.largeDrag = 1
			}
			state.largeDragPane, state.largeDragDoc = pane, doc
			return gfx.EventResult{Handled: true, Repaint: true, Focus: event.Target}, true
		case scrollbarID, rightScrollbarID:
			state.activatePane(pane)
			view.grab = view.thumb.Height / 2
			if event.Y >= view.thumb.Y && event.Y < view.thumb.Y+view.thumb.Height {
				view.grab = event.Y - view.thumb.Y
			}
			state.largeDrag, state.largeDragPane, state.largeDragDoc = 2, pane, doc
		}
	}
	if state.largeDrag != 0 {
		switch state.largeDrag {
		case 1:
			if offset, ok := code.OffsetAt(gfx.Point{X: event.X, Y: event.Y}); ok {
				view.caret = offset
			}
		case 2:
			travel := max(1, view.bar.Height-view.thumb.Height)
			fraction := float64(min(1, max(0, (event.Y-view.grab-view.bar.Y)/travel)))
			view.move(1 + int(math.Round(fraction*float64(view.bottom(doc.LineCount())-1))))
		}
		if event.Kind == gfx.PointerUp {
			state.largeDrag, state.largeDragDoc = 0, nil
		}
		return gfx.EventResult{Handled: true, Repaint: true}, true
	}
	return code.Pointer(event), true
}

func (state *model) scrollLarge(pane int, doc *document.Document, event gfx.ScrollEvent) gfx.EventResult {
	view := state.largeView(doc, pane)
	code := state.codeEditorForPane(pane)
	if event.Modifiers&gfx.ModifierShift != 0 {
		return code.Scroll(event)
	}
	step := event.StepY(max(1, view.rowHeight))
	if step != 0 {
		position := float64(view.top-1) + float64(view.fraction+step)/float64(max(1, view.rowHeight))
		position = min(float64(view.bottom(doc.LineCount())-1), max(0, position))
		if top := int(position) + 1; top != view.top {
			view.move(top)
		}
		view.fraction = float32(position-float64(view.top-1)) * view.rowHeight
	}
	code.ScrollBy(event.StepX(8), 0)
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (state *model) largeKey(doc *document.Document, event gfx.KeyEvent) gfx.EventResult {
	view := state.largeView(doc, state.activePane)
	result := gfx.EventResult{Handled: true, Repaint: true}
	switch event.Key {
	case gfx.KeyPageUp, gfx.KeyPageDown:
		step := max(1, view.rows-1)
		if event.Key == gfx.KeyPageUp {
			step = -step
		}
		view.move(min(view.bottom(doc.LineCount()), max(1, view.top+step)))
	case gfx.KeyHome, gfx.KeyEnd:
		if event.Modifiers&primary() != 0 {
			if event.Key == gfx.KeyHome {
				view.move(1)
				view.anchor, view.caret = 0, 0
			} else {
				view.move(view.bottom(doc.LineCount()))
				view.end = true
			}
			return result
		}
		fallthrough
	case gfx.KeyLeft, gfx.KeyRight, gfx.KeyUp, gfx.KeyDown:
		movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
		if movement == components.TextareaMoveNone {
			return gfx.EventResult{}
		}
		line, _, _ := view.position()
		if !extend && event.Key == gfx.KeyUp && line == view.top && view.top > 1 {
			state.jumpLarge(doc, line-1, largefile.Match{})
			return result
		}
		if !extend && event.Key == gfx.KeyDown && line >= view.top+view.rows-1 && line < doc.LineCount() {
			state.jumpLarge(doc, line+1, largefile.Match{})
			return result
		}
		view.caret = state.codeEditor().Move(view.value, view.caret, movement)
		if !extend {
			view.anchor = view.caret
		}
	default:
		if event.Key != gfx.KeyBackspace && event.Key != gfx.KeyDelete && event.Key != gfx.KeyEnter && event.Key != gfx.KeyTab && event.Text == "" {
			return gfx.EventResult{}
		}
		state.notice = document.ErrLargeFileReadOnly.Error()
	}
	return result
}

func (state *model) copyLarge(doc *document.Document) {
	if selected := state.largeView(doc, state.activePane).selectedText(); selected != "" {
		if err := app.SetClipboardText(strings.ToValidUTF8(selected, "\uFFFD")); err != nil {
			state.notice = err.Error()
		}
	}
}

func (state *model) largeStatus(doc *document.Document) string {
	progress := doc.LargeFile().Progress()
	line, column, _ := state.largeView(doc, state.activePane).position()
	total := strconv.Itoa(progress.Lines)
	if !progress.Done {
		total = "?"
	}
	return fmt.Sprintf("Large File (Read-Only)   %.1f MiB   Lines indexed: %d / %s   Ln: %d  Col: %d",
		float64(progress.Size)/(1<<20), progress.Lines, total, line, column)
}
