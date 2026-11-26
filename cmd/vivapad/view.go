package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/syntax"
)

var palette = struct {
	chrome, toolbar, tabs, tab, border, hover, text, muted, gutter, blue, white gfx.Color
	changeReverted, changeSaved, changeModified, changeRevertedModified         gfx.Color
}{
	chrome: gfx.RGB(248, 248, 249), toolbar: gfx.RGB(253, 253, 253),
	tabs: gfx.RGB(238, 239, 241), tab: gfx.RGB(230, 232, 235),
	border: gfx.RGB(207, 210, 214), hover: gfx.RGB(252, 236, 239),
	text: gfx.RGB(39, 42, 47), muted: gfx.RGB(112, 116, 123),
	gutter: gfx.RGB(249, 249, 249), blue: gfx.RGB(215, 38, 61), white: gfx.RGB(255, 255, 255),
	// Change margin states, following the original family's palette.
	changeReverted: gfx.RGB(0x40, 0xa0, 0xbf), changeSaved: gfx.RGB(0x00, 0xa0, 0x00),
	changeModified: gfx.RGB(0xff, 0x80, 0x00), changeRevertedModified: gfx.RGB(0xa0, 0xc0, 0x00),
}

var menuLabels = [...]string{"File", "Edit", "Search", "View", "Encoding", "Language", "Settings", "Tools", "Macro", "Run", "Plugins", "Window", "?"}

var menuCommands = [...][]string{
	{"New", "Open...", "Save", "Save As...", "Save a Copy As...", "Save All", "Close", "Open as GB18030...", "Open as Windows-1252...", "Reload from Disk", "Recent Files", "Close All", "Close Other Tabs", "Sessions", "Rename...", "Close Multiple Documents", "Open Folder as Workspace...", "Open in Default Viewer", "Open Containing Folder", "Open as Encoding...", "Move to Recycle Bin", "Print...", "Print Now", "Exit"},
	{"Undo", "Redo", "Cut", "Copy", "Paste", "Select All", "Duplicate Line", "Delete Line", "Indent Lines", "Unindent Lines", "Line Operations", "Convert Case", "Blank Operations", "Copy to Clipboard", "Delete", "Begin/End Select", "Read-Only in Notepad++", fileReadOnlyLabel(), "Auto-Completion", "Comment/Uncomment", "Insert", "EOL Conversion", "Column Editor...", "Begin/End Select in Column Mode", columnModeHelpCommand, "On Selection", "Multi-select All", "Multi-select Next", "Undo Latest Multi-Select", "Skip Current Multi-Select", pasteSpecialCommand, characterPanelCommand},
	{
		"Find...", "Find in Files...", "Find Next", "Find Previous", "Selected Text Search",
		"Replace...", "Incremental Search", "Search Results Window", "Next Search Result", "Previous Search Result",
		"Go to Line...", "Go to Matching Brace", "Select All In-between {} [] or ()", "Mark...", "Change History",
		"Style All Occurrences of Token", "Style One Token", "Clear Style", "Jump Up", "Jump Down", "Copy Styled Text",
		"Bookmark", "Find characters in range...",
		"Replace in Files...", "Find All in Current Document", "Find All in Open Documents", "Replace All in Open Documents",
		"Clear Marks", "Next Change", "Previous Change", "Clear Change History",
	},
	{"Show Line Numbers", "Word Wrap", "Zoom In", "Zoom Out", "Restore Default Zoom", "Split View", "Move to Other View", "Clone to Other View", "Folder as Workspace", "Show Symbol", "Tab", "Fold All", "Unfold All", "Fold Current Level", "Unfold Current Level", "Fold Level", "Unfold Level", "Document Map", "Function List", "Document List", "Synchronize Vertical Scrolling", "Synchronize Horizontal Scrolling", textDirectionRTL, textDirectionLTR, "Summary...", "Monitoring (tail -f)", "Focus on Another View", "Hide Lines", "Always on Top", "Toggle Full Screen Mode", "Synchronize Zoom Across Views", distractionFreeCommand, namedBrowserMenuCommand},
	{"Unix (LF)", "Windows (CR LF)", "UTF-8", "UTF-8 BOM", "UTF-16 LE", "UTF-16 BE", "GB18030", "Windows-1252", "Character sets"},
	{"Normal Text", "Go", "JavaScript", "TypeScript", "Python", "JSON", "HTML", "CSS", "Markdown", "SQL"},
	{"Preferences...", "Tab Width: 2", "Tab Width: 4", "Tab Width: 8", "Indent: Language Default", "Indent: Tabs", "Indent: Spaces", "Auto Indent", "Pair Brackets", "Date Time Format...", "Change History: Margin", "Change History: Indicator", "Change History: Both", "Change History: Off"},
	{"MD5", "SHA-1", "SHA-256", "SHA-512"},
	{"Record Macro", "Stop Recording", "Playback Macro", "Save Current Recorded Macro...", "Run a Macro Multiple Times...", "Playback 5 Times", "Clear Macro", "Saved Macros"},
	{"Run..."},
	{},
	{"Sort By", "Windows...", "Recent Window"},
	{aboutHelpCommand, debugHelpCommand, projectPageHelpCommand, userGuideHelpCommand, commandLineHelpCommand},
}

func fileMenuCount() int {
	for index, command := range menuCommands[0] {
		if command == "" {
			return index
		}
	}
	return len(menuCommands[0])
}

const (
	menuHeight        float32 = 26
	fileMenuRowHeight float32 = 24
	toolbarHeight     float32 = 34
	tabHeight         float32 = 28
	statusHeight      float32 = 24
	searchHeight      float32 = 94
	goToHeight        float32 = 42
)

func resultPaneHeight(windowHeight float32) float32 {
	return min(180, max(120, windowHeight*0.35))
}

func (state *model) paint(canvas *gfx.Canvas, size gfx.Size) {
	state.consumeSystemDocuments()
	window := gfx.Rect{Width: size.Width, Height: size.Height}
	canvas.Fill(window, palette.white)
	if !state.fullscreen {
		state.paintMenuBar(canvas, size.Width)
		state.paintToolbar(canvas, size.Width)
	}
	if !state.distractionFree {
		state.paintTabs(canvas, size.Width)
	}
	if state.fullscreen {
		bounds := gfx.Rect{X: size.Width - 30, Width: 30, Height: tabHeight}
		canvas.Border(bounds, 1, palette.border, palette.tabs)
		canvas.DrawIcon(fullscreenRestoreIcon, bounds.Inset(gfx.All(6)), palette.text)
		label := "Exit full screen"
		if state.distractionFree {
			label = "Exit Distraction Free Mode"
		}
		canvas.HitWithOptions(fullscreenRestoreID, bounds, label, gfx.HitOptions{Role: gfx.RoleButton, Hint: "Restore window"})
		if canvas.Clicked(fullscreenRestoreID) {
			state.command(state.fullscreenExitCommand())
		}
	}

	bottom := size.Height - statusHeight
	if state.distractionFree {
		bottom = size.Height
	}
	showResults := !state.distractionFree && state.resultsOpen && !state.searchOpen && !state.goToOpen && !state.incrementOpen
	if showResults {
		bottom -= resultPaneHeight(size.Height)
	}
	if state.searchOpen {
		bottom -= searchHeight
	} else if state.goToOpen {
		bottom -= goToHeight
	} else if state.incrementOpen {
		bottom -= state.incrementalHeight(size.Width)
	}
	if state.incrementOpen {
		current := state.space.Current()
		if state.incrementDocument != current || state.incrementValue != current.Editor.String() {
			state.findIncremental(false, false)
		}
	}
	panelWidth := state.folderPanelWidth(size.Width)
	mapWidth := state.documentMapWidth(size.Width)
	top := menuHeight + toolbarHeight + tabHeight
	if state.fullscreen {
		top = tabHeight
	}
	if state.distractionFree {
		panelWidth, mapWidth, top = 0, 0, 0
	}
	editorBounds := gfx.Rect{X: panelWidth, Y: top, Width: size.Width - panelWidth - mapWidth, Height: max(64, bottom-top)}
	if panelWidth > 0 {
		leftBounds := gfx.Rect{Y: editorBounds.Y, Width: panelWidth, Height: editorBounds.Height}
		if count := state.leftDockCount(); count > 1 {
			state.paintLeftDockTabs(canvas, gfx.Rect{Y: leftBounds.Y, Width: leftBounds.Width, Height: 28})
			leftBounds.Y, leftBounds.Height = leftBounds.Y+28, leftBounds.Height-28
		}
		switch state.activeDock() {
		case "functions":
			state.paintFunctionPanel(canvas, leftBounds)
		case "documents":
			state.paintDocumentList(canvas, leftBounds)
		case "characters":
			state.paintCharacterPanel(canvas, leftBounds)
		default:
			state.paintFolderPanel(canvas, leftBounds)
		}
	}
	if state.distractionFree {
		padding := editorBounds.Width / 4
		editorBounds.X, editorBounds.Width = padding, editorBounds.Width-2*padding
		state.paintPane(canvas, editorBounds, state.activePane, state.paneDocument(state.activePane))
	} else {
		state.paintEditor(canvas, editorBounds)
	}
	if !state.distractionFree && state.documentMapVisible {
		state.paintDocumentMap(canvas, gfx.Rect{X: size.Width - mapWidth, Y: editorBounds.Y, Width: mapWidth, Height: editorBounds.Height}, editorBounds.Height)
	}
	if state.searchOpen {
		state.paintSearch(canvas, size.Width, bottom)
	} else if state.goToOpen {
		state.paintGoTo(canvas, size.Width, bottom)
	} else if state.incrementOpen {
		state.paintIncrementalSearch(canvas, size.Width, bottom)
	}
	if showResults {
		state.paintFileResults(canvas, size.Width, size.Height-statusHeight-resultPaneHeight(size.Height), resultPaneHeight(size.Height))
	}
	if !state.distractionFree {
		state.paintStatus(canvas, size.Width, size.Height-statusHeight)
	}
	if !state.fullscreen {
		state.paintOpenMenu(canvas, size)
	}
	if state.menuOverflowOpen {
		state.paintOverflowMenu(canvas, size)
	}
	if state.provisional.doc == nil || state.wordCompletion == nil || state.wordCompletion.doc != state.provisional.doc {
		state.paintWordCompletionPopup(canvas, size)
	}
	state.paintCalltip(canvas, size)
	state.checkMacroSource()
	if state.markDialogOpen {
		state.paintMarkDialog(canvas, size)
	}
	if state.filesDialogOpen {
		state.paintFilesDialog(canvas, size)
	}
	if state.replaceConfirmOpen {
		state.paintReplaceConfirm(canvas, size)
	}
	if state.pendingClose >= 0 {
		state.paintCloseDialog(canvas, size)
	}
	if state.pendingReload >= 0 {
		state.paintReloadDialog(canvas, size)
	}
	if state.renameDoc != nil || state.renamePlan != nil {
		state.paintRenameDialog(canvas, size)
	}
	if state.pendingTrash != nil {
		state.paintTrashDialog(canvas, size)
	}
	if state.dateTimeDialogOpen {
		state.paintDateTimeDialog(canvas, size)
	}
	if state.macroSaveOpen || state.macroRunOpen {
		state.paintMacroDialog(canvas, size)
	}
	if state.hashDialogOpen {
		state.paintHashDialog(canvas, size)
	}
	if state.windowsDialogOpen {
		state.paintWindowsDialog(canvas, size)
	}
	if state.summaryOpen {
		state.paintSummary(canvas, size)
	}
	if state.helpMode != noHelp {
		state.paintHelp(canvas, size)
	}
	if state.webSearchOpen {
		state.paintWebSearchSettings(canvas, size)
	}
	if state.runDialogOpen {
		state.paintRunDialog(canvas, size)
	}
	if state.preferencesOpen {
		state.paintPreferencesDialog(canvas, size)
	}
	if state.columnDialogOpen {
		state.paintColumnDialog(canvas, size)
	}
	if state.byteRangeOpen {
		state.paintByteRangeDialog(canvas, size)
	}
	if state.saveSessionOptionsOpen {
		state.paintSaveSessionOptions(canvas, size)
	}
	if state.resultFocusRequested {
		if state.resultFocusArmed {
			focus := resultsPaneID
			if state.resultReturnToEditor {
				focus = editorID
				if state.splitEnabled && state.activePane == 1 {
					focus = rightEditorID
				}
			}
			canvas.RequestFocus(focus)
			state.resultFocusRequested, state.resultFocusArmed, state.resultReturnToEditor = false, false, false
		} else {
			state.resultFocusArmed = true
			state.invalidator.Invalidate()
		}
	}
	if state.incrementReturnFocusRequested {
		if state.incrementReturnFocusArmed {
			focus := editorID
			if state.splitEnabled && state.activePane == 1 {
				focus = rightEditorID
			}
			canvas.RequestFocus(focus)
			state.incrementReturnFocusRequested, state.incrementReturnFocusArmed = false, false
		} else {
			state.incrementReturnFocusArmed = true
			state.invalidator.Invalidate()
		}
	}
	state.paintEditorCommandFocus(canvas)
	state.snapshotRecovery()
}

