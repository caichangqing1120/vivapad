package main

import (
	"fmt"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/batchconvert"
)

var (
	batchPanelID         = tree.ID(page, "batch-convert")
	batchRootID          = tree.ID(page, "batch-root")
	batchBrowseID        = tree.ID(page, "batch-browse")
	batchGlobID          = tree.ID(page, "batch-glob")
	batchRecursiveID     = tree.ID(page, "batch-recursive")
	batchListID          = tree.ID(page, "batch-list")
	batchPreviousID      = tree.ID(page, "batch-previous")
	batchNextID          = tree.ID(page, "batch-next")
	batchPreviewID       = tree.ID(page, "batch-preview")
	batchApplyID         = tree.ID(page, "batch-apply")
	batchCloseID         = tree.ID(page, "batch-close")
	batchScopeChoices    = tree.Sub(page, tree.Name("batch-scope"))
	batchEncodingChoices = tree.Sub(page, tree.Name("batch-encoding"))
	batchEOLChoices      = tree.Sub(page, tree.Name("batch-eol"))
	batchRows            = tree.Sub(page, tree.Name("batch-rows"))
)

func (state *model) paintBatchConvert(canvas *gfx.Canvas, size gfx.Size) {
	dialog := state.batchConvert
	dialog.readFields()
	palette := state.palette()
	title := state.preferenceText(batchConvertCommand)
	width, height := min(float32(780), size.Width-20), min(float32(530), size.Height-20)
	panel := gfx.Rect{X: (size.Width - width) / 2, Y: (size.Height - height) / 2, Width: width, Height: height}
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	canvas.PushFocusScope(batchPanelID)
	canvas.TrapFocus(batchPanelID)
	canvas.SetEscapeTarget(batchCloseID)
	canvas.PushLayer(5)
	canvas.Fill(viewport, gfx.RGBA(0, 0, 0, .3))
	canvas.HitWithOptions(dialogBackdropID, viewport, title, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	canvas.PushLayer(6)
	canvas.RoundedBorder(panel, 4, 1, palette.border, palette.popup)
	defer func() {
		canvas.PopLayer()
		canvas.PopLayer()
		canvas.PopFocusScope()
	}()
	inner := panel.Inset(gfx.All(14))
	left, top, right := inner.X, inner.Y, inner.X+inner.Width
	state.interfaceLabel(canvas, state.tr("Batch Convert", "批量转换"), gfx.Rect{X: left, Y: top, Width: inner.Width, Height: 23}, palette.text, 16)
	busy := dialog.job != nil
	selectedScope := 0
	if dialog.inputs.directory {
		selectedScope = 1
	}
	keep := state.tr("Keep unchanged", "保持不变")
	if selected := state.paintBatchChoices(canvas, inner, top+30, state.tr("Scope", "范围"), batchScopeChoices,
		[]string{state.tr("Open Tabs", "打开的标签页"), state.tr("Directory", "目录")}, selectedScope, busy); selected != selectedScope {
		dialog.inputs.directory = selected == 1
		dialog.resetPreview()
	}
	if dialog.focus {
		canvas.RequestFocusOnce(batchScopeChoices.At(0))
		dialog.focus = false
	}
	line := top + 59
	if dialog.inputs.directory {
		state.paintBatchField(canvas, gfx.Rect{X: left, Y: line, Width: inner.Width - 96, Height: 24}, batchRootID,
			state.tr("Directory path", "目录路径"), state.tr("Directory path", "目录路径"), dialog.inputs.root, busy)
		if palette.m5Button(canvas, batchBrowseID, gfx.Rect{X: right - 88, Y: line, Width: 88, Height: 24}, state.tr("Browse…", "选择目录…"), busy) {
			state.browseBatchDirectory()
		}
		line += 29
		state.paintBatchField(canvas, gfx.Rect{X: left, Y: line, Width: inner.Width - 152, Height: 24}, batchGlobID,
			state.tr("File filter (glob)", "文件筛选（通配符）"), state.tr("All files, or e.g. *.txt, *.go", "所有文件，也可输入 *.txt, *.go"), dialog.inputs.glob, busy)
		checkBounds := gfx.Rect{X: right - 144, Y: line - 1, Width: 144, Height: 26}
		if busy {
			state.interfaceLabel(canvas, state.tr("Include subfolders", "包含子目录"), checkBounds, palette.muted, 11)
		} else if state.paintPreferenceCheckbox(canvas, batchRecursiveID, checkBounds, state.tr("Include subfolders", "包含子目录"), dialog.inputs.recursive) {
			dialog.inputs.recursive = !dialog.inputs.recursive
			dialog.resetPreview()
		}
		line += 29
	}
	encodings := []string{keep}
	for _, encoding := range batchEncodings {
		encodings = append(encodings, encoding.Label())
	}
	if selected := state.paintBatchChoices(canvas, inner, line, state.tr("Encoding", "编码"), batchEncodingChoices, encodings, dialog.inputs.encoding, busy); selected != dialog.inputs.encoding {
		dialog.inputs.encoding = selected
		dialog.resetPreview()
	}
	line += 29
	if selected := state.paintBatchChoices(canvas, inner, line, state.tr("Line endings", "换行符"), batchEOLChoices,
		[]string{keep, "LF", "CRLF", "CR"}, int(dialog.inputs.eol), busy); selected != int(dialog.inputs.eol) {
		dialog.inputs.eol = batchconvert.EOL(selected)
		dialog.resetPreview()
	}
	line += 30
	footer := inner.Y + inner.Height - 27
	summary := state.batchCounts(dialog)
	if dialog.preview != nil && dialog.preview.Truncated() {
		summary += state.tr("  ·  Scan limit reached", "  ·  已达扫描上限")
	}
	state.interfaceLabel(canvas, summary, gfx.Rect{X: left, Y: line, Width: inner.Width, Height: 22}, palette.accent, 12)
	line += 24
	state.paintBatchList(canvas, gfx.Rect{X: left, Y: line, Width: inner.Width, Height: max(0, footer-line-47)}, dialog)
	if palette.m5Button(canvas, batchPreviewID, gfx.Rect{X: left, Y: footer, Width: 132, Height: 27}, state.tr("Preview (Dry Run)", "预览（不修改）"), !dialog.canPreview()) {
		state.previewBatchConversion()
	}
	applyColor := *palette
	if dialog.canApply() {
		applyColor.toolbar = palette.selectedRow
	}
	if applyColor.m5Button(canvas, batchApplyID, gfx.Rect{X: right - 209, Y: footer, Width: 132, Height: 27}, state.tr("Apply Conversion", "应用转换"), !dialog.canApply()) {
		state.applyBatchConversion()
	}
	if palette.m5Button(canvas, batchCloseID, gfx.Rect{X: right - 70, Y: footer, Width: 70, Height: 27}, state.tr("Close", "关闭"), busy && dialog.job.applying) {
		state.closeBatchConvertDialog()
	}
}

func (state *model) paintBatchChoices(canvas *gfx.Canvas, area gfx.Rect, y float32, label string, scope gfx.Scope, choices []string, selected int, disabled bool) int {
	palette := state.palette()
	state.interfaceLabel(canvas, label, gfx.Rect{X: area.X, Y: y, Width: 92, Height: 24}, palette.muted, 12)
	width := min(float32(114), (area.Width-96-5*float32(len(choices)-1))/float32(len(choices)))
	for index, label := range choices {
		id := scope.At(index)
		bounds := gfx.Rect{X: area.X + 96 + float32(index)*(width+5), Y: y, Width: width, Height: 24}
		background, color := palette.toolbar, palette.text
		if index == selected {
			background, color = palette.selectedRow, palette.accent
		} else if !disabled && (canvas.Hovered() == id || canvas.FocusVisible() == id) {
			background = palette.hover
		}
		if disabled {
			color = palette.muted
		}
		font := state.interfaceSize(11)
		canvas.Border(bounds, 1, palette.border, background)
		canvas.LabelCenter(canvas.FitLine(label, font, gfx.WeightRegular, bounds.Width-6), bounds, gfx.TextStyle{Size: font, Color: color})
		canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleRadio, Checked: index == selected, Disabled: disabled})
		if !disabled && canvas.Clicked(id) {
			selected = index
		}
	}
	return selected
}

