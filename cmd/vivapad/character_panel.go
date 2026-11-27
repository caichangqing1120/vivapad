package main

import (
	"fmt"
	"html"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

const characterPanelCommand = "Character Panel"

type characterRow struct {
	values, html [3]string
	label        string
}

type characterTable struct {
	encoding document.Encoding
	rows     [256]characterRow
}

func characterEncoding(encoding document.Encoding) document.Encoding {
	if encoding.IsLegacy() {
		return encoding
	}
	return document.Windows1252
}

var controlCharacterLabels = strings.Fields("NUL SOH STX ETX EOT ENQ ACK BEL BS TAB LF VT FF CR SO SI DLE DC1 DC2 DC3 DC4 NAK SYN ETB CAN EM SUB ESC FS GS RS US")

var characterEntityNames = func() map[string]string {
	names := strings.Fields(`quot amp apos lt gt nbsp iexcl cent pound curren yen brvbar sect uml copy ordf laquo not shy reg macr deg plusmn sup2 sup3 acute micro para middot cedil sup1 ordm raquo frac14 frac12 frac34 iquest
		Agrave Aacute Acirc Atilde Auml Aring AElig Ccedil Egrave Eacute Ecirc Euml Igrave Iacute Icirc Iuml ETH Ntilde Ograve Oacute Ocirc Otilde Ouml times Oslash Ugrave Uacute Ucirc Uuml Yacute THORN szlig
		agrave aacute acirc atilde auml aring aelig ccedil egrave eacute ecirc euml igrave iacute icirc iuml eth ntilde ograve oacute ocirc otilde ouml divide oslash ugrave uacute ucirc uuml yacute thorn yuml
		euro sbquo fnof bdquo hellip dagger Dagger circ permil Scaron lsaquo OElig Zcaron lsquo rsquo ldquo rdquo bull ndash mdash tilde trade scaron rsaquo oelig zcaron Yuml`)
	result := make(map[string]string, len(names))
	for _, name := range names {
		entity := "&" + name + ";"
		if decoded := html.UnescapeString(entity); decoded != entity {
			result[decoded] = entity
		}
	}
	return result
}()

func makeCharacterRow(encoding document.Encoding, value byte) characterRow {
	codePage := characterEncoding(encoding)
	row := characterRow{values: [3]string{fmt.Sprint(value), fmt.Sprintf("%02X", value)}}
	decoded, ok := document.DecodeLegacyByte(codePage, value)
	switch {
	case value < 32:
		row.label = controlCharacterLabels[value]
	case value == 32:
		row.label = "Space"
	case value == 127:
		row.label = "DEL"
	case ok:
		row.label = decoded
	default:
		row.label = "-"
	}
	if ok {
		row.values[2] = decoded
		if value == 13 {
			row.values[2] = "\n"
		}
		if codePage == document.Windows1252 && value >= 32 && value != 127 {
			for _, r := range decoded {
				row.html = [3]string{characterEntityNames[decoded], fmt.Sprintf("&#%d;", r), fmt.Sprintf("&#x%x;", r)}
				break
			}
		}
	}
	return row
}

func (state *model) toggleCharacterPanel() {
	state.characterVisible = !state.characterVisible
	if state.characterVisible {
		state.activeLeftDock = "characters"
	} else {
		state.characterTable = nil
		state.activeLeftDock = state.activeDock()
	}
}

func (state *model) insertPanelCharacter(value string) {
	doc := state.paneDocument(state.activePane)
	if state.isReadOnly(doc) {
		state.notice = "Read-only document"
		return
	}
	state.cancelDocumentComposition(doc)
	input := state.documentTextInput(state.activePane)
	if input == nil || input.OnEvent == nil {
		return
	}
	if _, err := input.OnEvent(app.TextEvent{Kind: app.TextPaste, Text: value}); err != nil {
		state.notice = err.Error()
	}
}

func (state *model) moveCharacterPanel(step int) {
	state.characterOffset = min(max(0, state.characterOffset+step), max(0, 256-state.characterPageRows))
}

func (state *model) paintCharacterPanel(canvas *gfx.Canvas, bounds gfx.Rect) {
	palette := state.palette()
	doc := state.paneDocument(state.activePane)
	encoding := characterEncoding(doc.Encoding())
	if state.characterTable == nil || state.characterTable.encoding != encoding {
		table := &characterTable{encoding: encoding}
		for value := range table.rows {
			table.rows[value] = makeCharacterRow(encoding, byte(value))
		}
		state.characterTable = table
	}
	canvas.Fill(bounds, palette.dock)
	canvas.Fill(gfx.Rect{X: bounds.Width - 1, Y: bounds.Y, Width: 1, Height: bounds.Height}, palette.border)
	canvas.HitWithOptions(characterPanelID, bounds, characterPanelCommand, gfx.HitOptions{Role: gfx.RoleGroup})
	smallLabel(canvas, characterPanelCommand, gfx.Rect{X: 8, Y: bounds.Y + 3, Width: bounds.Width - 37, Height: 23}, palette.text, 11)
	close := gfx.Rect{X: bounds.Width - 25, Y: bounds.Y + 6, Width: 18, Height: 18}
	canvas.DrawIcon(closeIcon, close.Inset(gfx.All(3)), palette.muted)
	if hitButton(canvas, characterCloseID, close, "Close Character Panel") {
		state.toggleCharacterPanel()
		canvas.RequestFocus(state.commandEditorFocusID())
		return
	}
	spec, _ := encoding.LegacySpec()
	label := gfx.Rect{X: 8, Y: bounds.Y + 28, Width: bounds.Width - 63, Height: 22}
	smallLabel(canvas, canvas.FitLine(spec.Label, 10, gfx.WeightRegular, label.Width), label, palette.muted, 10)
	mode := gfx.Rect{X: bounds.Width - 50, Y: bounds.Y + 28, Width: 43, Height: 21}
	if encoding == document.Windows1252 {
		canvas.Border(mode, 1, palette.border, palette.toolbar)
		canvas.LabelCenter("HTML", mode, gfx.TextStyle{Size: 10, Color: palette.text})
		canvas.HitWithOptions(characterHTMLID, mode, "HTML representations", gfx.HitOptions{Role: gfx.RoleButton, Checked: state.characterHTML})
		if canvas.Clicked(characterHTMLID) {
			state.characterHTML = !state.characterHTML
		}
	}
	htmlMode := encoding == document.Windows1252 && state.characterHTML
	headers := [3]string{"Dec", "Hex", "Character"}
	cellSize := float32(11)
	if htmlMode {
		headers = [3]string{"Name", "HTML dec", "HTML hex"}
		cellSize = 10
	}
	columnWidth := (bounds.Width - 12) / 3
	for i, header := range headers {
		smallLabel(canvas, header, gfx.Rect{X: 6 + float32(i)*columnWidth, Y: bounds.Y + 52, Width: columnWidth, Height: 21}, palette.muted, 10)
	}
	const rowHeight = 21
	state.characterPageRows = min(256, max(1, int((bounds.Height-104)/rowHeight)))
	state.moveCharacterPanel(0)
	for rowIndex := range state.characterPageRows {
		index := state.characterOffset + rowIndex
		row := state.characterTable.rows[index]
		values, labels := row.values, [3]string{row.values[0], row.values[1], row.label}
		if htmlMode {
			values, labels = row.html, row.html
		}
		for col, value := range values {
			field := gfx.Rect{X: 6 + float32(col)*columnWidth, Y: bounds.Y + 73 + float32(rowIndex*rowHeight), Width: columnWidth, Height: rowHeight}
			id := characterCellScope.At(index*3 + col)
			if value != "" && (canvas.Hovered() == id || canvas.FocusVisible() == id) {
				canvas.Fill(field, palette.hover)
			}
			text := canvas.FitLine(labels[col], cellSize, gfx.WeightRegular, field.Width-2)
			smallLabel(canvas, text, field, palette.text, cellSize)
			canvas.HitWithOptions(id, field, fmt.Sprintf("Byte %02X, %s: %s", index, headers[col], labels[col]), gfx.HitOptions{Role: gfx.RoleButton, Disabled: value == ""})
			if value != "" && canvas.Clicked(id) {
				state.insertPanelCharacter(value)
				canvas.RequestFocus(state.commandEditorFocusID())
			}
		}
	}
	y := bounds.Y + bounds.Height - 25
	for _, control := range []struct {
		id    gfx.ID
		x     float32
		icon  gfx.Icon
		label string
		step  int
	}{{characterPrevID, 6, upIcon, "Previous character page", -1}, {characterNextID, bounds.Width - 26, downIcon, "Next character page", 1}} {
		button := gfx.Rect{X: control.x, Y: y, Width: 20, Height: 20}
		canvas.DrawIcon(control.icon, button.Inset(gfx.All(3)), palette.muted)
		if hitButton(canvas, control.id, button, control.label) {
			state.moveCharacterPanel(control.step * state.characterPageRows)
		}
	}
	canvas.LabelCenter(fmt.Sprintf("%02X-%02X", state.characterOffset, state.characterOffset+state.characterPageRows-1),
		gfx.Rect{X: 30, Y: y, Width: bounds.Width - 60, Height: 20}, gfx.TextStyle{Size: 10, Color: palette.muted})
}