func smallLabel(canvas *gfx.Canvas, value string, bounds gfx.Rect, color gfx.Color, size float32) {
	canvas.Label(value, bounds, gfx.TextStyle{Size: size, Color: color})
}

func hitButton(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, label string) bool {
	canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleButton})
	return canvas.Clicked(id)
}

func (state *model) paintMenuBar(canvas *gfx.Canvas, width float32) {
	canvas.Fill(gfx.Rect{Width: width, Height: menuHeight}, palette.chrome)
	canvas.Fill(gfx.Rect{Y: menuHeight - 1, Width: width, Height: 1}, palette.border)
	allWidth := float32(8)
	for _, label := range menuLabels {
		allWidth += canvas.TextWidth(label, 11) + 19
	}
	reserve := float32(0)
	if allWidth > width-4 {
		reserve = 30
	}
	x := float32(8)
	state.menuFirstHidden = len(menuLabels)
	for index, label := range menuLabels {
		itemWidth := canvas.TextWidth(label, 11) + 19
		if x+itemWidth > width-reserve-4 {
			state.menuFirstHidden = index
			break
		}
		item := gfx.Rect{X: x, Y: 1, Width: itemWidth, Height: 24}
		enabled := index < len(menuCommands) && len(menuCommands[index]) > 0 && menuCommands[index][0] != ""
		if state.menuOpen == index || canvas.Hovered() == menuScope.At(index) && enabled {
			canvas.Fill(item, palette.hover)
		}
		color := palette.text
		if !enabled {
			color = palette.muted
		}
		smallLabel(canvas, label, gfx.Rect{X: x + 9, Y: 1, Width: itemWidth - 11, Height: 24}, color, 11)
		if enabled {
			canvas.HitWithOptions(menuScope.At(index), item, label, gfx.HitOptions{Role: gfx.RoleButton})
			if canvas.Clicked(menuScope.At(index)) {
				state.pasteSpecialOpen = false
				state.recentOpen, state.lineMenuOpen, state.caseMenuOpen, state.sessionMenuOpen, state.bookmarkMenuOpen, state.changeHistoryMenuOpen, state.closeMultipleMenuOpen, state.containingMenuOpen, state.autoMenuOpen, state.selectedSearchMenuOpen, state.blankMenuOpen, state.copyPathsMenuOpen, state.readOnlyMenuOpen, state.onSelectionMenuOpen, state.multiAllMenuOpen, state.multiNextMenuOpen = false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false
				state.markJumpMenuOpen = false
				state.styleMenuMode = ""
				state.insertDateOpen = false
				state.hashMenuOpen = false
				state.symbolMenuOpen = false
				state.viewTabMenuOpen = false
				state.namedBrowserMenuOpen = false
				state.foldMenuOpen = false
				state.windowSortMenuOpen = false
				state.menuOverflowOpen = false
				state.commentMenuOpen = false
				state.eolMenuOpen = false
				state.charsetMenuOpen = false
				state.savedMacroMenuOpen = false
				if state.menuOpen == index {
					state.menuOpen = -1
				} else {
					state.menuOpen = index
					if index == 1 {
						state.editMenuOffset = 0
					}
					if index == 2 {
						state.searchMenuOffset = 0
					}
					if index == 3 {
						state.viewMenuOffset = 0
					}
					if index == 11 {
						state.windowRecentOffset = 0
					}
				}
			}
		}
		x += itemWidth
	}
	if reserve != 0 {
		button := gfx.Rect{X: width - 28, Y: 1, Width: 25, Height: 24}
		if state.menuOverflowOpen || canvas.Hovered() == menuOverflowID {
			canvas.Fill(button, palette.hover)
		}
		canvas.DrawIcon(rightIcon, button.Inset(gfx.All(5)), palette.text)
		canvas.HitWithOptions(menuOverflowID, button, "More menus", gfx.HitOptions{Role: gfx.RoleButton, Hint: "Show hidden menus"})
		if canvas.Clicked(menuOverflowID) {
			state.menuOpen, state.menuOverflowOpen = -1, !state.menuOverflowOpen
			state.windowSortMenuOpen = false
		}
	} else {
		state.menuOverflowOpen = false
	}
}

func toolbarButton(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, icon gfx.Icon, name string, tint gfx.Color, enabled, selected bool) bool {
	if selected {
		canvas.Rounded(bounds, 5.5, gfx.RGB(221, 243, 237))
	}
	if enabled && canvas.Hovered() == id {
		canvas.Rounded(bounds, 5.5, gfx.RGB(232, 236, 238))
	}
	if enabled && canvas.Pressed() == id {
		canvas.Rounded(bounds, 5.5, gfx.RGB(220, 226, 229))
	}
	if enabled && canvas.FocusVisible() == id {
		canvas.RoundedBorder(bounds, 5.5, 1.5, gfx.RGB(9, 131, 110), gfx.RGBA(0, 0, 0, 0))
	}
	iconBounds := controlIconBounds(bounds)
	canvas.DrawIcon(icon, iconBounds, tint)
	canvas.HitWithOptions(id, bounds, name, gfx.HitOptions{Role: gfx.RoleButton, Disabled: !enabled, Selected: selected, Hint: name})
	return enabled && canvas.Clicked(id)
}

func toolbarTint(id gfx.ID, canUndo, canRedo bool) gfx.Color {
	switch {
	case id == undoID && !canUndo || id == redoID && !canRedo:
		return gfx.RGB(166, 175, 182)
	default:
		return gfx.RGB(52, 61, 67)
	}
}

func (state *model) paintToolbar(canvas *gfx.Canvas, width float32) {
	y := menuHeight
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: toolbarHeight}, gfx.RGB(251, 252, 252))
	canvas.Fill(gfx.Rect{Y: y + toolbarHeight - 1, Width: width, Height: 1}, palette.border)
	x := float32(8)
	button := func(id gfx.ID, icon gfx.Icon, label string) bool {
		doc := state.space.Current()
		canUndo, canRedo := doc.Editor.CanUndo(), doc.Editor.CanRedo()
		tint := toolbarTint(id, canUndo, canRedo)
		enabled := !(id == undoID && !canUndo || id == redoID && !canRedo)
		selected := id == searchID && state.searchOpen
		if selected {
			tint = gfx.RGB(8, 105, 87)
		}
		clicked := toolbarButton(canvas, id, gfx.Rect{X: x, Y: y + 4, Width: 26, Height: 26}, icon, label, tint, enabled, selected)
		x += 30
		return clicked
	}
	if button(newID, newIcon, "New") {
		state.cancelMacroForCommand()
		state.showInActivePane(state.space.NewDocument())
	}
	if button(openID, openIcon, "Open") {
		state.cancelMacroForCommand()
		state.openFiles()
	}
	if button(saveID, saveIcon, "Save") {
		state.cancelMacroForCommand()
		state.save(state.space.Active, false)
	}
	separator := func() {
		canvas.Fill(gfx.Rect{X: x + 4, Y: y + 10, Width: 1, Height: 14}, gfx.RGB(125, 138, 145))
		x += 13
	}
	separator()
	if button(undoID, undoIcon, "Undo") {
		state.command("Undo")
	}
	if button(redoID, redoIcon, "Redo") {
		state.command("Redo")
	}
	separator()
	if button(cutID, cutIcon, "Cut") {
		state.command("Cut")
	}
	if button(copyID, copyIcon, "Copy") {
		state.command("Copy")
	}
	if button(pasteID, pasteIcon, "Paste") {
		state.command("Paste")
	}
	separator()
	if button(searchID, searchIcon, "Find") {
		state.cancelMacroForCommand()
		state.closeIncrementalSearch()
		state.markDialogOpen = false
		state.searchOpen, state.goToOpen = !state.searchOpen, false
	}
}

