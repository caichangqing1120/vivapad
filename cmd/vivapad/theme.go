package main

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/preferences"
	"vivapad/internal/syntax"
)

type themePalette struct {
	menuBar, toolbarBar, toolbar, tabs, tab, activeTab, dock, mapBackground, statusBar, popup gfx.Color
	border, hover, text, muted, gutter, accent, selectedRow, inputBackground, invalidInput    gfx.Color
	editorBackground, editorForeground, gutterForeground, selection, selectionText            gfx.Color
	toolbarInk, toolbarDisabled, toolbarSelected, toolbarHover, toolbarPressed                gfx.Color
	toolbarFocus, toolbarSeparator, pin, bookmark, mapViewport, mapLine, invisible            gfx.Color
	findMark, findMatch                                                                       gfx.Color
	tokenMarks                                                                                [5]gfx.Color
	changeReverted, changeSaved, changeModified, changeRevertedModified                       gfx.Color
	diff                                                                                      [3]gfx.Color
}

var classicPalette = themePalette{
	menuBar: gfx.RGB24(0xf8f8f9), toolbarBar: gfx.RGB24(0xfbfcfc), toolbar: gfx.RGB24(0xfdfdfd),
	tabs: gfx.RGB24(0xeeeff1), tab: gfx.RGB24(0xe6e8eb), activeTab: gfx.RGB24(0xffffff),
	dock: gfx.RGB24(0xfafafa), mapBackground: gfx.RGB24(0xf7f8f9), statusBar: gfx.RGB24(0xf8f8f9), popup: gfx.RGB24(0xffffff),
	border: gfx.RGB24(0xcfd2d6), hover: gfx.RGB24(0xfcecef), text: gfx.RGB24(0x272a2f), muted: gfx.RGB24(0x70747b),
	gutter: gfx.RGB24(0xe0e0e0), accent: gfx.RGB24(0xd7263d), selectedRow: gfx.RGB24(0xf1f3f6), inputBackground: gfx.RGB24(0xffffff), invalidInput: gfx.RGB24(0xffeaed),
	editorBackground: gfx.RGB24(0xffffff), editorForeground: gfx.RGB24(0x272a2f), gutterForeground: gfx.RGB24(0x737373),
	selection: gfx.RGB24(0xb5d3ff), selectionText: gfx.RGB24(0x111827),
	toolbarInk: gfx.RGB24(0x343d43), toolbarDisabled: gfx.RGB24(0xa6afb6), toolbarSelected: gfx.RGB24(0xddf3ed),
	toolbarHover: gfx.RGB24(0xe8ecee), toolbarPressed: gfx.RGB24(0xdce2e5), toolbarFocus: gfx.RGB24(0x09836e), toolbarSeparator: gfx.RGB24(0x7d8a91),
	pin: gfx.RGB24(0x086957), bookmark: gfx.RGB24(0x2b68bb), mapViewport: gfx.RGB24(0xfff6f7), mapLine: gfx.RGB24(0x9ea5ac), invisible: gfx.RGB24(0x92969d),
	findMark:       gfx.RGBA(255, 0, 0, 100.0/255),
	tokenMarks:     [5]gfx.Color{gfx.RGB24(0x00ffff), gfx.RGB24(0xff8000), gfx.RGB24(0xffff00), gfx.RGB24(0x8000ff), gfx.RGB24(0x008000)},
	changeReverted: gfx.RGB24(0x40a0bf), changeSaved: gfx.RGB24(0x00a000), changeModified: gfx.RGB24(0xff8000), changeRevertedModified: gfx.RGB24(0xa0c000),
	diff: [3]gfx.Color{gfx.RGB24(0xbb3645), gfx.RGB24(0x237d4a), gfx.RGB24(0x946200)},
}

