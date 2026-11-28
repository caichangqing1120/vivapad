package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/internal/privatejson"
	"vivapad/internal/syntax"
)

const udlManagerCommand = "User Defined Language..."
const udlLabelPrefix = "UDL: "

var (
	udlSaveID           = tree.ID(page, "udl-save")
	udlNewID            = tree.ID(page, "udl-new")
	udlPrevID           = tree.ID(page, "udl-previous")
	udlNextID           = tree.ID(page, "udl-next")
	udlDropID           = tree.ID(page, "udl-delete")
	udlPresetID         = tree.ID(page, "udl-preset")
	udlImportID         = tree.ID(page, "udl-import")
	udlExportID         = tree.ID(page, "udl-export")
	udlBackID           = tree.ID(page, "udl-back")
	udlChooseImportID   = tree.ID(page, "udl-choose-import")
	udlExportSelectedID = tree.ID(page, "udl-export-selected")
	udlExportAllID      = tree.ID(page, "udl-export-all")
	udlPresetScope      = tree.Sub(page, tree.Name("udl-presets"))
	udlPolicyScope      = tree.Sub(page, tree.Name("udl-policies"))
	udlPresets          = syntax.BuiltinUDLPresets()
)

type udlManagerAction uint8

const (
	udlEdit udlManagerAction = iota
	udlAddPreset
	udlImport
	udlExport
)

type udlManagerOptions struct {
	action udlManagerAction
	policy syntax.ConflictPolicy
}

type udlHighlightCache struct {
	value, mode string
	highlight   func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun
}

func (state *model) udlForMode(mode string) *syntax.UDL {
	if !strings.HasPrefix(mode, "udl:") {
		return nil
	}
	for index := range state.udls {
		if state.udls[index].Mode() == mode {
			return &state.udls[index]
		}
	}
	return nil
}

func (state *model) languageForCommand(command string) (string, bool) {
	if name, custom := strings.CutPrefix(command, udlLabelPrefix); custom {
		for _, definition := range state.udls {
			if definition.Name == name {
				return definition.Mode(), true
			}
		}
	}
	return languageExtension(command)
}

func (state *model) openUDLManager(index int) {
	var definition syntax.UDL
	if index >= 0 && index < len(state.udls) {
		definition = state.udls[index]
	} else {
		index = -1
	}
	state.openM5Dialog(udlManagerCommand, index, definition.Name, strings.Join(definition.Extensions, ", "),
		strings.Join(definition.Keywords, " "), definition.LineComment, definition.BlockStart, definition.BlockEnd, strings.Join(definition.Quotes, " "))
}

func (state *model) setUDLDefinitions(definitions []syntax.UDL, previous, next string) error {
	if err := syntax.SaveUDLs(state.udlPath, definitions); err != nil {
		return err
	}
	loaded, err := syntax.LoadUDLs(state.udlPath)
	if err != nil {
		return err
	}
	state.udls = loaded
	clear(state.udlHighlights)
	for _, doc := range state.space.Documents {
		if previous != "" && state.languages[doc] == previous {
			if next != "" {
				state.languages[doc] = next
			} else {
				delete(state.languages, doc)
			}
		}
		state.configureCode(doc)
	}
	state.invalidator.Invalidate()
	return nil
}

