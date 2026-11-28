package main

import (
	"os"
	"path/filepath"
	"slices"
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
	// Top menu bar
	"File": "文件", "Edit": "编辑", "Search": "搜索", "View": "视图", "Encoding": "编码",
	"Language": "语言", "Settings": "设置", "Tools": "工具", "Macro": "宏", "Run": "运行",
	"Plugins": "插件", "Window": "窗口", "?": "帮助",

	// File menu
	"New": "新建", "Open": "打开", "Open...": "打开…", "Save": "保存", "Save As...": "另存为…",
	"Save a Copy As...": "保存副本为…", "Save All": "全部保存", "Close": "关闭",
	"Open as GB18030...": "以 GB18030 打开…", "Open as Windows-1252...": "以 Windows-1252 打开…",
	"Reload from Disk": "从磁盘重新加载", "Recent Files": "最近打开的文件", "Close All": "全部关闭",
	"Close Other Tabs": "关闭其他标签页", "Sessions": "会话", "Rename...": "重命名…",
	"Close Multiple Documents": "批量关闭文档", "Open Folder as Workspace...": "打开文件夹作为工作区…",
	"Open in Default Viewer": "在默认查看器中打开", "Open Containing Folder": "打开所在文件夹",
	"Open as Encoding...": "指定编码打开…", "Move to Recycle Bin": "移至回收站",
	"Print...": "打印…", "Print Now": "立即打印", newWindowCommand: "在新窗口中打开", "Exit": "退出",

	// Edit menu
	"Undo": "撤销", "Redo": "重做", "Cut": "剪切", "Copy": "复制", "Paste": "粘贴",
	"Select All": "全选", "Duplicate Line": "复制当前行", "Delete Line": "删除当前行",
	"Indent Lines": "增加缩进", "Unindent Lines": "减少缩进", "Line Operations": "行操作",
	"Convert Case": "转换大小写", "Blank Operations": "空白字符操作", "Copy to Clipboard": "复制到剪贴板",
	"Delete": "删除", "Begin/End Select": "开始/结束选择", "Read-Only in Notepad++": "只读（编辑器内）",
	"Auto-Completion": "自动补全", "Comment/Uncomment": "注释/取消注释", "Insert": "插入",
	"EOL Conversion": "行尾符转换", "Column Editor...": "列块编辑…",
	"Begin/End Select in Column Mode": "开始/结束列模式选择", columnModeHelpCommand: "列模式帮助",
	"On Selection": "基于选区", "Multi-select All": "全选相同匹配", "Multi-select Next": "选中下一个匹配",
	"Undo Latest Multi-Select": "撤销上次多选", "Skip Current Multi-Select": "跳过当前匹配",
	pasteSpecialCommand: "选择性粘贴", characterPanelCommand: "字符面板",

	// Search menu
	"Find": "查找", "Find...": "查找…", "Find in Files...": "在文件中查找…", "Find Next": "查找下一个",
	"Find Previous": "查找上一个", "Selected Text Search": "搜索选中文本", "Replace...": "替换…",
	"Incremental Search": "增量搜索", "Search Results Window": "搜索结果窗口",
	"Next Search Result": "下一个搜索结果", "Previous Search Result": "上一个搜索结果",
	"Go to Line...": "跳转到行…", "Go to Matching Brace": "跳转到匹配括号",
	"Select All In-between {} [] or ()": "选中括号内全部内容", "Mark...": "标记…",
	"Change History": "修改历史", "Style All Occurrences of Token": "高亮所有相同词元",
	"Style One Token": "高亮当前词元", "Clear Style": "清除高亮样式",
	"Jump Up": "向上跳转", "Jump Down": "向下跳转", "Copy Styled Text": "复制高亮文本",
	"Bookmark": "书签", "Find characters in range...": "按字符范围查找…",
	"Replace in Files...": "在文件中替换…", "Find All in Current Document": "在当前文档中查找全部",
	"Find All in Open Documents": "在所有打开文档中查找", "Replace All in Open Documents": "在所有打开文档中替换",
	"Clear Marks": "清除标记", "Next Change": "下一处修改", "Previous Change": "上一处修改",
	"Clear Change History": "清除修改历史",

	// View menu
	"Show Line Numbers": "显示行号", "Word Wrap": "自动换行", "Zoom In": "放大", "Zoom Out": "缩小",
	"Restore Default Zoom": "恢复默认缩放", "Split View": "分屏视图", "Move to Other View": "移至另一视图",
	"Clone to Other View": "克隆到另一视图", "Folder as Workspace": "文件夹工作区", "Show Symbol": "显示符号",
	"Tab": "标签页", "Fold All": "全部折叠", "Unfold All": "全部展开",
	"Fold Current Level": "折叠当前层级", "Unfold Current Level": "展开当前层级",
	"Fold Level": "按层级折叠", "Unfold Level": "按层级展开", "Document Map": "文档缩略图",
	"Function List": "函数列表", "Document List": "文档列表",
	"Synchronize Vertical Scrolling": "同步垂直滚动", "Synchronize Horizontal Scrolling": "同步水平滚动",
	textDirectionRTL: "文本方向从右到左", textDirectionLTR: "文本方向从左到右",
	"Summary...": "文档摘要…", "Monitoring (tail -f)": "实时监控 (tail -f)",
	"Focus on Another View": "切换至另一视图", "Hide Lines": "隐藏行", "Always on Top": "窗口置顶",
	"Toggle Full Screen Mode": "切换全屏模式", "Synchronize Zoom Across Views": "同步视图缩放",
	distractionFreeCommand: "专注模式", namedBrowserMenuCommand: "在浏览器中查看",
	postItCommand: "便签模式", compareMenu: "文件对比", projectPanelCommand: "项目面板",
	themeMenuCommand: "界面主题",

	// Encoding & Language menus
	"Character sets": "字符集", batchConvertCommand: "批量转换…", "Normal Text": "纯文本",

	// Settings menu
	"Preferences...": "偏好设置…", "Tab Width: 2": "制表符宽度：2", "Tab Width: 4": "制表符宽度：4",
	"Command Palette": "命令面板", "Quick File Switcher": "快速切换文件",
	"Tab Width: 8": "制表符宽度：8", "Indent: Language Default": "缩进：按语言默认",
	"Indent: Tabs": "缩进：制表符", "Indent: Spaces": "缩进：空格",
	"Auto Indent": "自动缩进", "Pair Brackets": "自动配对括号", "Date Time Format...": "日期时间格式…",
	"Change History: Margin": "修改历史：边栏显示", "Change History: Indicator": "修改历史：指示器显示",
	"Change History: Both": "修改历史：全部显示", "Change History: Off": "修改历史：关闭",
	shortcutMapperCommand: "快捷键管理…", styleConfiguratorCommand: "配色与样式配置…",

	// Tools, Macro, Run, Window, Help menus
	structuredToolsMenu: "JSON / XML 工具", encodingToolsMenu: "编码与时间戳工具",
	"Record Macro": "录制宏", "Stop Recording": "停止录制", "Playback Macro": "回放宏",
	"Save Current Recorded Macro...": "保存当前录制的宏…", "Run a Macro Multiple Times...": "多次运行宏…",
	"Playback 5 Times": "回放 5 次", "Clear Macro": "清除宏", "Saved Macros": "已保存的宏",
	"Run...": "运行命令…", "Sort By": "排序方式", "Windows...": "窗口列表…", "Recent Window": "最近窗口",
	aboutHelpCommand: "关于 VivaPad", debugHelpCommand: "调试信息…",
	projectPageHelpCommand: "项目主页", userGuideHelpCommand: "使用指南",
	commandLineHelpCommand: "命令行参数", checkUpdatesCommand: "检查更新…",

	// Preferences & dock panels
	"General": "通用", "Editing": "编辑", "Print": "打印", "Header/Footer": "页眉和页脚", "Print Fonts": "打印字体", "Symbols": "符号", "About & Updates": "关于与更新",
	"Tab width": "制表符宽度", "Indent using": "缩进方式", "Tabs": "制表符", "Spaces": "空格",
	"Classic Light": "经典浅色", "Modern Dark": "现代深色", "Follow System": "跟随系统",
	"Line Numbers": "行号", "Show Space and Tab": "显示空格和制表符", "Show End of Line": "显示行尾符", "Show Indent Guide": "显示缩进参考线",
	"Non-printing style": "不可打印字符样式", "Original": "原样", "Abbreviation": "缩写", "Codepoint": "码位", "Include control / Unicode EOL": "包含控制字符和 Unicode 行尾符",
	"Color options": "颜色选项", "Margins (mm)": "页边距（毫米）", "Header and Footer": "页眉和页脚", "Header": "页眉", "Footer": "页脚", "Left": "左", "Middle": "中", "Right": "右", "Top": "上", "Bottom": "下", "Size": "字号",
	"WYSIWYG": "所见即所得", "Invert": "反色", "Black on white": "黑字白底", "No background": "无背景", "Print line numbers": "打印行号", "Form feed as page break": "将换页符视为分页",
	"Full file path": "完整文件路径", "File name": "文件名", "File directory": "文件所在目录", "Page": "页码", "Short date": "短日期", "Long date": "长日期", "Time": "时间",
	"Documents": "文档", "Files": "文件", "Workspace": "工作区", "Folder": "文件夹", "Functions": "函数", "Funcs": "函数", "Characters": "字符", "Chars": "字符", "No projects": "没有项目",
	"No search results": "没有搜索结果", "Search canceled": "搜索已取消", "Read-only document": "只读文档", "READ ONLY": "只读", "Lightweight Edit": "轻量编辑", "Lightweight · Read-Only": "轻量编辑 · 只读", "Large File (Read-Only)": "大文件（只读）",
}

