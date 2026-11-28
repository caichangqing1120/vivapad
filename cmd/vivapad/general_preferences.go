package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
	"golang.org/x/text/language"
	"vivapad/internal/external"
	"vivapad/internal/preferences"
)

const (
	preferencesEditingPage = iota
	preferencesViewPage
	preferencesPrintPage
	preferencesPrintBandsPage
	preferencesPrintFontPage
	preferencesSymbolsPage
	preferencesGeneralPage
	preferencesAboutPage
)

var (
	preferencesGeneralID      = tree.ID(page, "preferences-general")
	preferencesAboutID        = tree.ID(page, "preferences-about-updates")
	interfaceLanguageScope    = tree.Sub(page, tree.Name("preferences-interface-language"))
	interfaceSizeScope        = tree.Sub(page, tree.Name("preferences-interface-size"))
	tabNumbersID              = tree.ID(page, "preferences-tab-numbers")
	autoCheckUpdatesID        = tree.ID(page, "preferences-auto-check-updates")
	openConfigFolderID        = tree.ID(page, "preferences-open-config-folder")
	resetPreferencesID        = tree.ID(page, "preferences-reset")
	confirmResetPreferencesID = tree.ID(page, "preferences-confirm-reset")
	cancelResetPreferencesID  = tree.ID(page, "preferences-cancel-reset")
)

func (state *model) resolveInterfaceLanguage() {
	state.chineseUI = state.language == "zh-CN"
	if (state.language != "system" && state.language != "") || state.systemLanguages == nil {
		return
	}
	for _, preferred := range state.systemLanguages() {
		base, _ := language.Make(preferred).Base()
		switch base.String() {
		case "zh":
			state.chineseUI = true
			return
		case "en":
			return
		}
	}
}

func (state *model) tr(english, chinese string) string {
	if state.chineseUI {
		return chinese
	}
	return english
}

func (state *model) interfaceSize(base float32) float32 {
	switch state.uiTextSize {
	case "small":
		return base * .9
	case "large":
		return base * 1.18
	default:
		return base
	}
}

func (state *model) interfaceLabel(canvas *gfx.Canvas, value string, bounds gfx.Rect, color gfx.Color, size float32) {
	size = state.interfaceSize(size)
	smallLabel(canvas, canvas.FitLine(value, size, gfx.WeightRegular, bounds.Width), bounds, color, size)
}

var preferenceChinese = map[string]string{
	"Editing": "编辑", "View": "视图", "Print": "打印", "Header/Footer": "页眉和页脚", "Print Fonts": "打印字体", "Symbols": "符号",
	"Tab width": "制表符宽度", "Indent using": "缩进方式", "Language": "按文档语言", "Tabs": "制表符", "Spaces": "空格",
	"Auto Indent": "自动缩进", "Pair Brackets": "自动配对括号", "Theme": "主题", "Classic Light": "经典浅色", "Modern Dark": "现代深色", "Follow System": "跟随系统",
	"Line Numbers": "行号", "Word Wrap": "自动换行", "Show Space and Tab": "显示空格和制表符", "Show End of Line": "显示行尾符", "Show Indent Guide": "显示缩进参考线", "Synchronize Zoom Across Views": "同步视图缩放",
	"Non-printing style": "不可打印字符样式", "Original": "原样", "Abbreviation": "缩写", "Codepoint": "码位", "Include control / Unicode EOL": "包含控制字符和 Unicode 行尾符",
	"Color options": "颜色选项", "Margins (mm)": "页边距（毫米）", "Header and Footer": "页眉和页脚", "Header": "页眉", "Footer": "页脚", "Left": "左", "Middle": "中", "Right": "右", "Top": "上", "Bottom": "下", "Insert": "插入", "Size": "字号",
	"WYSIWYG": "所见即所得", "Invert": "反色", "Black on white": "黑字白底", "No background": "无背景", "Print line numbers": "打印行号", "Form feed as page break": "将换页符视为分页",
	"Preferences...": "偏好设置…", checkUpdatesCommand: "检查更新…", "Settings": "设置", batchConvertCommand: "批量转换…",
	"Documents": "文档", "Files": "文件", "Workspace": "工作区", "Folder": "文件夹", "Folder as Workspace": "文件夹工作区", "Functions": "函数", "Funcs": "函数", "Characters": "字符", "Chars": "字符", "Projects": "项目", "No projects": "没有项目", "Function List": "函数列表", "Document List": "文档列表",
	"No search results": "没有搜索结果", "Search canceled": "搜索已取消", "Read-only document": "只读文档", "READ ONLY": "只读", "Lightweight Edit": "轻量编辑", "Lightweight · Read-Only": "轻量编辑 · 只读", "Large File (Read-Only)": "大文件（只读）",
	"New": "新建", "Open": "打开", "Save": "保存", "Undo": "撤销", "Redo": "重做", "Cut": "剪切", "Copy": "复制", "Paste": "粘贴", "Find": "查找",
}