func (state *model) paintTabs(canvas *gfx.Canvas, width float32) {
	if state.splitEnabled {
		origin := state.folderPanelWidth(width)
		state.paintSplitTabs(canvas, width-origin, origin)
		return
	}
	y := menuHeight + toolbarHeight
	if state.fullscreen {
		y = 0
	}
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: tabHeight}, palette.tabs)
	canvas.Fill(gfx.Rect{Y: y + tabHeight - 1, Width: width, Height: 1}, palette.border)
	contentWidth := width
	if state.fullscreen {
		contentWidth -= 30
	}
	visible := max(1, int((contentWidth-10)/161))
	overflow := len(state.space.Documents) > visible
	if overflow {
		visible = max(1, int((contentWidth-58)/161))
	}
	if state.tabActive != state.space.Active || state.tabVisible != visible {
		if state.space.Active < state.tabOffset {
			state.tabOffset = state.space.Active
		} else if state.space.Active >= state.tabOffset+visible {
			state.tabOffset = state.space.Active - visible + 1
		}
		state.tabActive = state.space.Active
		state.tabVisible = visible
	}
	state.tabOffset = min(max(0, state.tabOffset), max(0, len(state.space.Documents)-visible))
	start := state.tabOffset
	selected, closed, pinning := -1, -1, -1
	for index := start; index < len(state.space.Documents) && index-start < visible; index++ {
		doc := state.space.Documents[index]
		x := 7 + float32(index-start)*161
		rect := gfx.Rect{X: x, Y: y + 2, Width: 157, Height: 26}
		background, accent := state.tabColorPaint(doc, index == state.space.Active)
		canvas.Border(rect, 1, palette.border, background)
		if index == state.space.Active {
			canvas.Fill(gfx.Rect{X: x + 1, Y: y + 1, Width: 155, Height: 2}, accent)
		}
		name := doc.Name
		if doc.Dirty() {
			name = "* " + name
		}
		name = canvas.FitLine(name, 11, gfx.WeightRegular, 103)
		smallLabel(canvas, name, gfx.Rect{X: x + 10, Y: y + 2, Width: 103, Height: 25}, palette.text, 11)
		canvas.HitWithOptions(tabScope.At(index), rect, doc.Name, gfx.HitOptions{Role: gfx.RoleTab, Selected: index == state.space.Active})
		if canvas.Clicked(tabScope.At(index)) {
			selected = index
		}
		pinRect := gfx.Rect{X: x + 113, Y: y + 5, Width: 17, Height: 18}
		if canvas.Hovered() == tabPinScope.At(index) {
			canvas.Fill(pinRect, palette.hover)
		}
		pinColor, pinLabel := palette.muted, "Pin "+doc.Name
		if state.isPinned(doc) {
			pinColor, pinLabel = gfx.RGB(8, 105, 87), "Unpin "+doc.Name
		}
		canvas.DrawIcon(pinIcon, controlIconBounds(pinRect), pinColor)
		if hitButton(canvas, tabPinScope.At(index), pinRect, pinLabel) {
			pinning = index
		}
		closeRect := gfx.Rect{X: x + 135, Y: y + 5, Width: 17, Height: 18}
		if canvas.Hovered() == tabCloseScope.At(index) {
			canvas.Fill(closeRect, palette.hover)
		}
		canvas.DrawIcon(closeIcon, controlIconBounds(closeRect), palette.muted)
		if hitButton(canvas, tabCloseScope.At(index), closeRect, "Close "+doc.Name) {
			closed = index
		}
	}
	if selected >= 0 {
		state.space.Select(selected)
		state.showInActivePane(state.space.Current())
		state.focusEditorOnTabClick(canvas)
	}
	if closed >= 0 {
		state.closeTab(closed)
	}
	if pinning >= 0 {
		state.togglePinned(state.space.Documents[pinning], 0)
	}
	if overflow {
		left := gfx.Rect{X: contentWidth - 49, Y: y + 3, Width: 22, Height: 22}
		right := gfx.Rect{X: contentWidth - 25, Y: y + 3, Width: 22, Height: 22}
		canvas.Border(left, 1, palette.border, palette.toolbar)
		canvas.Border(right, 1, palette.border, palette.toolbar)
		canvas.DrawIcon(leftIcon, controlIconBounds(left), palette.text)
		canvas.DrawIcon(rightIcon, controlIconBounds(right), palette.text)
		if hitButton(canvas, tabLeftID, left, "Previous tabs") {
			state.tabOffset = max(0, state.tabOffset-1)
		}
		if hitButton(canvas, tabRightID, right, "Next tabs") {
			state.tabOffset = min(len(state.space.Documents)-visible, state.tabOffset+1)
		}
	}
}

func (state *model) paintEditor(canvas *gfx.Canvas, bounds gfx.Rect) {
	state.splitWidth = bounds.Width
	if !state.splitEnabled {
		state.paintPane(canvas, bounds, 0, state.space.Current())
		return
	}
	position := state.splitPosition(bounds.Width)
	left := gfx.Rect{X: bounds.X, Y: bounds.Y, Width: position, Height: bounds.Height}
	right := gfx.Rect{X: bounds.X + position + 8, Y: bounds.Y, Width: bounds.Width - position - 8, Height: bounds.Height}
	state.paintPane(canvas, left, 0, state.paneDocument(0))
	divider := gfx.Rect{X: bounds.X + position, Y: bounds.Y, Width: 8, Height: bounds.Height}
	canvas.Fill(divider, palette.tabs)
	canvas.Fill(gfx.Rect{X: divider.X + 3, Y: divider.Y, Width: 1, Height: divider.Height}, palette.border)
	canvas.HitWithOptions(splitterID, divider, "Resize editor views", gfx.HitOptions{Role: gfx.RoleSlider, Cursor: gfx.PointerCursorResizeHorizontal})
	state.paintPane(canvas, right, 1, state.paneDocument(1))
	state.syncScrollAfterPaint()
}

