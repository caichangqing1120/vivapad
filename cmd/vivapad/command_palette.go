package main

import (
	"context"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/foldertree"
	"vivapad/internal/shortcuts"
)

var (
	palettePanelID   = tree.ID(page, "command-palette")
	paletteInputID   = tree.ID(page, "command-palette-input")
	paletteDismissID = tree.ID(page, "command-palette-dismiss")
	paletteRowScope  = tree.Sub(page, tree.Name("command-palette-rows"))
)

var paletteCommands = [...]struct{ command, english, chinese string }{
	{"Preferences...", "Preferences", "偏好设置"},
	{shortcutMapperCommand, "Shortcut Mapper", "快捷键管理"},
	{styleConfiguratorCommand, "Style Configurator", "配色与样式配置"},
	{udlManagerCommand, "User Defined Language (UDL) Manager", "用户自定义语言管理"},
	{batchConvertCommand, "Batch Convert Encoding / Line Endings", "批量转换编码与换行符"},
	{compareSplitCommand, "Compare with Other View (Active Tabs)", "对比两个视图的活动标签页"},
	{compareSavedCommand, "Compare with Saved File on Disk", "与磁盘上的已保存文件对比"},
	{"Format JSON (2 Spaces)", "Format JSON (2 Spaces)", "格式化 JSON（2 个空格）"},
	{"Format JSON (4 Spaces)", "Format JSON (4 Spaces)", "格式化 JSON（4 个空格）"},
	{"Format XML", "Format XML", "格式化 XML"},
	{"Word Wrap", "Toggle Word Wrap", "切换自动换行"},
	{"Show Line Numbers", "Toggle Line Numbers", "切换行号"},
	{"Split View", "Toggle Split View", "切换分屏视图"},
	{"Theme: Classic Light", "Switch Theme: Classic Light", "切换主题：经典浅色"},
	{"Theme: Modern Dark", "Switch Theme: Modern Dark", "切换主题：现代深色"},
	{"Theme: Monokai", "Switch Theme: Monokai", "切换主题：Monokai"},
	{"Theme: Follow System", "Switch Theme: Follow System", "切换主题：跟随系统"},
	{checkUpdatesCommand, "Check for Updates", "检查更新"},
	{"Save", "Save", "保存"},
	{"New", "New Document", "新建文档"},
	{"Open...", "Open File", "打开文件"},
	{"Find...", "Find in Document", "在文档中查找"},
	{"Go to Line...", "Go to Line", "跳转到行"},
	{"Playback Macro", "Playback Macro", "回放宏"},
	{shortcuts.QuickSwitcher, "Quick File Switcher", "快速切换文件"},
}

type paletteEntry struct {
	command, label, alternate, detail, path string
	doc                                     *document.Document
	position, score                         int
}

type commandPalette struct {
	mode             string
	input            *text.Editor
	returnFocus      gfx.ID
	requestFocus     bool
	entries, matches []paletteEntry
	query            string
	selected, offset int
	visible          int
	needsRefresh     bool
	navigated        bool
	index            <-chan []paletteEntry
	cancel           context.CancelFunc
}

func (state *model) openCommandPalette(mode string, returnFocus gfx.ID) {
	state.closeCommandPalette()
	state.cancelLiveComposition()
	state.menuOpen, state.menuOverflowOpen = -1, false
	state.statusPopup = statusPopup{}
	state.wordCompletion = nil
	state.focusViewRequested, state.focusViewArmed = false, false
	palette := &commandPalette{
		mode: mode, input: text.NewEditor(""), returnFocus: returnFocus, requestFocus: true, needsRefresh: true,
	}
	state.commandPalette = palette
	if mode == shortcuts.CommandPalette {
		for _, command := range paletteCommands {
			palette.entries = append(palette.entries, paletteEntry{command: command.command,
				label: state.tr(command.english, command.chinese), alternate: state.tr(command.chinese, command.english)})
		}
	} else {
		seen := make(map[string]bool)
		for _, doc := range state.space.Documents {
			palette.entries = append(palette.entries, paletteEntry{label: doc.Name, detail: doc.Path, doc: doc})
			if doc.Path != "" {
				seen[doc.Path] = true
			}
		}
		for _, project := range state.projects.groups {
			for _, path := range project.Files {
				if !seen[path] {
					seen[path] = true
					palette.entries = append(palette.entries, paletteEntry{label: filepath.Base(path), detail: path, path: path})
				}
			}
		}
		if len(state.folderRoots) != 0 {
			ctx, cancel := context.WithCancel(context.Background())
			index := make(chan []paletteEntry, 1)
			palette.index, palette.cancel = index, cancel
			roots, wake := slices.Clone(state.folderRoots), state.invalidator.Invalidate
			go func() {
				indexPaletteWorkspace(ctx, roots, seen, index, wake)
				close(index)
				wake()
			}()
		}
	}
	state.invalidator.Invalidate()
}