func (state *model) paintBatchField(canvas *gfx.Canvas, bounds gfx.Rect, id gfx.ID, label, placeholder, value string, disabled bool) {
	theme := state.uiTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM = bounds.Height, state.interfaceSize(12)
	focused := canvas.Focused() == id && !disabled
	display := value
	if !focused {
		display = canvas.FitLine(value, theme.Metrics.FontSizeM, gfx.WeightRegular, bounds.Width-24)
	}
	components.PaintInput(canvas, theme, bounds, components.InputSnapshot{ID: id, Label: label, Placeholder: placeholder,
		Value: value, DisplayValue: display, Hint: value, Disabled: disabled, PlatformEditing: focused})
}

func (state *model) paintBatchList(canvas *gfx.Canvas, bounds gfx.Rect, dialog *batchConvertDialog) {
	const rowHeight = 21
	palette := state.palette()
	total := dialog.rowCount()
	dialog.visible = max(1, int((bounds.Height-rowHeight)/rowHeight))
	dialog.offset = min(max(0, dialog.offset), max(0, total-dialog.visible))
	canvas.Border(bounds, 1, palette.border, palette.inputBackground)
	canvas.HitWithOptions(batchListID, bounds, state.tr("Conversion preview", "转换预览"), gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	column := func(row gfx.Rect, number int) gfx.Rect {
		fractions := [...]float32{0, .34, .65, .83, 1}
		left, right := fractions[number]*row.Width, fractions[number+1]*row.Width
		return gfx.Rect{X: row.X + left + 6, Y: row.Y, Width: right - left - 10, Height: row.Height}
	}
	header := gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: rowHeight}
	canvas.Fill(header, palette.toolbar)
	labels := []string{state.tr("File / Tab", "文件 / 标签页"), state.tr("Encoding", "编码"), state.tr("Line endings", "换行符"), state.tr("Action", "操作")}
	if dialog.result != nil {
		labels = []string{labels[0], state.tr("Error details", "错误详情")}
	}
	for index, label := range labels {
		state.interfaceLabel(canvas, label, column(header, index), palette.muted, 11)
	}
	canvas.Clip(bounds)
	for index := dialog.offset; index < min(total, dialog.offset+dialog.visible); index++ {
		row := gfx.Rect{X: bounds.X + 1, Y: bounds.Y + float32(index-dialog.offset+1)*rowHeight, Width: bounds.Width - 2, Height: rowHeight}
		id := batchRows.At(index)
		name, encoding, eol, detail, action := state.batchRow(dialog, index)
		if canvas.Hovered() == id || canvas.FocusVisible() == id || dialog.selected == index {
			canvas.Fill(row, palette.hover)
		}
		values := []string{name, encoding, eol, action}
		for part, value := range values {
			area := column(row, part)
			if dialog.result != nil && part == 1 {
				area.Width = row.X + row.Width - area.X - 5
			}
			state.interfaceLabel(canvas, value, area, palette.text, 11)
		}
		canvas.HitWithOptions(id, row, name+" · "+action, gfx.HitOptions{Role: gfx.RoleButton, Hint: detail})
		if canvas.Clicked(id) {
			dialog.selected = index
		}
	}
	canvas.Unclip()
	bottom := bounds.Y + bounds.Height
	if total != 0 {
		previous := gfx.Rect{X: bounds.X + bounds.Width - 58, Y: bottom + 4, Width: 26, Height: 20}
		next := gfx.Rect{X: previous.X + 29, Y: previous.Y, Width: 26, Height: 20}
		if state.paintBatchPager(canvas, batchPreviousID, previous, upIcon, state.tr("Previous page", "上一页"), dialog.offset == 0) {
			dialog.offset = max(0, dialog.offset-dialog.visible)
		}
		if state.paintBatchPager(canvas, batchNextID, next, downIcon, state.tr("Next page", "下一页"), dialog.offset+dialog.visible >= total) {
			dialog.offset = min(total-dialog.visible, dialog.offset+dialog.visible)
		}
		state.interfaceLabel(canvas, fmt.Sprintf("%d–%d / %d", dialog.offset+1, min(total, dialog.offset+dialog.visible), total),
			gfx.Rect{X: previous.X - 123, Y: bottom + 4, Width: 119, Height: 20}, palette.muted, 11)
	}
	if dialog.err != nil {
		state.interfaceLabel(canvas, dialog.err.Error(), gfx.Rect{X: bounds.X, Y: bottom + 24, Width: bounds.Width, Height: 19}, palette.text, 11)
	} else if dialog.selected >= 0 && dialog.selected < total {
		_, _, _, detail, _ := state.batchRow(dialog, dialog.selected)
		state.interfaceLabel(canvas, detail, gfx.Rect{X: bounds.X, Y: bottom + 24, Width: bounds.Width, Height: 19}, palette.text, 11)
	}
}