func (state *model) paintPane(canvas *gfx.Canvas, bounds gfx.Rect, pane int, doc *document.Document) {
	if state.isReadOnly(doc) {
		state.cancelDocumentComposition(doc)
	}
	theme := gfx.LightTheme()
	theme.Background, theme.Card, theme.Foreground = palette.white, palette.white, palette.text
	theme.Border, theme.Ring, theme.Input, theme.Primary = palette.white, palette.white, palette.white, palette.white
	fontSize := state.paneFontSize(pane)
	theme.Metrics.RadiusControl, theme.Metrics.FontSizeM = 0, float32(fontSize)
	value := doc.Editor.String()
	// A live preedit is not committed source. Painting must not settle the
	// edit-tracked indicator groups (fold regions, hidden lines, Find Mark
	// spans, user styles) from the transient final text; the terminal event
	// settles them exactly once on a later paint.
	provisional := state.provisional.doc == doc
	regions, hidden := state.foldCache[doc].regions, state.hiddenLines[doc]
	if !provisional {
		regions, hidden = state.foldRegions(doc), state.hiddenRanges(doc)
		state.revealHiddenCaret(doc)
		state.syncFindMarks(doc)
	}
	selection := doc.Editor.Selection()
	composition, composing := doc.Editor.Composition()
	id, barID, horizontalID := editorID, scrollbarID, horizontalBarID
	if pane == 1 {
		id, barID, horizontalID = rightEditorID, rightScrollbarID, rightHorizontalID
	}
	snapshot := components.CodeEditorSnapshot{
		ID: id, Label: "Text editor", Value: value,
		ShapedBidi: true, Direction: editorDirection(doc),
		ReadOnly:       state.isReadOnly(doc),
		MarkerGutter:   true,
		ShowCaret:      selection.Start == selection.End,
		Caret:          doc.Editor.ByteOffset(doc.Editor.Caret()),
		SelectionStart: doc.Editor.ByteOffset(selection.Start), SelectionEnd: doc.Editor.ByteOffset(selection.End),
		FontSize: float32(fontSize), TabWidth: state.tabWidth, Wrap: state.wrap, LineNumbers: state.showLineNumbers,
		CurrentLine: true, MatchBrackets: true,
		ShowWhitespace: state.showWhitespace, ShowEOL: state.showEOL, ShowIndentGuides: state.showIndentGuides, ShowWrapSymbol: state.showWrapSymbol, InvisibleColor: symbolColor,
		Representations: state.codeRepresentations(),
		ScrollbarID:     barID, HorizontalScrollbarID: horizontalID,
		Language: state.languageFor(doc), Highlight: state.syntax.Highlight,
	}
	if snapshot.Language == ".nfo" {
		snapshot.MonoFace, snapshot.Highlight = gfx.MonoClassic, nil
	}
	if strings.EqualFold(snapshot.Language, ".json5") || strings.EqualFold(snapshot.Language, ".jsonc") {
		highlighter := state.json5Highlighters[doc]
		if highlighter == nil {
			if state.json5Highlighters == nil {
				state.json5Highlighters = make(map[*document.Document]*syntax.JSON5Highlighter)
			}
			highlighter = syntax.NewJSON5Highlighter(&state.syntax)
			state.json5Highlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.json5Highlighters, doc)
	}
	if strings.EqualFold(snapshot.Language, ".iss") {
		highlighter := state.innoHighlighters[doc]
		if highlighter == nil {
			if state.innoHighlighters == nil {
				state.innoHighlighters = make(map[*document.Document]*syntax.InnoHighlighter)
			}
			highlighter = syntax.NewInnoHighlighter()
			state.innoHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.innoHighlighters, doc)
	}
	if strings.EqualFold(snapshot.Language, ".mib") {
		highlighter := state.asn1Highlighters[doc]
		if highlighter == nil {
			if state.asn1Highlighters == nil {
				state.asn1Highlighters = make(map[*document.Document]*syntax.ASN1Highlighter)
			}
			highlighter = syntax.NewASN1Highlighter()
			state.asn1Highlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.asn1Highlighters, doc)
	}
	if strings.EqualFold(snapshot.Language, ".kix") {
		highlighter := state.kixHighlighters[doc]
		if highlighter == nil {
			if state.kixHighlighters == nil {
				state.kixHighlighters = make(map[*document.Document]*syntax.KixHighlighter)
			}
			highlighter = syntax.NewKixHighlighter()
			state.kixHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.kixHighlighters, doc)
	}
	language, _ := syntax.LanguageByMode(snapshot.Language)
	if language.Mode == ".orc" {
		highlighter := state.csoundHighlighters[doc]
		if highlighter == nil {
			if state.csoundHighlighters == nil {
				state.csoundHighlighters = make(map[*document.Document]*syntax.CSoundHighlighter)
			}
			highlighter = syntax.NewCSoundHighlighter()
			state.csoundHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.csoundHighlighters, doc)
	}
	if language.Mode == ".reb" {
		highlighter := state.rebolHighlighters[doc]
		if highlighter == nil {
			if state.rebolHighlighters == nil {
				state.rebolHighlighters = make(map[*document.Document]*syntax.REBOLHighlighter)
			}
			highlighter = syntax.NewREBOLHighlighter()
			state.rebolHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.rebolHighlighters, doc)
	}
	if language.Mode == ".avs" {
		highlighter := state.avsHighlighters[doc]
		if highlighter == nil {
			if state.avsHighlighters == nil {
				state.avsHighlighters = make(map[*document.Document]*syntax.AviSynthHighlighter)
			}
			highlighter = syntax.NewAviSynthHighlighter()
			state.avsHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.avsHighlighters, doc)
	}
	if language.Mode == ".bas" {
		highlighter := state.freeBasicHighlighters[doc]
		if highlighter == nil {
			if state.freeBasicHighlighters == nil {
				state.freeBasicHighlighters = make(map[*document.Document]*syntax.FreeBasicHighlighter)
			}
			highlighter = syntax.NewFreeBasicHighlighter()
			state.freeBasicHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.freeBasicHighlighters, doc)
	}
	if language.Mode == ".osx" {
		highlighter := state.oscriptHighlighters[doc]
		if highlighter == nil {
			if state.oscriptHighlighters == nil {
				state.oscriptHighlighters = make(map[*document.Document]*syntax.OScriptHighlighter)
			}
			highlighter = syntax.NewOScriptHighlighter()
			state.oscriptHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.oscriptHighlighters, doc)
	}
	if language.Mode == ".pb" {
		highlighter := state.pureBasicHighlighters[doc]
		if highlighter == nil {
			if state.pureBasicHighlighters == nil {
				state.pureBasicHighlighters = make(map[*document.Document]*syntax.PureBasicHighlighter)
			}
			highlighter = syntax.NewPureBasicHighlighter()
			state.pureBasicHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.pureBasicHighlighters, doc)
	}
	if language.Mode == ".gui" {
		highlighter := state.gui4CliHighlighters[doc]
		if highlighter == nil {
			if state.gui4CliHighlighters == nil {
				state.gui4CliHighlighters = make(map[*document.Document]*syntax.Gui4CliHighlighter)
			}
			highlighter = syntax.NewGui4CliHighlighter()
			state.gui4CliHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.gui4CliHighlighters, doc)
	}
	if language.Mode == ".t2t" {
		highlighter := state.txt2tagsHighlighters[doc]
		if highlighter == nil {
			if state.txt2tagsHighlighters == nil {
				state.txt2tagsHighlighters = make(map[*document.Document]*syntax.Txt2tagsHighlighter)
			}
			highlighter = syntax.NewTxt2tagsHighlighter()
			state.txt2tagsHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.txt2tagsHighlighters, doc)
	}
	if language.Mode == ".hws" {
		highlighter := state.hollywoodHighlighters[doc]
		if highlighter == nil {
			if state.hollywoodHighlighters == nil {
				state.hollywoodHighlighters = make(map[*document.Document]*syntax.HollywoodHighlighter)
			}
			highlighter = syntax.NewHollywoodHighlighter()
			state.hollywoodHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.hollywoodHighlighters, doc)
	}
	if language.Mode == ".pro" {
		highlighter := state.visualPrologHighlighters[doc]
		if highlighter == nil {
			if state.visualPrologHighlighters == nil {
				state.visualPrologHighlighters = make(map[*document.Document]*syntax.VisualPrologHighlighter)
			}
			highlighter = syntax.NewVisualPrologHighlighter()
			state.visualPrologHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.visualPrologHighlighters, doc)
	}
	if language.Mode == ".src" {
		highlighter := state.escriptHighlighters[doc]
		if highlighter == nil {
			if state.escriptHighlighters == nil {
				state.escriptHighlighters = make(map[*document.Document]*syntax.ESCRIPTHighlighter)
			}
			highlighter = syntax.NewESCRIPTHighlighter()
			state.escriptHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.escriptHighlighters, doc)
	}
	if language.Mode == ".bc" {
		highlighter := state.baanHighlighters[doc]
		if highlighter == nil {
			if state.baanHighlighters == nil {
				state.baanHighlighters = make(map[*document.Document]*syntax.BaanCHighlighter)
			}
			highlighter = syntax.NewBaanCHighlighter()
			state.baanHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(value)
	} else {
		delete(state.baanHighlighters, doc)
	}
	if language.Mode == ".asp" || language.Mode == ".jsp" {
		highlighter := state.serverPageHighlighters[doc]
		if highlighter == nil {
			if state.serverPageHighlighters == nil {
				state.serverPageHighlighters = make(map[*document.Document]*syntax.ServerPageHighlighter)
			}
			highlighter = new(syntax.ServerPageHighlighter)
			state.serverPageHighlighters[doc] = highlighter
		}
		snapshot.HighlightLine = highlighter.ForText(language.Mode, value)
	} else {
		delete(state.serverPageHighlighters, doc)
	}
	if language.Mode == ".ans" {
		background := state.ansiBackgrounds[doc]
		if background == nil {
			if state.ansiBackgrounds == nil {
				state.ansiBackgrounds = make(map[*document.Document]*syntax.ANSIBackground)
			}
			background = new(syntax.ANSIBackground)
			state.ansiBackgrounds[doc] = background
		}
		snapshot.MarkLayers = background.AppendLayers(value, snapshot.MarkLayers)
	} else {
		delete(state.ansiBackgrounds, doc)
	}
	state.applyChangeDecorations(&snapshot, doc)
	state.paintRectangleSelection(&snapshot, doc, pane)
	state.paintMultiSelection(&snapshot, doc, pane)
	foldScope, hideScope, badgeScope := leftFoldScope, hideLineScope, leftFoldBadgeScope
	if pane == 1 {
		foldScope, hideScope, badgeScope = rightFoldScope, rightHideLineScope, rightFoldBadgeScope
	}
	if len(regions) != 0 || len(hidden) != 0 {
		snapshot.FoldGutter = true
		if provisional {
			snapshot.FoldedLines = provisionalFoldRanges(regions, hidden, state.folded[doc])
		} else {
			snapshot.FoldedLines = state.visualFoldRanges(doc)
		}
		for _, region := range regions {
			marker := components.CodeEditorFoldMarker{
				Line: region.Header, ID: foldScope.At(region.Header), Collapsed: state.folded[doc][region.Header],
			}
			if marker.Collapsed {
				marker.PlaceholderID = badgeScope.At(region.Header)
			}
			snapshot.FoldMarkers = append(snapshot.FoldMarkers, marker)
		}
		for _, group := range hidden {
			header := group.Start - 1
			snapshot.FoldMarkers = slices.DeleteFunc(snapshot.FoldMarkers, func(marker components.CodeEditorFoldMarker) bool { return marker.Line == header })
			snapshot.FoldMarkers = append(snapshot.FoldMarkers, components.CodeEditorFoldMarker{
				Line: header, ID: hideScope.At(header), PlaceholderID: badgeScope.At(header), Collapsed: true, Label: "Show hidden lines",
			})
		}
		slices.SortFunc(snapshot.FoldMarkers, func(left, right components.CodeEditorFoldMarker) int { return left.Line - right.Line })
	}
	if doc.LineEnding() == document.CRLF {
		snapshot.EOLLabel = "CRLF"
	} else if doc.LineEnding() == document.CR {
		snapshot.EOLLabel = "CR"
	} else {
		snapshot.EOLLabel = "LF"
	}
	if state.showEOL {
		state.eolLabels[pane] = doc.EOLLabels(state.eolLabels[pane])
		snapshot.EOLLabels = state.eolLabels[pane]
	}
	var bookmarks []int
	if state.provisional.doc == doc {
		// A live preedit has not yet committed its line mapping. Painting the
		// pre-composition lines keeps bookmarks from walking on every frame.
		if stored := state.bookmarks[doc]; stored != nil {
			bookmarks = stored.lines
		}
	} else {
		bookmarks = state.bookmarkLines(doc)
	}
	for _, line := range bookmarks {
		snapshot.GutterMarkers = append(snapshot.GutterMarkers, components.CodeEditorGutterMarker{Line: line, Color: gfx.RGB(43, 104, 187)})
	}
	for style, color := range styleColors {
		var spans []text.Span
		if provisional {
			if stored := state.styledMarks[doc]; stored != nil && style < len(stored.styles) {
				spans = stored.styles[style]
			}
		} else {
			spans = state.styleSpans(doc, style)
		}
		if len(spans) != 0 {
			color.Alpha = 100.0 / 255
			snapshot.MarkLayers = append(snapshot.MarkLayers, components.CodeEditorMarkLayer{Spans: spans, Color: color})
		}
	}
	if state.incrementOpen && state.incrementHighlight && doc == state.space.Current() {
		snapshot.Marks = state.incrementMatches
		snapshot.CurrentMark = text.Span{Start: snapshot.SelectionStart, End: snapshot.SelectionEnd}
	} else if state.searchOpen && doc == state.space.Current() {
		snapshot.Marks = state.searchMarks()
		snapshot.CurrentMark = text.Span{Start: snapshot.SelectionStart, End: snapshot.SelectionEnd}
	} else if state.markedDocument == doc {
		snapshot.MarkLayers = append(snapshot.MarkLayers, components.CodeEditorMarkLayer{Spans: state.markedSpans, Color: gfx.Color{Red: 1, Alpha: 100.0 / 255}})
	}
	if composing {
		snapshot.CompositionStart = doc.Editor.ByteOffset(composition.Start)
		snapshot.CompositionEnd = doc.Editor.ByteOffset(composition.End)
	}
	state.codeEditorForPane(pane).Paint(canvas, theme, bounds, snapshot)
	for _, marker := range snapshot.FoldMarkers {
		if !canvas.Clicked(marker.ID) && !canvas.Clicked(marker.PlaceholderID) {
			continue
		}
		if marker.ID.Scope() == hideScope {
			state.showHiddenLines(doc, marker.Line)
		} else {
			state.toggleFold(doc, marker.Line)
		}
		break
	}
}

func (state *model) paintSearch(canvas *gfx.Canvas, width, y float32) {
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: searchHeight}, palette.chrome)
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: 1}, palette.border)
	fieldWidth := min(330, max(170, width-380))
	fieldTheme := gfx.LightTheme()
	fieldTheme.Metrics.ControlHeight, fieldTheme.Metrics.FontSizeM, fieldTheme.Metrics.RadiusControl = 26, 12, 2
	fieldTheme.Border, fieldTheme.Ring = palette.border, palette.blue
	fieldTheme.Background, fieldTheme.Foreground = palette.white, palette.text
	paintField := func(id gfx.ID, editorValue string, row float32, label string) {
		smallLabel(canvas, label, gfx.Rect{X: 12, Y: y + row, Width: 58, Height: 26}, palette.text, 12)
		components.PaintInput(canvas, fieldTheme, gfx.Rect{X: 68, Y: y + row, Width: fieldWidth, Height: 25}, components.InputSnapshot{
			ID: id, Label: label, Value: editorValue, PlatformEditing: canvas.Focused() == id,
		})
	}
	paintField(queryID, state.query.String(), 6, "Find:")
	paintField(replacementID, state.replacement.String(), 37, "Replace:")
	x := 76 + fieldWidth
	searchAction := func(id gfx.ID, label string, top float32, buttonWidth float32) bool {
		bounds := gfx.Rect{X: x, Y: y + top, Width: buttonWidth, Height: 25}
		background := palette.toolbar
		if canvas.Hovered() == id {
			background = palette.hover
		}
		canvas.Border(bounds, 1, palette.border, background)
		canvas.LabelCenter(label, bounds, gfx.TextStyle{Size: 11, Color: palette.text})
		x += buttonWidth + 5
		return hitButton(canvas, id, bounds, label)
	}
	if searchAction(findNextID, "Next", 6, 64) {
		state.find(false)
	}
	if searchAction(findPreviousID, "Previous", 6, 74) {
		state.find(true)
	}
	if searchAction(findAllCurrentID, "Find All", 6, 72) {
		state.findAllCurrent()
	}
	x = 76 + fieldWidth
	if searchAction(replaceID, "Replace", 37, 70) {
		doc := state.space.Current()
		if state.isReadOnly(doc) {
			state.notice = "Read-only document"
		} else {
			state.bookmarkLines(doc)
			replaced, _, err := doc.ReplaceCurrentWithEdit(state.query.String(), state.replacement.String(), state.searchOptions)
			state.bookmarkLines(doc)
			if err != nil {
				state.notice = err.Error()
			} else if !replaced {
				state.find(false)
			}
		}
	}
	if searchAction(replaceAllID, "Replace All", 37, 87) {
		doc := state.space.Current()
		if state.isReadOnly(doc) {
			state.notice = "Read-only document"
		} else {
			state.bookmarkLines(doc)
			count, _, err := doc.ReplaceAllWithEdits(state.query.String(), state.replacement.String(), state.searchOptions)
			state.bookmarkLines(doc)
			if err != nil {
				state.notice = err.Error()
			} else {
				state.notice = fmt.Sprintf("Replaced %d occurrence(s)", count)
			}
		}
	}
	if searchAction(replaceOpenID, "Replace Open", 37, 88) {
		state.replaceAllOpenDocuments()
	}
	options := []struct {
		id      gfx.ID
		label   string
		x, span float32
		checked *bool
	}{
		{matchCaseID, "Match case", 12, 119, &state.searchOptions.MatchCase},
		{wholeWordID, "Whole word", 137, 119, &state.searchOptions.WholeWord},
		{regexID, "Regular expression", 262, 138, &state.searchOptions.Regex},
	}
	for _, option := range options {
		box := gfx.Rect{X: option.x, Y: y + 72, Width: 14, Height: 14}
		canvas.Border(box, 1, palette.border, palette.white)
		if *option.checked {
			canvas.Fill(box.Inset(gfx.All(3)), palette.blue)
		}
		smallLabel(canvas, option.label, gfx.Rect{X: option.x + 20, Y: y + 66, Width: 118, Height: 25}, palette.text, 11)
		canvas.HitWithOptions(option.id, gfx.Rect{X: option.x, Y: y + 66, Width: option.span, Height: 25}, option.label, gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: *option.checked})
		if canvas.Clicked(option.id) {
			*option.checked = !*option.checked
		}
	}
	closeBounds := gfx.Rect{X: width - 29, Y: y + 6, Width: 22, Height: 22}
	canvas.DrawIcon(closeIcon, controlIconBounds(closeBounds), palette.muted)
	if hitButton(canvas, searchCloseID, closeBounds, "Close search") {
		state.searchOpen = false
	}
}