var modernPalette = themePalette{
	menuBar: gfx.RGB24(0x27282c), toolbarBar: gfx.RGB24(0x2b2c30), toolbar: gfx.RGB24(0x303238),
	tabs: gfx.RGB24(0x242529), tab: gfx.RGB24(0x2c2e33), activeTab: gfx.RGB24(0x1e1f22),
	dock: gfx.RGB24(0x24252a), mapBackground: gfx.RGB24(0x23252a), statusBar: gfx.RGB24(0x27282c), popup: gfx.RGB24(0x292b31),
	border: gfx.RGB24(0x4d505a), hover: gfx.RGB24(0x3b3d46), text: gfx.RGB24(0xe4e6eb), muted: gfx.RGB24(0xa6adb9),
	gutter: gfx.RGB24(0x292b31), accent: gfx.RGB24(0x9abfff), selectedRow: gfx.RGB24(0x333d50), inputBackground: gfx.RGB24(0x202126), invalidInput: gfx.RGB24(0x502e39),
	editorBackground: gfx.RGB24(0x1e1f22), editorForeground: gfx.RGB24(0xe5e7eb), gutterForeground: gfx.RGB24(0xa2aab8),
	selection: gfx.RGB24(0x345785), selectionText: gfx.RGB24(0xffffff),
	toolbarInk: gfx.RGB24(0xc8d0dc), toolbarDisabled: gfx.RGB24(0x777e8b), toolbarSelected: gfx.RGB24(0x28453d),
	toolbarHover: gfx.RGB24(0x393d47), toolbarPressed: gfx.RGB24(0x444a57), toolbarFocus: gfx.RGB24(0x7eddbd), toolbarSeparator: gfx.RGB24(0x535a67),
	pin: gfx.RGB24(0x7eddbd), bookmark: gfx.RGB24(0x87baff), mapViewport: gfx.RGB24(0x303b53), mapLine: gfx.RGB24(0x788394), invisible: gfx.RGB24(0x8890a0),
	findMark: gfx.RGB24(0x512731), findMatch: gfx.RGB24(0x4b371d),
	tokenMarks:     [5]gfx.Color{gfx.RGB24(0x23444a), gfx.RGB24(0x54321c), gfx.RGB24(0x45401d), gfx.RGB24(0x423055), gfx.RGB24(0x274330)},
	changeReverted: gfx.RGB24(0x66c5e2), changeSaved: gfx.RGB24(0x63c77b), changeModified: gfx.RGB24(0xffa662), changeRevertedModified: gfx.RGB24(0xb8cd58),
	diff: [3]gfx.Color{gfx.RGB24(0xff7686), gfx.RGB24(0x65d6a3), gfx.RGB24(0xf6c568)},
}

var monokaiPalette = func() themePalette {
	palette := modernPalette
	palette.menuBar, palette.statusBar = gfx.RGB24(0x30312b), gfx.RGB24(0x30312b)
	palette.toolbarBar, palette.toolbar = gfx.RGB24(0x32332c), gfx.RGB24(0x383a31)
	palette.tabs, palette.tab, palette.activeTab = gfx.RGB24(0x24251f), gfx.RGB24(0x32342b), gfx.RGB24(0x272822)
	palette.dock, palette.mapBackground, palette.popup = gfx.RGB24(0x2d2e27), gfx.RGB24(0x2d2e27), gfx.RGB24(0x313229)
	palette.inputBackground = gfx.RGB24(0x26271f)
	palette.text, palette.muted, palette.border, palette.hover = gfx.RGB24(0xf8f8f2), gfx.RGB24(0xb6b5a7), gfx.RGB24(0x53554a), gfx.RGB24(0x424438)
	palette.gutter, palette.gutterForeground, palette.invisible = gfx.RGB24(0x34352d), gfx.RGB24(0xb0ae9b), gfx.RGB24(0x9b9989)
	palette.accent, palette.selectedRow, palette.mapViewport = gfx.RGB24(0xc1e872), gfx.RGB24(0x3e4531), gfx.RGB24(0x3c442e)
	palette.editorBackground, palette.editorForeground = gfx.RGB24(0x272822), gfx.RGB24(0xf8f8f2)
	palette.selection = gfx.RGB24(0x515642)
	palette.diff = [3]gfx.Color{gfx.RGB24(0xf96a9a), gfx.RGB24(0xa6e22e), gfx.RGB24(0xe6db74)}
	return palette
}()

type themeOption struct {
	choice preferences.Theme
	label  string
}

func (option themeOption) command() string { return "Theme: " + option.label }

var themeOptions = [...]themeOption{
	{preferences.ClassicLight, "Classic Light"},
	{preferences.ModernDark, "Modern Dark"},
	{preferences.Monokai, "Monokai"},
	{preferences.FollowSystem, "Follow System"},
}

const themeMenuCommand = "Theme"

var (
	themeMenuScope        = tree.Sub(page, tree.Name("view-theme-menu"))
	preferencesThemeScope = tree.Sub(page, tree.Name("preferences-theme"))
)

func (state *model) beginThemeFrame() {
	choice := state.themeChoice.Normalized()
	if choice != state.nativeThemeChoice {
		state.nativeThemeChoice = choice
		state.unwatchAppearance()
		switch choice {
		case preferences.FollowSystem:
			app.SetNativeAppearance(app.AppearanceUnknown)
			state.stopAppearanceWatch = app.WatchSystemAppearance(state.invalidator.Invalidate)
		case preferences.ModernDark, preferences.Monokai:
			app.SetNativeAppearance(app.AppearanceDark)
		default:
			app.SetNativeAppearance(app.AppearanceLight)
		}
	}
	resolved := syntax.ClassicLight
	switch choice {
	case preferences.ModernDark:
		resolved = syntax.ModernDark
	case preferences.Monokai:
		resolved = syntax.Monokai
	case preferences.FollowSystem:
		if app.SystemAppearance() == app.AppearanceDark {
			resolved = syntax.ModernDark
		}
	}
	if resolved != state.renderTheme {
		state.renderTheme = resolved
		clear(state.changeView)
	}
}