func indexPaletteWorkspace(ctx context.Context, roots []string, seen map[string]bool, index chan<- []paletteEntry, wake func()) {
	const batchSize = 256
	entries := make([]paletteEntry, 0, batchSize)
	publish := func() bool {
		if len(entries) == 0 {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case index <- entries:
			entries = make([]paletteEntry, 0, batchSize)
			wake()
			return true
		}
	}
	for _, root := range roots {
		if ctx.Err() != nil {
			return
		}
		if foldertree.CheckCanonicalPath(root) != nil {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return fs.SkipAll
			}
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if path != root && strings.HasPrefix(entry.Name(), ".") {
					return fs.SkipDir
				}
			} else if entry.Type().IsRegular() && !seen[path] {
				seen[path] = true
				relative, _ := filepath.Rel(root, path)
				detail := filepath.ToSlash(filepath.Join(filepath.Base(root), relative))
				entries = append(entries, paletteEntry{label: entry.Name(), detail: detail, path: path})
				if len(entries) == batchSize && !publish() {
					return fs.SkipAll
				}
			}
			return nil
		})
		if !publish() {
			return
		}
	}
}

func (state *model) consumePaletteIndex() {
	palette := state.commandPalette
	for palette != nil && palette.index != nil {
		select {
		case entries, ok := <-palette.index:
			if !ok {
				palette.index = nil
				return
			}
			palette.entries = append(palette.entries, entries...)
			palette.needsRefresh = true
		default:
			return
		}
	}
}

func (state *model) closeCommandPalette() {
	if palette := state.commandPalette; palette != nil {
		if palette.cancel != nil {
			palette.cancel()
		}
		state.commandPalette = nil
	}
}

func paletteMatch(query []rune, value string) int {
	if len(query) == 0 {
		return 1
	}
	if value == "" {
		return 0
	}
	index, last, matched, score := 0, -1, 0, 0
	boundary := true
	for _, char := range strings.ToLower(value) {
		if char == query[matched] {
			score += 8
			if boundary {
				score += 6
			}
			if last >= 0 {
				score += max(0, 5-(index-last))
			} else {
				score += max(0, 8-index)
			}
			last, matched = index, matched+1
			if matched == len(query) {
				return score
			}
		}
		boundary = strings.ContainsRune(" /\\-_.:(", char)
		index++
	}
	return 0
}