func (state *model) paintGoTo(canvas *gfx.Canvas, width, y float32) {
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: goToHeight}, palette.chrome)
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: 1}, palette.border)
	smallLabel(canvas, "Go to line:", gfx.Rect{X: 12, Y: y + 8, Width: 76, Height: 26}, palette.text, 12)
	theme := gfx.LightTheme()
	theme.Metrics.ControlHeight, theme.Metrics.FontSizeM, theme.Metrics.RadiusControl = 26, 12, 2
	components.PaintInput(canvas, theme, gfx.Rect{X: 90, Y: y + 8, Width: 160, Height: 26}, components.InputSnapshot{
		ID: goToFieldID, Label: "Line number", Value: state.goToField.String(), PlatformEditing: canvas.Focused() == goToFieldID,
	})
	button := gfx.Rect{X: 259, Y: y + 8, Width: 48, Height: 26}
	canvas.Border(button, 1, palette.border, palette.toolbar)
	canvas.LabelCenter("Go", button, gfx.TextStyle{Size: 12, Color: palette.text})
	if hitButton(canvas, goToButtonID, button, "Go to line") {
		state.goToLine()
	}
	closeBounds := gfx.Rect{X: width - 29, Y: y + 9, Width: 22, Height: 22}
	canvas.DrawIcon(closeIcon, controlIconBounds(closeBounds), palette.muted)
	if hitButton(canvas, goToCloseID, closeBounds, "Close go to line") {
		state.goToOpen = false
	}
}