func (state *model) unwatchAppearance() {
	if state.stopAppearanceWatch != nil {
		state.stopAppearanceWatch()
		state.stopAppearanceWatch = nil
	}
}

func (state *model) palette() *themePalette {
	switch state.renderTheme {
	case syntax.ModernDark:
		return &modernPalette
	case syntax.Monokai:
		return &monokaiPalette
	default:
		return &classicPalette
	}
}

func (state *model) uiTheme() gfx.Theme {
	if state.renderTheme == syntax.ClassicLight {
		return ui.LightTheme()
	}
	palette := state.palette()
	theme := ui.DarkTheme()
	theme.Background, theme.Foreground = palette.inputBackground, palette.text
	theme.Card, theme.CardForeground, theme.CardElevated = palette.popup, palette.text, palette.toolbar
	theme.Popover, theme.PopoverForeground = palette.popup, palette.text
	theme.Border, theme.Hover, theme.Ring = palette.border, palette.hover, palette.accent
	theme.Muted, theme.MutedForeground = palette.dock, palette.muted
	theme.Input, theme.Primary, theme.PrimaryForeground = palette.border, palette.accent, palette.editorBackground
	theme.Selection, theme.SelectionForeground = palette.selection, palette.selectionText
	return theme
}

func (state *model) editorTheme() gfx.Theme {
	palette := state.palette()
	theme := state.uiTheme()
	theme.Background, theme.Card, theme.Foreground = palette.editorBackground, palette.editorBackground, palette.editorForeground
	theme.Border, theme.Ring, theme.Input, theme.Primary = palette.editorBackground, palette.editorBackground, palette.editorBackground, palette.editorBackground
	theme.MutedForeground = palette.gutterForeground
	theme.Selection, theme.SelectionForeground = palette.selection, palette.selectionText
	return theme
}

func (state *model) themeCommand(command string) bool {
	if command == themeMenuCommand {
		state.productivityMenu = ""
		state.menuOpen, state.themeMenuOpen = 3, !state.themeMenuOpen
		state.symbolMenuOpen, state.viewTabMenuOpen, state.foldMenuOpen, state.namedBrowserMenuOpen = false, false, false, false
		state.invalidator.Invalidate()
		return true
	}
	for _, option := range themeOptions {
		if command != option.command() {
			continue
		}
		state.menuOpen, state.themeMenuOpen, state.menuOverflowOpen = -1, false, false
		if !state.preferencesOpen {
			state.requestEditorCommandFocus()
		}
		if state.themeChoice.Normalized() != option.choice {
			state.themeChoice = option.choice
			_ = state.savePreferences()
		}
		state.invalidator.Invalidate()
		return true
	}
	return false
}

func (state *model) paintThemeMenu(canvas *gfx.Canvas, size gfx.Size, parentRight, anchorY float32) {
	palette := state.palette()
	const rowHeight, width = 26, 168
	height := float32(len(themeOptions)*rowHeight + 8)
	x := min(parentRight, size.Width-width-4)
	y := max(menuHeight, min(anchorY, size.Height-statusHeight-height-4))
	canvas.PushLayer(3)
	canvas.PushOverlayClip(gfx.Rect{Width: size.Width, Height: size.Height})
	canvas.Border(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 1, palette.border, palette.popup)
	for index, option := range themeOptions {
		bounds := gfx.Rect{X: x + 4, Y: y + 4 + float32(index)*rowHeight, Width: width - 8, Height: rowHeight}
		id := themeMenuScope.At(index)
		checked := state.themeChoice.Normalized() == option.choice
		if canvas.Hovered() == id {
			canvas.Fill(bounds, palette.hover)
		}
		if checked {
			canvas.DrawIcon(checkIcon, gfx.Rect{X: bounds.X + 4, Y: bounds.Y + 6, Width: 14, Height: 14}, palette.accent)
		}
		smallLabel(canvas, option.label, gfx.Rect{X: bounds.X + 25, Y: bounds.Y, Width: bounds.Width - 28, Height: rowHeight}, palette.text, 11)
		canvas.HitWithOptions(id, bounds, option.label, gfx.HitOptions{Role: gfx.RoleMenuItem, Checked: checked})
		if canvas.Clicked(id) {
			state.themeCommand(option.command())
		}
	}
	canvas.PopOverlayClip()
	canvas.PopLayer()
}