func (palette *commandPalette) refresh() {
	query := strings.ToLower(strings.Join(strings.Fields(palette.input.String()), ""))
	if palette.mode == shortcuts.QuickSwitcher {
		query = strings.ReplaceAll(query, `\`, "/")
	}
	if !palette.needsRefresh && query == palette.query {
		return
	}
	if query != palette.query {
		palette.offset, palette.navigated = 0, false
	}
	selectedPosition := -1
	if palette.navigated && palette.selected < len(palette.matches) {
		selectedPosition = palette.matches[palette.selected].position
	}
	palette.query, palette.needsRefresh = query, false
	needle := []rune(query)
	palette.matches = palette.matches[:0]
	for index, entry := range palette.entries {
		entry.position = index
		entry.score = max(paletteMatch(needle, entry.label), paletteMatch(needle, entry.alternate))
		if len(needle) > 0 {
			entry.score = max(entry.score, paletteMatch(needle, filepath.ToSlash(entry.detail))-5)
		}
		if entry.score > 0 {
			palette.matches = append(palette.matches, entry)
		}
	}
	if len(needle) > 0 {
		slices.SortFunc(palette.matches, func(first, second paletteEntry) int {
			if first.score != second.score {
				return second.score - first.score
			}
			return first.position - second.position
		})
	}
	palette.selected = 0
	if selectedPosition >= 0 {
		for index, entry := range palette.matches {
			if entry.position == selectedPosition {
				palette.selected = index
				break
			}
		}
	}
}

func (state *model) paletteKey(event gfx.KeyEvent) gfx.EventResult {
	result := gfx.EventResult{Handled: true, Repaint: true}
	if event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}
	}
	palette := state.commandPalette
	palette.refresh()
	switch event.Key {
	case gfx.KeyEscape:
		state.closeCommandPalette()
		result.Focus = palette.returnFocus
	case gfx.KeyUp, gfx.KeyDown:
		if count := len(palette.matches); count > 0 {
			step := 1
			if event.Key == gfx.KeyUp {
				step = -1
			}
			palette.selected = (palette.selected + step + count) % count
			palette.navigated = true
		}
	case gfx.KeyEnter:
		if len(palette.matches) != 0 {
			result.Focus = state.executePaletteEntry(palette.matches[palette.selected])
		}
	default:
		return gfx.EventResult{}
	}
	return result
}

func (state *model) executePaletteEntry(entry paletteEntry) gfx.ID {
	state.closeCommandPalette()
	switch {
	case entry.doc != nil:
		if index := state.documentIndex(entry.doc); index >= 0 {
			state.space.Select(index)
			state.activateOpenedDocument(entry.doc)
		}
	case entry.path != "":
		if err := foldertree.CheckCanonicalPath(entry.path); err != nil {
			state.notice = err.Error()
		} else {
			state.openDocumentPaths([]string{entry.path})
		}
	case entry.command == shortcuts.QuickSwitcher:
		state.openCommandPalette(entry.command, state.commandEditorFocusID())
		return gfx.ID{}
	default:
		state.command(entry.command)
		switch entry.command {
		case "Find...":
			return queryID
		case "Go to Line...":
			return goToFieldID
		}
	}
	if state.editorCommandFocusBlocked() {
		return gfx.ID{}
	}
	return state.commandEditorFocusID()
}

func (state *model) paintCommandPalette(canvas *gfx.Canvas, size gfx.Size) {
	palette := state.commandPalette
	if palette == nil || size.Width < 70 || size.Height < 145 {
		return
	}
	state.consumePaletteIndex()
	palette.refresh()
	colors := state.palette()
	const rowHeight = 30
	width := min(float32(540), size.Width-32)
	palette.visible = min(8, max(1, int((size.Height-128)/rowHeight)))
	if palette.selected < palette.offset {
		palette.offset = palette.selected
	} else if palette.selected >= palette.offset+palette.visible {
		palette.offset = palette.selected - palette.visible + 1
	}
	palette.offset = min(palette.offset, max(0, len(palette.matches)-palette.visible))
	height := float32(82 + rowHeight*palette.visible)
	x, y := (size.Width-width)/2, min(float32(96), max(12, (size.Height-height)/3))
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	title, placeholder := state.tr("Command Palette", "命令面板"), state.tr("Search commands…", "搜索命令…")
	if palette.mode == shortcuts.QuickSwitcher {
		title, placeholder = state.tr("Quick File Switcher", "快速切换文件"), state.tr("Search tabs and workspace files…", "搜索标签页和工作区文件…")
	}
	canvas.PushFocusScope(palettePanelID)
	canvas.TrapFocus(palettePanelID)
	canvas.PushLayer(5)
	canvas.Fill(viewport, gfx.RGBA(0, 0, 0, .22))
	canvas.HitWithOptions(paletteDismissID, viewport, title, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	canvas.PushLayer(6)
	canvas.Rounded(gfx.Rect{X: x - 2, Y: y + 4, Width: width + 4, Height: height + 3}, 14, gfx.RGBA(0, 0, 0, .14))
	canvas.RoundedBorder(panel, 12, 1, colors.border, colors.popup)
	canvas.HitWithOptions(palettePanelID, panel, title, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	state.interfaceLabel(canvas, title, gfx.Rect{X: x + 14, Y: y + 7, Width: width - 28, Height: 23}, colors.muted, 12)
	theme := state.uiTheme()
	theme.Metrics.RadiusControl, theme.Metrics.FontSizeM = 6, 13
	components.PaintInput(canvas, theme, gfx.Rect{X: x + 12, Y: y + 34, Width: width - 24, Height: 34}, components.InputSnapshot{
		ID: paletteInputID, Label: placeholder, Placeholder: placeholder, Value: palette.input.String(), NavigationKeys: true,
		PlatformEditing: canvas.Focused() == paletteInputID,
	})
	if palette.requestFocus {
		canvas.RequestFocusOnce(paletteInputID)
		palette.requestFocus = false
	}
	chosen := -1
	for index := palette.offset; index < min(len(palette.matches), palette.offset+palette.visible); index++ {
		entry := palette.matches[index]
		row := gfx.Rect{X: x + 8, Y: y + 74 + float32(index-palette.offset)*rowHeight, Width: width - 16, Height: rowHeight}
		id := paletteRowScope.At(entry.position)
		if index == palette.selected {
			canvas.Rounded(row, 6, colors.selectedRow)
			canvas.Rounded(gfx.Rect{X: row.X + 2, Y: row.Y + 7, Width: 2, Height: row.Height - 14}, 1, colors.accent)
		} else if canvas.Hovered() == id {
			canvas.Rounded(row, 6, colors.hover)
		}
		label := gfx.Rect{X: row.X + 12, Y: row.Y, Width: row.Width - 24, Height: row.Height}
		if entry.command != "" {
			if binding := state.shortcuts.Binding(entry.command); !binding.Empty() && label.Width >= 150 {
				badge := formatShortcut(binding)
				badgeWidth := canvas.TextWidth(badge, 11) + 12
				badgeBounds := gfx.Rect{X: label.X + label.Width - badgeWidth, Y: row.Y + 5, Width: badgeWidth, Height: 20}
				canvas.RoundedBorder(badgeBounds, 4, 1, colors.chromeLine, colors.popup)
				smallLabel(canvas, badge, gfx.Rect{X: badgeBounds.X + 6, Y: badgeBounds.Y, Width: badgeWidth - 10, Height: 20}, colors.muted, 11)
				label.Width -= badgeWidth + 12
			}
		} else if entry.detail != "" && label.Width >= 195 {
			detailWidth := label.Width * .46
			state.interfaceLabel(canvas, filepath.ToSlash(entry.detail), gfx.Rect{X: label.X + label.Width - detailWidth, Y: label.Y, Width: detailWidth, Height: label.Height}, colors.muted, 11)
			label.Width -= detailWidth + 12
		}
		state.interfaceLabel(canvas, entry.label, label, colors.text, 13)
		canvas.HitWithOptions(id, row, entry.label, gfx.HitOptions{Role: gfx.RoleOption, Hint: entry.detail, Selected: index == palette.selected, SkipFocus: true})
		if canvas.Clicked(id) {
			chosen = index
		}
	}
	if len(palette.matches) == 0 {
		message := state.tr("No matches", "没有匹配项")
		if palette.index != nil {
			message = state.tr("Scanning workspace…", "正在扫描工作区…")
		}
		state.interfaceLabel(canvas, message, gfx.Rect{X: x + 18, Y: y + 78, Width: width - 36, Height: 30}, colors.muted, 12)
	}
	canvas.PopLayer()
	canvas.PopLayer()
	canvas.PopFocusScope()
	if chosen >= 0 {
		if focus := state.executePaletteEntry(palette.matches[chosen]); !focus.IsZero() {
			canvas.RequestFocusOnce(focus)
		}
		state.invalidator.Invalidate()
	} else if canvas.Clicked(paletteDismissID) {
		state.closeCommandPalette()
		canvas.RequestFocusOnce(palette.returnFocus)
		state.invalidator.Invalidate()
	}
}