func (state *model) paintFilesDialog(canvas *gfx.Canvas, size gfx.Size) {
	canvas.SetEscapeTarget(filesCancelID)
	canvas.PushLayer(5)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.20))
	canvas.HitWithOptions(filesBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Find in Files dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(6)
	width, height, shift := min(560, size.Width-32), float32(304), float32(0)
	title := "Find in Files"
	if state.filesReplaceMode {
		height, shift, title = 342, 38, "Replace in Files"
	}
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	smallLabel(canvas, title, gfx.Rect{X: x + 18, Y: y + 12, Width: width - 36, Height: 26}, palette.text, 15)
	fieldTheme := gfx.LightTheme()
	fieldTheme.Metrics.ControlHeight, fieldTheme.Metrics.FontSizeM, fieldTheme.Metrics.RadiusControl = 26, 12, 2
	fieldTheme.Border, fieldTheme.Ring = palette.border, palette.blue
	fieldTheme.Background, fieldTheme.Foreground = palette.white, palette.text
	field := func(id gfx.ID, label, value string, offset, fieldWidth float32) {
		smallLabel(canvas, label, gfx.Rect{X: x + 18, Y: y + offset, Width: 76, Height: 26}, palette.text, 12)
		components.PaintInput(canvas, fieldTheme, gfx.Rect{X: x + 94, Y: y + offset, Width: fieldWidth, Height: 26}, components.InputSnapshot{
			ID: id, Label: label, Value: value, PlatformEditing: canvas.Focused() == id,
		})
	}
	field(queryID, "Find:", state.query.String(), 45, width-116)
	if state.filesReplaceMode {
		field(replacementID, "Replace:", state.replacement.String(), 81, width-116)
	}
	field(filesRootID, "Directory:", state.filesRoot.String(), 81+shift, width-201)
	field(filesPatternID, "Filter:", state.filesPattern.String(), 117+shift, min(220, width-116))
	button := func(id gfx.ID, label string, bounds gfx.Rect) bool {
		background := palette.toolbar
		if canvas.Hovered() == id {
			background = palette.hover
		}
		canvas.Border(bounds, 1, palette.border, background)
		canvas.LabelCenter(label, bounds, gfx.TextStyle{Size: 12, Color: palette.text})
		return hitButton(canvas, id, bounds, label)
	}
	if button(filesBrowseID, "Browse...", gfx.Rect{X: x + width - 96, Y: y + 81 + shift, Width: 78, Height: 26}) {
		state.browseFilesDirectory()
	}
	checkbox := func(id gfx.ID, label string, checked *bool, left, top, span float32) {
		box := gfx.Rect{X: left, Y: top + 6, Width: 14, Height: 14}
		canvas.Border(box, 1, palette.border, palette.white)
		if *checked {
			canvas.Fill(box.Inset(gfx.All(3)), palette.blue)
		}
		smallLabel(canvas, label, gfx.Rect{X: left + 20, Y: top, Width: span - 20, Height: 26}, palette.text, 11)
		canvas.HitWithOptions(id, gfx.Rect{X: left, Y: top, Width: span, Height: 26}, label, gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: *checked})
		if canvas.Clicked(id) {
			*checked = !*checked
		}
	}
	smallLabel(canvas, "Encoding:", gfx.Rect{X: x + 18, Y: y + 151 + shift, Width: 76, Height: 26}, palette.text, 12)
	segmentX := x + float32(94)
	for _, mode := range []struct {
		id       gfx.ID
		label    string
		encoding document.Encoding
		width    float32
	}{
		{filesUnicodeID, "Unicode (auto)", document.UTF8, 111},
		{filesGB18030ID, "GB18030", document.GB18030, 92},
		{filesWindowsID, "Windows-1252", document.Windows1252, 119},
	} {
		bounds := gfx.Rect{X: segmentX, Y: y + 151 + shift, Width: mode.width, Height: 26}
		selected := state.filesEncoding == mode.encoding
		background, foreground := palette.toolbar, palette.text
		if selected || canvas.Hovered() == mode.id {
			background = palette.hover
		}
		if selected {
			foreground = palette.blue
		}
		canvas.Border(bounds, 1, palette.border, background)
		canvas.LabelCenter(mode.label, bounds, gfx.TextStyle{Size: 11, Color: foreground})
		canvas.HitWithOptions(mode.id, bounds, mode.label, gfx.HitOptions{Role: gfx.RoleRadio, Checked: selected})
		if canvas.Clicked(mode.id) {
			state.filesEncoding = mode.encoding
		}
		segmentX += mode.width
	}
	checkbox(filesRecursiveID, "In all sub-folders", &state.filesRecursive, x+94, y+183+shift, 170)
	checkbox(matchCaseID, "Match case", &state.searchOptions.MatchCase, x+18, y+215+shift, 112)
	checkbox(wholeWordID, "Whole word", &state.searchOptions.WholeWord, x+136, y+215+shift, 116)
	checkbox(regexID, "Regular expression", &state.searchOptions.Regex, x+260, y+215+shift, 157)
	if state.filesSearching {
		smallLabel(canvas, "Working...", gfx.Rect{X: x + 18, Y: y + 262 + shift, Width: 160, Height: 26}, palette.muted, 12)
	} else if state.filesReplaceMode {
		if button(replacePreviewID, "Preview", gfx.Rect{X: x + width - 190, Y: y + 259 + shift, Width: 82, Height: 29}) {
			state.startReplacePreview()
		}
	} else if button(filesSearchID, "Find All", gfx.Rect{X: x + width - 190, Y: y + 259, Width: 82, Height: 29}) {
		state.startFileSearch()
	}
	if state.filesMessage != "" {
		message := canvas.FitLine(state.filesMessage, 11, gfx.WeightRegular, width-230)
		smallLabel(canvas, message, gfx.Rect{X: x + 18, Y: y + 262 + shift, Width: width - 230, Height: 26}, palette.blue, 11)
	}
	if button(filesCancelID, "Cancel", gfx.Rect{X: x + width - 98, Y: y + 259 + shift, Width: 80, Height: 29}) {
		state.cancelFilesDialog()
		canvas.RequestFocus(state.commandEditorFocusID())
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

func (state *model) paintReplaceConfirm(canvas *gfx.Canvas, size gfx.Size) {
	plan := state.replacePlan
	if plan == nil {
		return
	}
	if !state.filesApplying {
		canvas.SetEscapeTarget(replaceCancelID)
	}
	canvas.PushLayer(7)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(replaceBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Replace in Files preview", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(8)
	width, height := min(560, size.Width-32), min(320, size.Height-32)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 4, 1, palette.border, palette.white)
	title := fmt.Sprintf("Replace %d matches in %d files?", plan.Replacements, len(plan.Changes))
	smallLabel(canvas, title, gfx.Rect{X: x + 20, Y: y + 14, Width: width - 40, Height: 30}, palette.text, 15)
	warning := fmt.Sprintf("%d files skipped. Files are saved one by one; batch undo is unavailable.", plan.FilesSkipped)
	warning = canvas.FitLine(warning, 11, gfx.WeightRegular, width-40)
	smallLabel(canvas, warning, gfx.Rect{X: x + 20, Y: y + 47, Width: width - 40, Height: 24}, palette.muted, 11)
	for index, change := range plan.Changes {
		if index >= 6 {
			break
		}
		row := gfx.Rect{X: x + 20, Y: y + 78 + float32(index)*25, Width: width - 40, Height: 24}
		label := fmt.Sprintf("%s  (%d replacements)", filepath.Base(change.Path), change.Count)
		label = canvas.FitLine(label, 12, gfx.WeightRegular, row.Width)
		smallLabel(canvas, label, row, palette.text, 12)
	}
	if len(plan.Changes) > 6 {
		smallLabel(canvas, fmt.Sprintf("and %d more files", len(plan.Changes)-6), gfx.Rect{X: x + 20, Y: y + 228, Width: width - 40, Height: 22}, palette.muted, 11)
	}
	if state.filesApplying {
		smallLabel(canvas, "Applying changes...", gfx.Rect{X: x + 20, Y: y + height - 43, Width: 190, Height: 28}, palette.muted, 12)
	} else if state.notice == "Save or close modified tabs before replacing files" ||
		state.notice == "Close tabs opened with another encoding before replacing files" {
		message := canvas.FitLine(state.notice, 11, gfx.WeightRegular, width-40)
		smallLabel(canvas, message, gfx.Rect{X: x + 20, Y: y + height - 70, Width: width - 40, Height: 24}, palette.blue, 11)
	}
	button := func(id gfx.ID, label string, bounds gfx.Rect) bool {
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(label, bounds, gfx.TextStyle{Size: 12, Color: palette.text})
		return hitButton(canvas, id, bounds, label)
	}
	if !state.filesApplying {
		if button(replaceConfirmID, "Replace Files", gfx.Rect{X: x + width - 215, Y: y + height - 43, Width: 105, Height: 29}) {
			state.startReplaceApply()
		}
		if button(replaceCancelID, "Cancel", gfx.Rect{X: x + width - 98, Y: y + height - 43, Width: 80, Height: 29}) {
			state.cancelReplacePreview()
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

func (state *model) paintFileResults(canvas *gfx.Canvas, width, y, height float32) {
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: height}, palette.white)
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: 1}, palette.border)
	canvas.Fill(gfx.Rect{Y: y + 1, Width: width, Height: 28}, palette.chrome)
	if canvas.Focused() == resultsPaneID || canvas.Focused().Scope() == resultScope {
		canvas.Fill(gfx.Rect{Y: y + 1, Width: width, Height: 2}, palette.blue)
	}
	query := state.resultQuery
	if query == "" {
		query = state.query.String()
	}
	heading := fmt.Sprintf("Find result: %s  (%d matches, %d files, %d skipped)", query, len(state.fileResults.Matches), state.fileResults.FilesSearched, state.fileResults.FilesSkipped)
	if state.resultDocument != nil {
		heading = fmt.Sprintf("Find result: %s  (%d matches in %s)", query, len(state.fileResults.Matches), state.resultDocument.Name)
	} else if state.resultOpenDocuments {
		heading = fmt.Sprintf("Find result: %s  (%d matches in open documents)", query, len(state.fileResults.Matches))
	}
	heading = canvas.FitLine(heading, 12, gfx.WeightRegular, width-48)
	smallLabel(canvas, heading, gfx.Rect{X: 10, Y: y + 1, Width: width - 48, Height: 27}, palette.text, 12)
	closeBounds := gfx.Rect{X: width - 29, Y: y + 4, Width: 22, Height: 22}
	canvas.DrawIcon(closeIcon, controlIconBounds(closeBounds), palette.muted)
	if hitButton(canvas, resultsCloseID, closeBounds, "Close find results") {
		state.discardFileResults()
		return
	}
	canvas.HitWithOptions(resultsPaneID, gfx.Rect{Y: y + 29, Width: width, Height: height - 29}, "Find results", gfx.HitOptions{Role: gfx.RoleGroup})
	visible := max(1, int((height-30)/22))
	state.resultVisible = visible
	for row := 0; row < visible && state.resultOffset+row < len(state.fileResults.Matches); row++ {
		index := state.resultOffset + row
		match := state.fileResults.Matches[index]
		bounds := gfx.Rect{X: 4, Y: y + 30 + float32(row)*22, Width: width - 8, Height: 22}
		if index == state.resultSelected {
			canvas.Fill(bounds, gfx.RGB(241, 243, 246))
			canvas.Fill(gfx.Rect{X: bounds.X, Y: bounds.Y, Width: 2, Height: bounds.Height}, palette.blue)
		} else if canvas.Hovered() == resultScope.At(index) {
			canvas.Fill(bounds, palette.hover)
		}
		name := filepath.Base(match.Path)
		if state.resultDocument != nil {
			name = state.resultDocument.Name
		} else if state.resultOpenDocuments && index < len(state.resultDocuments) {
			name = state.resultDocuments[index].Name
		}
		label := fmt.Sprintf("%s:%d:%d  %s", name, match.Line, match.Column, match.Preview)
		label = canvas.FitLine(label, 11, gfx.WeightRegular, width-26)
		smallLabel(canvas, label, gfx.Rect{X: 12, Y: bounds.Y, Width: width - 26, Height: 22}, palette.text, 11)
		canvas.HitWithOptions(resultScope.At(index), bounds, match.Path, gfx.HitOptions{Role: gfx.RoleButton, Hint: match.Preview, Selected: index == state.resultSelected})
		if canvas.Clicked(resultScope.At(index)) {
			state.openFileResult(index)
		}
	}
}

func (state *model) paintStatus(canvas *gfx.Canvas, width, y float32) {
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: statusHeight}, palette.chrome)
	canvas.Fill(gfx.Rect{Y: y, Width: width, Height: 1}, palette.border)
	doc := state.space.Current()
	if state.noChangeNoticeDoc != nil {
		if state.notice != noChangeNavigationNotice || doc != state.noChangeNoticeDoc {
			state.retireNoChangeNotice("")
		} else if consumer, err := doc.ChangeHistory(); err != nil {
			state.retireNoChangeNotice(err.Error())
		} else if consumer.Revision() != state.noChangeNoticeRevision {
			state.retireNoChangeNotice("")
		}
	}
	line, column := doc.Position()
	message := state.notice
	if message == "" {
		if state.macroRecording {
			message = fmt.Sprintf("Recording macro (%d steps)", len(state.macroSteps))
		} else {
			message = state.languageLabel(doc)
		}
	}
	for _, hover := range []struct {
		id    gfx.ID
		label string
	}{
		{newID, "New"}, {openID, "Open"}, {saveID, "Save"}, {undoID, "Undo"},
		{redoID, "Redo"}, {cutID, "Cut"}, {copyID, "Copy"}, {pasteID, "Paste"}, {searchID, "Find"},
	} {
		if canvas.Hovered() == hover.id {
			message = hover.label
			break
		}
	}
	encoding := doc.EncodingLabel()
	mode, modeWidth := "INS", float32(38)
	if state.isReadOnly(doc) {
		mode, modeWidth = "READ ONLY", 72
	}
	cells := [...]struct {
		label string
		width float32
	}{
		{fmt.Sprintf("length: %d", doc.Editor.RuneCount()), 91},
		{fmt.Sprintf("lines: %d", doc.LineCount()), 78},
		{fmt.Sprintf("Ln: %d  Col: %d", line, column), 112},
		{encoding, 85}, {doc.NewlineLabel(), 112}, {mode, modeWidth},
	}
	shown := cells[:]
	if width < 850 {
		shown = cells[2:]
	}
	total := float32(0)
	for _, cell := range shown {
		total += cell.width
	}
	messageWidth := max(60, width-total-20)
	message = canvas.FitLine(message, 11, gfx.WeightRegular, messageWidth)
	smallLabel(canvas, message, gfx.Rect{X: 10, Y: y + 1, Width: messageWidth, Height: 21}, palette.muted, 11)
	x := width - total
	for _, cell := range shown {
		canvas.Fill(gfx.Rect{X: x, Y: y + 3, Width: 1, Height: 18}, palette.border)
		smallLabel(canvas, cell.label, gfx.Rect{X: x + 7, Y: y + 1, Width: cell.width - 9, Height: 21}, palette.text, 11)
		x += cell.width
	}
}