func (state *model) preferenceText(label string) string {
	if state.chineseUI {
		if label == fileReadOnlyLabel() {
			return "系统只读属性"
		}
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
		if rest, ok := strings.CutPrefix(message, "Recovered "); ok {
			if count, ok := strings.CutSuffix(rest, " unsaved document(s)"); ok {
				return "已恢复 " + count + " 个未保存文档"
			}
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

func (state *model) paintGeneralPreferences(canvas *gfx.Canvas, body gfx.Rect, height float32) {
	palette := state.palette()
	if state.resetPreferencesPending {
		card := state.settingsCard(canvas, body, 64+height)
		message := card.row(canvas, 0, 64)
		title := gfx.Rect{X: message.X, Y: message.Y + 10, Width: message.Width, Height: 24}
		state.settingsLabel(canvas, state.tr("Restore default preferences?", "恢复默认设置？"), title, palette.text, 13, gfx.WeightSemibold)
		title.Y += 24
		state.interfaceLabel(canvas, state.tr("Documents and shortcuts are not changed.", "不会更改文档和快捷键。"), title, palette.muted, 11)
		row := card.row(canvas, 64, height)
		confirmLabel := state.tr("Restore Defaults", "恢复默认设置")
		confirm := settingsControl(row, state.settingsButtonWidth(canvas, confirmLabel), 26)
		row.Width -= confirm.Width + 8
		if state.settingsButton(canvas, confirmResetPreferencesID, confirm, confirmLabel, true) {
			state.resetPreferences()
		}
		if state.settingsButton(canvas, cancelResetPreferencesID, settingsControl(row, 80, 26), state.tr("Cancel", "取消"), false) {
			state.resetPreferencesPending = false
		}
		return
	}
	card := state.settingsCard(canvas, body, height*4)
	state.interfaceChoices(canvas, card.row(canvas, 0, height), state.tr("Interface language", "界面语言"), interfaceLanguageScope,
		[]string{state.tr("Follow System", "跟随系统"), "简体中文", "English"}, []string{"system", "zh-CN", "en"}, &state.language)
	state.interfaceChoices(canvas, card.row(canvas, height, height), state.tr("Interface text size", "界面文字大小"), interfaceSizeScope,
		[]string{state.tr("Small", "小"), state.tr("Medium", "中"), state.tr("Large", "大")}, []string{"small", "medium", "large"}, &state.uiTextSize)
	state.generalToggle(canvas, tabNumbersID, card.row(canvas, height*2, height), state.tr("Show tab numbers", "显示标签序号"), &state.showTabNumbers)
	state.generalToggle(canvas, autoCheckUpdatesID, card.row(canvas, height*3, height), state.tr("Check for updates on startup", "启动时检查更新"), &state.autoCheckUpdates)
	body.Y += height*4 + settingsCardGap
	card = state.settingsCard(canvas, body, height*2)
	if state.settingsAction(canvas, card.row(canvas, 0, height), state.tr("Configuration", "配置文件"), openConfigFolderID, state.tr("Open Config Folder", "打开配置目录")) {
		state.openConfigFolder()
	}
	if state.settingsAction(canvas, card.row(canvas, height, height), state.tr("Default preferences", "默认设置"), resetPreferencesID, state.tr("Reset Defaults", "恢复默认设置")) {
		state.resetPreferencesPending = true
	}
}

func (state *model) interfaceChoices(canvas *gfx.Canvas, row gfx.Rect, title string, scope gfx.Scope, labels, values []string, current *string) {
	if index := state.settingsSegments(canvas, row, title, scope, labels, labels, slices.Index(values, *current)); index >= 0 {
		*current = values[index]
		state.resolveInterfaceLanguage()
		state.saveGeneralPreferences()
	}
}

func (state *model) generalToggle(canvas *gfx.Canvas, id gfx.ID, row gfx.Rect, label string, current *bool) {
	if state.paintPreferenceToggle(canvas, id, row, label, label, *current) {
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
	return state.settingsButtonWithLabel(canvas, id, bounds, label, label, emphasis)
}

func (state *model) settingsButtonWidth(canvas *gfx.Canvas, label string) float32 {
	return max(28, canvas.TextWidthWeight(label, state.interfaceSize(11), gfx.WeightMedium)+20)
}

func (state *model) settingsAction(canvas *gfx.Canvas, row gfx.Rect, title string, id gfx.ID, label string) bool {
	control := settingsControl(row, state.settingsButtonWidth(canvas, label), 26)
	state.settingsRowLabel(canvas, title, row, control)
	return state.settingsButton(canvas, id, control, label, false)
}

func (state *model) settingsButtonWithLabel(canvas *gfx.Canvas, id gfx.ID, bounds gfx.Rect, text, label string, emphasis bool) bool {
	palette := state.palette()
	border, background, ink := palette.border, palette.toolbar, palette.text
	if emphasis {
		border, background, ink = palette.accent, palette.selectedRow, palette.accent
	}
	if canvas.Hovered() == id || canvas.FocusVisible() == id {
		border, background = palette.accent, palette.hover
	}
	canvas.RoundedBorder(bounds, bounds.Height/2, 1, border, background)
	size := state.interfaceSize(11)
	canvas.LabelCenter(canvas.FitLine(text, size, gfx.WeightMedium, bounds.Width-10), bounds, gfx.TextStyle{Size: size, Color: ink, Weight: gfx.WeightMedium})
	return hitButton(canvas, id, bounds, label)
}