func (state *model) paintUDLManager(canvas *gfx.Canvas, bounds gfx.Rect, dialog *m5Dialog) {
	palette := state.palette()
	left, top, width := bounds.X, bounds.Y, bounds.Width
	bottom := top + bounds.Height - 29
	defer func() {
		if state.m5 == dialog {
			state.interfaceLabel(canvas, dialog.message, gfx.Rect{X: left, Y: bottom - 21, Width: width, Height: 20}, palette.diff[0], 11)
		}
	}()
	if dialog.udl.action != udlEdit {
		state.paintUDLActions(canvas, bounds, dialog)
		return
	}
	state.interfaceLabel(canvas, state.tr("User Defined Language", "用户自定义语言"), gfx.Rect{X: left, Y: top, Width: width, Height: 24}, palette.text, 15)
	row := gfx.Rect{X: left, Y: top + 31, Width: 30, Height: 26}
	if palette.m5Button(canvas, udlPrevID, row, "<", len(state.udls) == 0 || dialog.index == 0) {
		index := dialog.index - 1
		if dialog.index < 0 {
			index = len(state.udls) - 1
		}
		state.openUDLManager(index)
		return
	}
	row.X += 34
	if palette.m5Button(canvas, udlNextID, row, ">", dialog.index >= len(state.udls)-1) {
		state.openUDLManager(dialog.index + 1)
		return
	}
	row.X, row.Width = left+68, 52
	if palette.m5Button(canvas, udlNewID, row, state.tr("New", "新建"), dialog.index < 0) {
		state.openUDLManager(-1)
		return
	}
	name := state.tr("New language", "新建语言")
	if dialog.index >= 0 {
		name = fmt.Sprintf("%d / %d    %s", dialog.index+1, len(state.udls), state.udls[dialog.index].Name)
	}
	smallLabel(canvas, canvas.FitLine(name, 12, gfx.WeightRegular, width-134), gfx.Rect{X: left + 130, Y: row.Y, Width: width - 134, Height: row.Height}, palette.muted, 12)
	labels := [...]string{state.tr("Name", "名称"), state.tr("File extensions", "文件扩展名"), state.tr("Keywords", "关键字"),
		state.tr("Line comment", "行注释"), state.tr("Block start", "块注释开头"), state.tr("Block end", "块注释结尾"), state.tr("String quotes", "字符串引号")}
	placeholders := [...]string{"My language", "foo, bar", "if else return", "//", "/*", "*/", `" '`}
	for index, label := range labels {
		columnWidth := (width - 14) / 2
		x, y := left+float32(index%2)*(columnWidth+14), top+63+float32(index/2)*43
		smallLabel(canvas, label, gfx.Rect{X: x, Y: y, Width: columnWidth, Height: 16}, palette.muted, 11)
		state.paintM5Field(canvas, gfx.Rect{X: x, Y: y + 17, Width: columnWidth, Height: 23}, index, label, placeholders[index], false)
	}
	for index, control := range []struct {
		id       gfx.ID
		action   udlManagerAction
		label    string
		disabled bool
	}{
		{udlPresetID, udlAddPreset, state.tr("Add Preset…", "从预设添加…"), false},
		{udlImportID, udlImport, state.tr("Import JSON…", "导入 JSON…"), false},
		{udlExportID, udlExport, state.tr("Export JSON…", "导出 JSON…"), len(state.udls) == 0},
	} {
		buttonWidth := min(float32(134), (width-16)/3)
		button := gfx.Rect{X: left + float32(index)*(buttonWidth+8), Y: top + 236, Width: buttonWidth, Height: 26}
		if palette.m5Button(canvas, control.id, button, control.label, control.disabled) {
			dialog.udl.action, dialog.message = control.action, ""
			return
		}
	}
	if palette.m5Button(canvas, udlDropID, gfx.Rect{X: left, Y: bottom, Width: 74, Height: 29}, state.tr("Delete", "删除"), dialog.index < 0) {
		definitions := slices.Delete(slices.Clone(state.udls), dialog.index, dialog.index+1)
		if err := state.setUDLDefinitions(definitions, state.udls[dialog.index].Mode(), ""); err != nil {
			dialog.message = err.Error()
		} else {
			state.openUDLManager(min(dialog.index, len(definitions)-1))
		}
		return
	}
	if palette.m5Button(canvas, udlSaveID, gfx.Rect{X: left + width - 156, Y: bottom, Width: 74, Height: 29}, state.tr("Save", "保存"), false) {
		field := func(index int) string { return strings.TrimSpace(dialog.fields[index].String()) }
		definition := syntax.UDL{Name: field(0), Extensions: strings.FieldsFunc(field(1), func(value rune) bool { return unicode.IsSpace(value) || value == ',' }),
			Keywords: strings.Fields(field(2)), LineComment: field(3), BlockStart: field(4), BlockEnd: field(5), Quotes: strings.Fields(field(6))}
		definitions := slices.Clone(state.udls)
		previous := ""
		if dialog.index < 0 {
			definitions = append(definitions, definition)
		} else {
			previous = definitions[dialog.index].Mode()
			definitions[dialog.index] = definition
		}
		if err := state.setUDLDefinitions(definitions, previous, definition.Mode()); err != nil {
			dialog.message = err.Error()
		} else {
			state.notice = state.tr("User language saved", "已保存自定义语言")
			state.closeM5Dialog()
		}
	}
}