func (state *model) paintOpenMenu(canvas *gfx.Canvas, size gfx.Size) {
	if state.menuOpen < 0 || state.menuOpen >= len(menuCommands) {
		return
	}
	canvas.SetEscapeTarget(menuDismissID)
	canvas.PushLayer(1)
	canvas.HitWithOptions(menuDismissID, gfx.Rect{Y: menuHeight, Width: size.Width, Height: size.Height - menuHeight}, "Close menu", gfx.HitOptions{Role: gfx.RoleButton})
	if canvas.Clicked(menuDismissID) {
		state.pasteSpecialOpen = false
		state.menuOpen, state.recentOpen, state.lineMenuOpen, state.caseMenuOpen, state.sessionMenuOpen, state.bookmarkMenuOpen, state.changeHistoryMenuOpen, state.closeMultipleMenuOpen, state.containingMenuOpen, state.autoMenuOpen, state.selectedSearchMenuOpen, state.blankMenuOpen, state.copyPathsMenuOpen, state.readOnlyMenuOpen, state.onSelectionMenuOpen, state.multiAllMenuOpen, state.multiNextMenuOpen = -1, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false
		state.insertDateOpen = false
		state.markJumpMenuOpen = false
		state.styleMenuMode = ""
		state.hashMenuOpen = false
		state.symbolMenuOpen = false
		state.viewTabMenuOpen = false
		state.namedBrowserMenuOpen = false
		state.foldMenuOpen = false
		state.windowSortMenuOpen = false
		state.commentMenuOpen = false
		state.eolMenuOpen = false
		state.charsetMenuOpen = false
		state.savedMacroMenuOpen = false
		canvas.PopLayer()
		return
	}
	canvas.PopLayer()
	x := float32(8)
	for index := 0; index < state.menuOpen; index++ {
		x += canvas.TextWidth(menuLabels[index], 11) + 19
	}
	if state.menuOpen == 11 {
		state.paintWindowMenu(canvas, size, x)
		return
	}
	if state.menuOpen == 5 {
		state.paintLanguageMenu(canvas, size, x)
		return
	}
	if state.menuOpen == 3 {
		state.paintViewMenu(canvas, size, x)
		return
	}
	if state.menuOpen == 9 {
		state.paintRunMenu(canvas, size, x)
		return
	}
	commands := menuCommands[state.menuOpen]
	count := 0
	for _, command := range commands {
		if command != "" {
			count++
		}
	}
	width := float32(218)
	if state.menuOpen == 0 {
		width = min(310, size.Width-12)
	} else if state.menuOpen == 1 {
		width = min(260, size.Width-8)
	} else if state.menuOpen == 2 {
		width = 270
	}
	if x+width > size.Width {
		x = max(2, size.Width-width-2)
	}
	rowHeight := float32(27)
	visible, controls := count, float32(0)
	if state.menuOpen == 0 {
		rowHeight = fileMenuRowHeight
	}
	available := size.Height - menuHeight - statusHeight - 8
	if state.menuOpen == 0 && float32(count)*rowHeight+8 > available {
		rowHeight, controls = 20, 44
		visible = min(count, max(1, int((available-controls-8)/rowHeight)))
	}
	if (state.menuOpen == 1 || state.menuOpen == 2) && float32(count)*rowHeight+8 > available {
		rowHeight, controls = 24, 44
		visible = min(count, max(1, int((available-controls-8)/rowHeight)))
	}
	if state.menuOpen == 1 {
		state.editMenuVisible = visible
		state.editMenuOffset = min(max(0, state.editMenuOffset), count-visible)
	}
	if state.menuOpen == 0 {
		state.fileMenuVisible = visible
		state.fileMenuOffset = min(max(0, state.fileMenuOffset), count-visible)
	}
	if state.menuOpen == 2 {
		state.searchMenuVisible = visible
		state.revealSearchSubmenuParent()
		state.searchMenuOffset = min(max(0, state.searchMenuOffset), count-visible)
	}
	if state.menuOpen != 0 && state.menuOpen != 1 && state.menuOpen != 2 && float32(count)*20 > available {
		available = size.Height - menuHeight - 8
	}
	if state.menuOpen != 0 && state.menuOpen != 1 && state.menuOpen != 2 && float32(count)*rowHeight > available {
		minimum := float32(20)
		if state.menuOpen == 2 {
			minimum = 16
		}
		rowHeight = max(minimum, available/float32(count))
	}
	canvas.PushLayer(2)
	canvas.PushOverlayClip(gfx.Rect{Width: size.Width, Height: size.Height})
	height := float32(visible)*rowHeight + controls + 8
	panel := gfx.Rect{X: x, Y: menuHeight, Width: width, Height: height}
	canvas.Border(panel, 1, palette.border, palette.white)
	if state.menuOpen == 0 {
		canvas.HitWithOptions(fileMenuPanelID, panel, "File menu", gfx.HitOptions{Role: gfx.RoleMenu})
	} else if state.menuOpen == 1 {
		canvas.HitWithOptions(editMenuPanelID, panel, "Edit menu", gfx.HitOptions{Role: gfx.RoleMenu})
	} else if state.menuOpen == 2 {
		canvas.HitWithOptions(searchMenuPanelID, panel, "Search menu", gfx.HitOptions{Role: gfx.RoleMenu})
	}
	if controls > 0 {
		previousID, nextID, noun := editMenuPreviousID, editMenuNextID, "Edit"
		if state.menuOpen == 0 {
			previousID, nextID, noun = fileMenuPreviousID, fileMenuNextID, "File"
		} else if state.menuOpen == 2 {
			previousID, nextID, noun = searchMenuPreviousID, searchMenuNextID, "Search"
		}
		for _, control := range []struct {
			id    gfx.ID
			icon  gfx.Icon
			label string
			y     float32
			step  int
		}{
			{previousID, upIcon, "Previous " + noun + " commands", menuHeight + 4, -1},
			{nextID, downIcon, "Next " + noun + " commands", menuHeight + height - 26, 1},
		} {
			bounds := gfx.Rect{X: x + 4, Y: control.y, Width: width - 8, Height: 22}
			canvas.DrawIcon(control.icon, gfx.Rect{X: bounds.X + bounds.Width/2 - 8, Y: bounds.Y + 3, Width: 16, Height: 16}, palette.muted)
			if hitButton(canvas, control.id, bounds, control.label) {
				if state.menuOpen == 0 {
					state.fileMenuOffset = min(max(0, state.fileMenuOffset+control.step), count-visible)
				} else if state.menuOpen == 2 {
					state.searchMenuOffset = min(max(0, state.searchMenuOffset+control.step), count-visible)
					state.closeSearchSubmenus()
				} else {
					state.editMenuOffset = min(max(0, state.editMenuOffset+control.step), count-visible)
				}
			}
		}
	}
	rows := gfx.Rect{X: x + 4, Y: menuHeight + 4 + controls/2, Width: width - 8, Height: float32(visible) * rowHeight}
	if controls > 0 {
		canvas.Clip(rows)
	}
	// Dispatch after declaring rows: closing the menu changes their index basis.
	selected := ""
	menuHot := canvas.Hovered()
	if menuHot.Scope() != menuItemScope {
		menuHot = canvas.FocusVisible()
	}
	for row := 0; row < visible; row++ {
		index := row
		if state.menuOpen == 0 {
			index += state.fileMenuOffset
		} else if state.menuOpen == 1 {
			index += state.editMenuOffset
		} else if state.menuOpen == 2 {
			index += state.searchMenuOffset
		}
		command := commands[index]
		item := gfx.Rect{X: rows.X, Y: rows.Y + float32(row)*rowHeight, Width: rows.Width, Height: rowHeight}
		id := menuItemScope.At(index)
		if menuHot == id {
			canvas.Fill(item, palette.hover)
		}
		checked := state.menuSelected(command)
		iconSize := float32(14)
		iconY := item.Y + (rowHeight-iconSize)/2
		if rowHeight < 18 {
			iconSize = 11
			iconY = item.Y + (rowHeight-iconSize)/2
		}
		if checked {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: item.X + 6, Y: iconY, Width: iconSize, Height: iconSize}, palette.blue)
		}
		if state.menuOpen == 7 || command == "Recent Files" || command == "Line Operations" || command == "Insert" || command == "Convert Case" || command == "Comment/Uncomment" || command == "EOL Conversion" || command == "Show Symbol" || command == "Tab" || command == "Fold Level" || command == "Unfold Level" || command == "Sort By" || command == "Blank Operations" || command == "Copy to Clipboard" || command == "On Selection" || command == "Multi-select All" || command == "Multi-select Next" || command == "Read-Only in Notepad++" || command == "Auto-Completion" || command == "Sessions" || command == "Bookmark" || command == "Change History" || command == "Jump Up" || command == "Jump Down" || command == "Style All Occurrences of Token" || command == "Style One Token" || command == "Clear Style" || command == "Copy Styled Text" || command == "Close Multiple Documents" || command == "Open Containing Folder" || command == "Selected Text Search" || command == "Character sets" || command == "Open as Encoding..." || command == "Saved Macros" || command == pasteSpecialCommand {
			canvas.DrawIcon(rightIcon, gfx.Rect{X: item.X + item.Width - 21, Y: iconY, Width: iconSize, Height: iconSize}, palette.muted)
		}
		fontSize := float32(12)
		if rowHeight < 18 {
			fontSize = 11
		}
		label := canvas.FitLine(command, fontSize, gfx.WeightRegular, item.Width-30)
		disabled := state.menuOpen == 11 && command == "Recent Window"
		color := palette.text
		if disabled {
			color = palette.muted
		}
		smallLabel(canvas, label, gfx.Rect{X: item.X + 27, Y: item.Y, Width: item.Width - 30, Height: item.Height}, color, fontSize)
		canvas.HitWithOptions(id, item, command, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: checked, Hint: command, Disabled: disabled})
		if !disabled && canvas.Clicked(id) {
			selected = command
		}
	}
	if controls > 0 {
		canvas.Unclip()
	}
	if selected != "" {
		state.command(selected)
	}
	searchParentY := func(command string) float32 {
		index := slices.Index(commands, command) - state.searchMenuOffset
		return rows.Y + float32(index)*rowHeight
	}
	if state.recentOpen && state.menuOpen == 0 {
		state.paintRecentMenu(canvas, size, x+width)
	}
	if state.charsetMenuOpen && (state.menuOpen == 0 || state.menuOpen == 4) {
		state.paintCharsetMenu(canvas, size, x+width)
	}
	if state.savedMacroMenuOpen && state.menuOpen == 8 {
		state.paintSavedMacroMenu(canvas, size, x+width)
	}
	if state.lineMenuOpen && state.menuOpen == 1 {
		state.paintLineMenu(canvas, size, x+width)
	}
	if state.insertDateOpen && state.menuOpen == 1 {
		state.paintInsertDateMenu(canvas, size, x+width)
	}
	if state.hashMenuOpen && state.menuOpen == 7 {
		state.paintHashMenu(canvas, size, x, x+width)
	}
	if state.windowSortMenuOpen && state.menuOpen == 11 {
		state.paintWindowSortMenu(canvas, size, x+width)
	}
	if state.sessionMenuOpen && state.menuOpen == 0 {
		state.paintSessionMenu(canvas, size, x+width)
	}
	if state.caseMenuOpen && state.menuOpen == 1 {
		state.paintCaseMenu(canvas, size, x+width)
	}
	if state.commentMenuOpen && state.menuOpen == 1 {
		state.paintCommentMenu(canvas, size, x+width)
	}
	if state.eolMenuOpen && state.menuOpen == 1 {
		state.paintEOLMenu(canvas, size, x+width)
	}
	if state.blankMenuOpen && state.menuOpen == 1 {
		state.paintBlankMenu(canvas, size, x+width)
	}
	if state.copyPathsMenuOpen && state.menuOpen == 1 {
		state.paintCopyPathsMenu(canvas, size, x+width)
	}
	if state.pasteSpecialOpen && state.menuOpen == 1 {
		index := slices.Index(commands, pasteSpecialCommand) - state.editMenuOffset
		state.paintPasteSpecial(canvas, size, x+width, rows.Y+float32(index)*rowHeight)
	}
	if state.onSelectionMenuOpen && state.menuOpen == 1 {
		state.paintOnSelectionMenu(canvas, size, x+width)
	}
	if (state.multiAllMenuOpen || state.multiNextMenuOpen) && state.menuOpen == 1 {
		state.paintMultiSelectMenu(canvas, size, x+width)
	}
	if state.readOnlyMenuOpen && state.menuOpen == 1 {
		state.paintReadOnlyMenu(canvas, size, x+width)
	}
	if state.bookmarkMenuOpen && state.menuOpen == 2 {
		state.paintBookmarkMenu(canvas, size, x+width, searchParentY("Bookmark"))
	}
	if state.changeHistoryMenuOpen && state.menuOpen == 2 {
		state.paintChangeHistoryMenu(canvas, size, x+width, searchParentY("Change History"))
	}
	if state.markJumpMenuOpen && state.menuOpen == 2 {
		state.paintMarkJumpMenu(canvas, size, x+width, searchParentY(state.searchSubmenuParent()))
	}
	if state.styleMenuMode != "" && state.menuOpen == 2 {
		state.paintStylePopup(canvas, size, x+width, searchParentY(state.searchSubmenuParent()), state.styleMenuMode)
	}
	if state.closeMultipleMenuOpen && state.menuOpen == 0 {
		state.paintCloseMultipleMenu(canvas, size, x+width)
	}
	if state.containingMenuOpen && state.menuOpen == 0 {
		state.paintContainingFolderMenu(canvas, size, x+width)
	}
	if state.autoMenuOpen && state.menuOpen == 1 {
		state.paintAutoCompletionMenu(canvas, size, x+width)
	}
	if state.selectedSearchMenuOpen && state.menuOpen == 2 {
		state.paintSelectedSearchMenu(canvas, size, x+width, searchParentY("Selected Text Search"))
	}
	canvas.PopOverlayClip()
	canvas.PopLayer()
}