func (state *model) paintBatchPager(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, icon gfx.Icon, label string, disabled bool) bool {
	palette := state.palette()
	return palette.toolbarButton(canvas, id, bounds, icon, label, palette.toolbarTint(!disabled), !disabled, false)
}

func (state *model) batchRow(dialog *batchConvertDialog, index int) (name, encoding, eol, detail, action string) {
	if result := dialog.result; result != nil {
		failure := result.Errors[index]
		return failure.Relative, failure.Err.Error(), "", failure.Relative + ": " + failure.Err.Error(), ""
	}
	entry := dialog.entries[index]
	name, detail = entry.Relative, entry.Path
	if entry.HasFormat {
		encoding = entry.SourceEncoding.Label() + " → " + entry.TargetEncoding.Label()
		eol = state.batchEOLLabel(entry.SourceEOL) + " → " + state.batchEOLLabel(entry.TargetEOL)
	}
	switch entry.Action {
	case batchconvert.Convert:
		action = state.tr("Convert", "待转换")
	case batchconvert.Unchanged:
		action = state.tr("Unchanged", "未变化")
	case batchconvert.Skip:
		action = state.tr("Skip", "跳过")
		reason := state.batchSkipReason(entry.Reason)
		if !entry.HasFormat {
			encoding = reason
		}
		if detail != "" {
			detail += ": "
		}
		detail += reason
	}
	return
}

func (state *model) batchEOLLabel(ending batchconvert.EOL) string {
	switch ending {
	case batchconvert.Mixed:
		return state.tr("Mixed", "混合")
	case batchconvert.None:
		return state.tr("None", "无")
	default:
		return ending.String()
	}
}

func (state *model) batchSkipReason(reason string) string {
	switch reason {
	case batchconvert.ReasonVCS:
		return state.tr(reason, "版本控制元数据")
	case batchconvert.ReasonSymlink:
		return state.tr(reason, "符号链接")
	case batchconvert.ReasonSpecial:
		return state.tr(reason, "非常规文件")
	case batchconvert.ReasonTooLarge:
		return state.tr(reason, "超出文件大小上限")
	case batchconvert.ReasonBinary:
		return state.tr(reason, "二进制文件或不支持的编码")
	case batchconvert.ReasonReadOnly:
		return state.tr(reason, "只读文件")
	default:
		return reason
	}
}