func (state *model) paintUDLActions(canvas *gfx.Canvas, bounds gfx.Rect, dialog *m5Dialog) {
	palette := state.palette()
	row := func(top, height float32) gfx.Rect {
		return gfx.Rect{X: bounds.X, Y: bounds.Y + top, Width: bounds.Width, Height: height}
	}
	switch dialog.udl.action {
	case udlAddPreset:
		state.interfaceLabel(canvas, state.tr("Add Preset", "从预设添加"), row(0, 24), palette.text, 15)
		for index, preset := range udlPresets {
			item := row(44+float32(index/2)*38, 30)
			item.Width = (item.Width - 10) / 2
			item.X += float32(index%2) * (item.Width + 10)
			if palette.m5Button(canvas, udlPresetScope.At(index), item, preset.Name, false) {
				state.addUDLPreset(dialog, preset)
				return
			}
		}
	case udlImport:
		state.interfaceLabel(canvas, state.tr("Import JSON", "导入 JSON"), row(0, 24), palette.text, 15)
		state.interfaceLabel(canvas, state.tr("Same-name languages", "同名语言"), row(40, 22), palette.muted, 12)
		for index, label := range []string{state.tr("Skip", "跳过"), state.tr("Overwrite", "覆盖"), state.tr("Rename", "重命名")} {
			item := row(66, 29)
			item.Width = (item.Width - 16) / 3
			item.X += float32(index) * (item.Width + 8)
			policy := syntax.ConflictPolicy(index)
			if state.paintPreferenceChoice(canvas, udlPolicyScope.At(index), item, label, label, dialog.udl.policy == policy) {
				dialog.udl.policy = policy
			}
		}
		choose := row(112, 29)
		choose.Width = min(choose.Width, 150)
		if palette.m5Button(canvas, udlChooseImportID, choose, state.tr("Choose JSON File…", "选择 JSON 文件…"), false) {
			state.importUDLJSON(dialog)
			return
		}
	case udlExport:
		state.interfaceLabel(canvas, state.tr("Export JSON", "导出 JSON"), row(0, 24), palette.text, 15)
		for index, option := range []struct {
			id       gfx.ID
			label    string
			disabled bool
		}{
			{udlExportSelectedID, state.tr("Selected Saved Language…", "选中的已保存语言…"), dialog.index < 0},
			{udlExportAllID, state.tr("All Saved Languages…", "全部已保存语言…"), false},
		} {
			if palette.m5Button(canvas, option.id, row(44+float32(index)*38, 30), option.label, option.disabled) {
				state.exportUDLJSON(dialog, index == 1)
				return
			}
		}
	}
	back := row(bounds.Height-29, 29)
	back.Width = 74
	if palette.m5Button(canvas, udlBackID, back, state.tr("Back", "返回"), false) {
		dialog.udl.action, dialog.message, dialog.focus = udlEdit, "", true
	}
}

func (state *model) udlImportReport(report syntax.MergeReport) string {
	message := fmt.Sprintf(state.tr("Imported %d · Skipped %d · Overwritten %d · Renamed %d", "导入 %d · 跳过 %d · 覆盖 %d · 重命名 %d"),
		report.Imported, report.Skipped, report.Overwritten, report.Renamed)
	if report.UnassignedExtensions > 0 {
		message += fmt.Sprintf(state.tr(" · %d extensions kept on existing languages", " · %d 个扩展名仍归原语言"), report.UnassignedExtensions)
	}
	return message
}

func (state *model) addUDLPreset(dialog *m5Dialog, preset syntax.UDL) {
	merged, report, err := syntax.MergeUDLs(state.udls, []syntax.UDL{preset}, syntax.ConflictRename)
	if err == nil {
		err = state.setUDLDefinitions(merged, "", "")
	}
	if err != nil {
		dialog.message = state.tr("Could not add preset: ", "添加预设失败：") + err.Error()
		return
	}
	state.openUDLManager(len(merged) - 1)
	state.m5.message = state.udlImportReport(report)
}

func (state *model) udlJSONFilters() []app.FileFilter {
	return []app.FileFilter{{Name: state.tr("JSON files", "JSON 文件"), Pattern: "*.json"}}
}