func (state *model) preferenceText(label string) string {
	if state.chineseUI {
		if translated := preferenceChinese[label]; translated != "" {
			return translated
		}
	}
	return label
}

func (state *model) preferenceLabel(canvas *gfx.Canvas, label string, bounds gfx.Rect, color gfx.Color, size float32) {
	state.interfaceLabel(canvas, state.preferenceText(label), bounds, color, size)
}

func (state *model) statusMessage(message string) string {
	if state.chineseUI {
		if path, ok := strings.CutPrefix(message, "Saved "); ok {
			return "已保存 " + path
		}
	}
	return state.preferenceText(message)
}

func (state *model) numberedTabName(name string, index int) string {
	if state.showTabNumbers && index < 9 {
		return strconv.Itoa(index+1) + "  " + name
	}
	return name
}

func (state *model) selectNumberedTab(index int, target gfx.ID) gfx.EventResult {
	pane := state.activePane
	if target == editorID || target == rightEditorID {
		pane = paneForTarget(target)
	}
	docs := state.space.Documents
	if state.splitEnabled {
		docs = state.paneTabs(pane)
	}
	if index >= len(docs) {
		return gfx.EventResult{Handled: true}
	}
	state.activatePane(pane)
	state.space.Select(state.documentIndex(docs[index]))
	state.paneDocs[pane] = docs[index]
	state.menuOpen = -1
	return gfx.EventResult{Handled: true, Repaint: true, Focus: state.commandEditorFocusID()}
}

func (state *model) escapePreferences() {
	if state.resetPreferencesPending {
		state.resetPreferencesPending = false
		return
	}
	state.closePreferences()
}

func (state *model) paintGeneralPreferences(canvas *gfx.Canvas, x, y, width float32) {
	palette := state.palette()
	row := func(top, height float32) gfx.Rect { return gfx.Rect{X: x, Y: y + top, Width: width, Height: height} }
	state.interfaceLabel(canvas, state.tr("General", "常规"), row(60, 27), palette.text, 15)
	if state.resetPreferencesPending {
		state.interfaceLabel(canvas, state.tr("Restore default preferences?", "恢复默认设置？"), row(98, 26), palette.text, 13)
		state.interfaceLabel(canvas, state.tr("Documents and shortcuts are not changed.", "不会更改文档和快捷键。"), row(128, 24), palette.muted, 11)
		if state.settingsButton(canvas, confirmResetPreferencesID, gfx.Rect{X: x, Y: y + 170, Width: 146, Height: 29}, state.tr("Restore Defaults", "恢复默认设置"), true) {
			state.resetPreferences()
		}
		if state.settingsButton(canvas, cancelResetPreferencesID, gfx.Rect{X: x + 154, Y: y + 170, Width: 90, Height: 29}, state.tr("Cancel", "取消"), false) {
			state.resetPreferencesPending = false
		}
		return
	}
	state.interfaceLabel(canvas, state.tr("Interface language", "界面语言"), row(90, 20), palette.muted, 11)
	state.interfaceChoices(canvas, row(112, 26), interfaceLanguageScope, []string{state.tr("Follow System", "跟随系统"), "简体中文", "English"}, []string{"system", "zh-CN", "en"}, &state.language)
	state.interfaceLabel(canvas, state.tr("Interface text size", "界面文字大小"), row(145, 20), palette.muted, 11)
	state.interfaceChoices(canvas, row(168, 26), interfaceSizeScope, []string{state.tr("Small", "小"), state.tr("Medium", "中"), state.tr("Large", "大")}, []string{"small", "medium", "large"}, &state.uiTextSize)
	state.generalCheckbox(canvas, tabNumbersID, row(198, 23), state.tr("Show tab numbers", "显示标签序号"), &state.showTabNumbers)
	state.generalCheckbox(canvas, autoCheckUpdatesID, row(222, 23), state.tr("Check for updates on startup", "启动时检查更新"), &state.autoCheckUpdates)
	buttonWidth := min(float32(168), (width-8)/2)
	if state.settingsButton(canvas, openConfigFolderID, gfx.Rect{X: x, Y: y + 250, Width: buttonWidth, Height: 29}, state.tr("Open Config Folder", "打开配置目录"), false) {
		state.openConfigFolder()
	}
	if state.settingsButton(canvas, resetPreferencesID, gfx.Rect{X: x + buttonWidth + 8, Y: y + 250, Width: buttonWidth, Height: 29}, state.tr("Reset Defaults", "恢复默认设置"), false) {
		state.resetPreferencesPending = true
	}
}