func (state *model) menuSelected(command string) bool {
	doc := state.space.Current()
	if language, ok := languageExtension(command); ok {
		current := state.languageFor(doc)
		if current == language || language == "" && current == ".txt" {
			return true
		}
		if spec, ok := syntax.LanguageByMode(current); ok {
			return spec.Mode == language
		}
		return false
	}
	switch command {
	case "Change History: Margin", "Change History: Indicator", "Change History: Both", "Change History: Off":
		view := state.changeHistoryView
		if view == "" {
			view = "margin"
		}
		want := map[string]string{
			"Change History: Margin":    "margin",
			"Change History: Indicator": "indicator",
			"Change History: Both":      "both",
			"Change History: Off":       "off",
		}[command]
		return view == want
	case "Unix (LF)":
		return doc.LineEnding() == document.LF
	case "Windows (CR LF)":
		return doc.LineEnding() == document.CRLF
	case "Macintosh (CR)":
		return doc.LineEnding() == document.CR
	case "UTF-8":
		return doc.Encoding() == document.UTF8
	case "UTF-8 BOM":
		return doc.UTF8BOM()
	case "UTF-16 LE":
		return doc.Encoding() == document.UTF16LE
	case "UTF-16 BE":
		return doc.Encoding() == document.UTF16BE
	case "GB18030":
		return doc.Encoding() == document.GB18030
	case "Windows-1252":
		return doc.Encoding() == document.Windows1252
	case "Show Line Numbers":
		return state.showLineNumbers
	case "Show Space and Tab":
		return state.showWhitespace
	case "Show End of Line":
		return state.showEOL
	case "Show Indent Guide":
		return state.showIndentGuides
	case "Show Wrap Symbol":
		return state.showWrapSymbol
	case "Show Non-Printing Characters":
		return state.showNonPrinting
	case "Show Control Characters & Unicode EOL":
		return state.showControls
	case "Show All Characters":
		return state.showWhitespace && state.showEOL && state.showNonPrinting && state.showControls
	case "Always on Top":
		return state.alwaysOnTop
	case "Word Wrap":
		return state.wrap
	case "Tab Width: 2":
		return state.tabWidth == 2
	case "Tab Width: 4":
		return state.tabWidth == 4
	case "Tab Width: 8":
		return state.tabWidth == 8
	case "Indent: Language Default":
		return state.indentMode == "language"
	case "Indent: Tabs":
		return state.indentMode == "tabs"
	case "Indent: Spaces":
		return state.indentMode == "spaces"
	case "Auto Indent":
		return state.autoIndent
	case "Pair Brackets":
		return state.autoClose
	case "Record Macro":
		return state.macroRecording
	case "Split View":
		return state.splitEnabled
	case "Toggle Full Screen Mode":
		return state.fullscreen
	case distractionFreeCommand:
		return state.distractionFree
	case "Synchronize Zoom Across Views":
		return state.zoomSync
	case "Folder as Workspace":
		return state.folderVisible
	case "Document Map":
		return state.documentMapVisible
	case "Monitoring (tail -f)":
		return state.monitors[state.paneDocument(state.activePane)] != nil
	case "Function List":
		return state.functionListVisible
	case "Document List":
		return state.documentListVisible
	case characterPanelCommand:
		return state.characterVisible
	case "Synchronize Vertical Scrolling":
		return state.syncVertical
	case "Synchronize Horizontal Scrolling":
		return state.syncHorizontal
	case textDirectionRTL:
		return doc.RTL
	case textDirectionLTR:
		return !doc.RTL
	case "Begin/End Select":
		return state.selectAnchors[state.activePane].doc == doc
	case "Begin/End Select in Column Mode":
		return state.rectangles[state.activePane].doc == doc && state.rectangles[state.activePane].pending
	case "Read-Only on Current Document":
		return state.readOnly[doc]
	case "Read-Only Attribute in Windows", "File Read-Only Attribute":
		readonly, _ := document.FileReadOnly(doc.Path)
		return readonly
	}
	return false
}

func (state *model) languageLabel(doc *document.Document) string {
	mode := state.languageFor(doc)
	if mode == ".txt" {
		mode = ""
	}
	if spec, ok := syntax.LanguageByMode(mode); ok {
		return spec.Label
	}
	return "Normal text file"
}

func (state *model) paintCloseDialog(canvas *gfx.Canvas, size gfx.Size) {
	if state.pendingClose >= len(state.space.Documents) {
		state.pendingClose = -1
		return
	}
	canvas.PushLayer(3)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	canvas.HitWithOptions(dialogBackdropID, gfx.Rect{Width: size.Width, Height: size.Height}, "Save changes dialog", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true})
	canvas.PushLayer(4)
	width := float32(388)
	x, y := (size.Width-width)/2, (size.Height-158)/2
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: 158}, 4, 1, palette.border, palette.white)
	smallLabel(canvas, "Save changes to "+state.space.Documents[state.pendingClose].Name+"?", gfx.Rect{X: x + 20, Y: y + 18, Width: width - 40, Height: 30}, palette.text, 15)
	smallLabel(canvas, "Your changes will be lost if you discard them.", gfx.Rect{X: x + 20, Y: y + 56, Width: width - 40, Height: 25}, palette.muted, 12)
	buttons := []struct {
		id    gfx.ID
		label string
		x     float32
	}{
		{saveCloseID, "Save", x + 75}, {discardID, "Discard", x + 159}, {cancelCloseID, "Cancel", x + 270},
	}
	for _, button := range buttons {
		bounds := gfx.Rect{X: button.x, Y: y + 105, Width: 80, Height: 30}
		canvas.Border(bounds, 1, palette.border, palette.toolbar)
		canvas.LabelCenter(button.label, bounds, gfx.TextStyle{Size: 12, Color: palette.text})
		if hitButton(canvas, button.id, bounds, button.label) {
			switch button.id {
			case saveCloseID:
				state.finishClose(false)
			case discardID:
				state.finishClose(true)
			case cancelCloseID:
				state.cancelClose()
			}
		}
	}
	canvas.PopLayer()
	canvas.PopLayer()
}

// provisionalFoldRanges renders the fold geometry a live preedit has not
// committed: the cached regions and manually hidden ranges at their
// pre-composition positions instead of re-deriving them from the transient
// text. The terminal event replaces this with visualFoldRanges on a later
// paint.
func provisionalFoldRanges(regions []syntax.FoldRegion, hidden []hiddenLineRange, folded map[int]bool) []components.CodeEditorFoldRange {
	var ranges []components.CodeEditorFoldRange
	lastEnd := -1
	for _, region := range regions {
		if folded[region.Header] && region.Header > lastEnd {
			ranges = append(ranges, components.CodeEditorFoldRange{Header: region.Header, LastLine: region.LastLine})
			lastEnd = region.LastLine
		}
	}
	for _, group := range hidden {
		ranges = append(ranges, components.CodeEditorFoldRange{Header: group.Start - 1, LastLine: group.End})
	}
	slices.SortFunc(ranges, func(left, right components.CodeEditorFoldRange) int { return left.Header - right.Header })
	merged := ranges[:0]
	for _, candidate := range ranges {
		if len(merged) != 0 && candidate.Header <= merged[len(merged)-1].LastLine {
			merged[len(merged)-1].LastLine = max(merged[len(merged)-1].LastLine, candidate.LastLine)
		} else {
			merged = append(merged, candidate)
		}
	}
	return merged
}