func readUDLJSON(path string) ([]syntax.UDL, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > syntax.MaxUDLBundleBytes {
		return nil, fmt.Errorf("choose a regular JSON file no larger than 2 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return nil, fmt.Errorf("JSON file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, syntax.MaxUDLBundleBytes+1))
	if err != nil {
		return nil, err
	}
	return syntax.ParseUDLBundle(data)
}

func (state *model) importUDLJSON(dialog *m5Dialog) {
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: state.tr("Import JSON", "导入 JSON"), Filters: state.udlJSONFilters()})
	if errors.Is(err, app.ErrCanceled) || err == nil && len(paths) == 0 {
		return
	}
	if err == nil {
		err = state.mergeUDLJSON(dialog, paths[0])
	}
	if err != nil {
		dialog.message = state.tr("Import failed: ", "导入失败：") + err.Error()
	}
}

func (state *model) mergeUDLJSON(dialog *m5Dialog, path string) error {
	incoming, err := readUDLJSON(path)
	if err != nil {
		return err
	}
	merged, report, err := syntax.MergeUDLs(state.udls, incoming, dialog.udl.policy)
	if err != nil {
		return err
	}
	if report.Imported+report.Overwritten+report.Renamed > 0 {
		reload := dialog.index >= 0 && dialog.udl.policy == syntax.ConflictOverwrite && slices.ContainsFunc(incoming, func(definition syntax.UDL) bool {
			return definition.Mode() == state.udls[dialog.index].Mode()
		})
		if err := state.setUDLDefinitions(merged, "", ""); err != nil {
			return err
		}
		if reload || dialog.index < 0 && !slices.ContainsFunc(dialog.fields, func(field *text.Editor) bool { return field.String() != "" }) {
			state.openUDLManager(max(0, dialog.index))
			dialog = state.m5
		}
	}
	dialog.udl.action, dialog.message = udlEdit, state.udlImportReport(report)
	return nil
}

func (state *model) exportUDLJSON(dialog *m5Dialog, all bool) {
	definitions := state.udls
	name := "udl-bundle.json"
	if !all {
		definitions = definitions[dialog.index : dialog.index+1]
		name = "udl-language.json"
	}
	data, err := syntax.ExportUDLBundle(definitions)
	if err == nil {
		var path string
		path, err = app.SaveFile(app.SaveFileOptions{Title: state.tr("Export JSON", "导出 JSON"), SuggestedName: name, DefaultExtension: "json", Filters: state.udlJSONFilters()})
		if errors.Is(err, app.ErrCanceled) || err == nil && path == "" {
			return
		}
		if err == nil {
			err = state.writeUDLJSON(path, data)
		}
	}
	if err != nil {
		dialog.message = state.tr("Export failed: ", "导出失败：") + err.Error()
		return
	}
	dialog.udl.action, dialog.message = udlEdit, fmt.Sprintf(state.tr("Exported %d languages", "已导出 %d 个语言"), len(definitions))
	state.showToast(state.tr("UDL exported", "已导出自定义语言"))
}

func (state *model) writeUDLJSON(path string, data []byte) error {
	target, err := document.CanonicalSavePath(path)
	if err != nil {
		return err
	}
	if state.isPrivateDataPath(target) {
		return errors.New(state.tr("destination is an internal VivaPad data file", "目标是 VivaPad 内部数据文件"))
	}
	info, _ := os.Stat(target)
	for _, doc := range state.space.Documents {
		if doc.Path != "" && destinationMatchesFile(target, info, doc.Path) {
			return errors.New(state.tr("destination is open in the editor", "目标文件正在编辑器中打开"))
		}
	}
	return privatejson.WriteAtomic(target, data)
}

func (state *model) applyUDLHighlight(doc *document.Document, snapshot *components.CodeEditorSnapshot) {
	definition := state.udlForMode(snapshot.Language)
	if definition == nil {
		delete(state.udlHighlights, doc)
		return
	}
	cache := state.udlHighlights[doc]
	if cache.highlight == nil || cache.value != snapshot.Value || cache.mode != snapshot.Language {
		cache = udlHighlightCache{value: snapshot.Value, mode: snapshot.Language, highlight: definition.Highlighter(snapshot.Value)}
		state.udlHighlights[doc] = cache
	}
	snapshot.HighlightLine = cache.highlight
}