func (state *model) interfaceChoices(canvas *gfx.Canvas, bounds gfx.Rect, scope gfx.Scope, labels, values []string, current *string) {
	segment := bounds.Width / float32(len(values))
	for index, value := range values {
		item := gfx.Rect{X: bounds.X + segment*float32(index), Y: bounds.Y, Width: segment - 4, Height: bounds.Height}
		if state.paintPreferenceChoice(canvas, scope.At(index), item, labels[index], labels[index], *current == value) {
			*current = value
			state.resolveInterfaceLanguage()
			state.saveGeneralPreferences()
		}
	}
}

func (state *model) generalCheckbox(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, label string, current *bool) {
	if state.paintPreferenceCheckbox(canvas, id, bounds, label, *current) {
		*current = !*current
		state.saveGeneralPreferences()
	}
}

func (state *model) saveGeneralPreferences() {
	state.preferencesNotice = ""
	if err := state.savePreferences(); err != nil {
		state.preferencesNotice = state.tr("Could not save preferences", "无法保存设置") + ": " + err.Error()
	}
}

func (state *model) resetPreferences() {
	state.resetPreferencesPending = false
	defaults := preferences.Default()
	if state.preferencesPath == "" {
		state.preferencesNotice = state.tr("Settings storage is unavailable", "设置目录不可用")
		return
	}
	if err := preferences.Save(state.preferencesPath, defaults); err != nil {
		state.preferencesNotice = state.tr("Could not reset preferences", "无法恢复默认设置") + ": " + err.Error()
		return
	}
	state.applyPreferences(defaults)
	state.openPrintPreferenceFields()
	state.preferencesNotice = state.tr("Default preferences restored", "已恢复默认设置")
}

func (state *model) openConfigFolder() {
	if state.preferencesPath == "" {
		state.preferencesNotice = state.tr("Settings storage is unavailable", "设置目录不可用")
		return
	}
	path := filepath.Dir(state.preferencesPath)
	err := os.MkdirAll(path, 0700)
	if err == nil {
		err = external.OpenFolder(path)
	}
	state.preferencesNotice = ""
	if err != nil {
		state.preferencesNotice = state.tr("Could not open configuration folder", "无法打开配置目录") + ": " + err.Error()
	}
}

func (state *model) settingsButton(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, label string, emphasis bool) bool {
	palette := state.palette()
	border, background, ink := palette.border, palette.toolbar, palette.text
	if emphasis {
		border, background, ink = palette.accent, palette.selectedRow, palette.accent
	}
	if canvas.Hovered() == id || canvas.FocusVisible() == id {
		border, background = palette.accent, palette.hover
	}
	canvas.RoundedBorder(bounds, 5, 1, border, background)
	size := state.interfaceSize(11)
	canvas.LabelCenter(canvas.FitLine(label, size, gfx.WeightRegular, bounds.Width-10), bounds, gfx.TextStyle{Size: size, Color: ink})
	return hitButton(canvas, id, bounds, label)
}
