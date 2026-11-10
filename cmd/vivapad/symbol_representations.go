package main

import (
	"fmt"
	"sort"

	"github.com/vivaui/viva/components"
)

type symbolRepresentation struct {
	character    rune
	abbreviation string
}

// Notepad++ v8.9.8.1 ScintillaEditView.cpp: g_nonPrintingChars and g_ccUniEolChars.
var nonPrintingCharacters = [...]symbolRepresentation{
	{'\u00a0', "NBSP"}, {'\u00ad', "SHY"}, {'\u061c', "ALM"}, {'\u070f', "SAM"},
	{'\u1680', "OSPM"}, {'\u180e', "MVS"},
	{'\u2000', "NQSP"}, {'\u2001', "MQSP"}, {'\u2002', "ENSP"}, {'\u2003', "EMSP"},
	{'\u2004', "3/MSP"}, {'\u2005', "4/MSP"}, {'\u2006', "6/MSP"}, {'\u2007', "FSP"},
	{'\u2008', "PSP"}, {'\u2009', "THSP"}, {'\u200a', "HSP"}, {'\u200b', "ZWSP"},
	{'\u200c', "ZWNJ"}, {'\u200d', "ZWJ"}, {'\u200e', "LRM"}, {'\u200f', "RLM"},
	{'\u202a', "LRE"}, {'\u202b', "RLE"}, {'\u202c', "PDF"}, {'\u202d', "LRO"},
	{'\u202e', "RLO"}, {'\u202f', "NNBSP"}, {'\u205f', "MMSP"}, {'\u2060', "WJ"},
	{'\u2061', "(FA)"}, {'\u2062', "(IT)"}, {'\u2063', "(IS)"}, {'\u2064', "(IP)"},
	{'\u2066', "LRI"}, {'\u2067', "RLI"}, {'\u2068', "FSI"}, {'\u2069', "PDI"},
	{'\u206a', "ISS"}, {'\u206b', "ASS"}, {'\u206c', "IAFS"}, {'\u206d', "AAFS"},
	{'\u206e', "NADS"}, {'\u206f', "NODS"}, {'\u3000', "IDSP"},
	{'\ufeff', "ZWNBSP"}, {'\ufff9', "IAA"}, {'\ufffa', "IAS"}, {'\ufffb', "IAT"},
}

var controlUnicodeCharacters = [...]symbolRepresentation{
	{'\x00', "NUL"}, {'\x01', "SOH"}, {'\x02', "STX"}, {'\x03', "ETX"},
	{'\x04', "EOT"}, {'\x05', "ENQ"}, {'\x06', "ACK"}, {'\x07', "BEL"},
	{'\x08', "BS"}, {'\x0b', "VT"}, {'\x0c', "FF"}, {'\x0e', "SO"},
	{'\x0f', "SI"}, {'\x10', "DLE"}, {'\x11', "DC1"}, {'\x12', "DC2"},
	{'\x13', "DC3"}, {'\x14', "DC4"}, {'\x15', "NAK"}, {'\x16', "SYN"},
	{'\x17', "ETB"}, {'\x18', "CAN"}, {'\x19', "EM"}, {'\x1a', "SUB"},
	{'\x1b', "ESC"}, {'\x1c', "FS"}, {'\x1d', "GS"}, {'\x1e', "RS"},
	{'\x1f', "US"}, {'\x7f', "DEL"},
	{'\u0080', "PAD"}, {'\u0081', "HOP"}, {'\u0082', "BPH"}, {'\u0083', "NBH"},
	{'\u0084', "IND"}, {'\u0085', "NEL"}, {'\u0086', "SSA"}, {'\u0087', "ESA"},
	{'\u0088', "HTS"}, {'\u0089', "HTJ"}, {'\u008a', "VTS"}, {'\u008b', "PLD"},
	{'\u008c', "PLU"}, {'\u008d', "RI"}, {'\u008e', "SS2"}, {'\u008f', "SS3"},
	{'\u0090', "DCS"}, {'\u0091', "PU1"}, {'\u0092', "PU2"}, {'\u0093', "STS"},
	{'\u0094', "CCH"}, {'\u0095', "MW"}, {'\u0096', "SPA"}, {'\u0097', "EPA"},
	{'\u0098', "SOS"}, {'\u0099', "SGCI"}, {'\u009a', "SCI"}, {'\u009b', "CSI"},
	{'\u009c', "ST"}, {'\u009d', "OSC"}, {'\u009e', "PM"}, {'\u009f', "APC"},
	{'\u2028', "LS"}, {'\u2029', "PS"},
}

func abbreviationOf(table []symbolRepresentation, character rune) (string, bool) {
	index := sort.Search(len(table), func(index int) bool { return table[index].character >= character })
	if index < len(table) && table[index].character == character {
		return table[index].abbreviation, true
	}
	return "", false
}

func (state *model) symbolLabel(item symbolRepresentation, control bool) string {
	if control && !state.npcIncludeControls {
		return item.abbreviation
	}
	switch state.npcMode {
	case "identity":
		return string(item.character)
	case "codepoint":
		return fmt.Sprintf("U+%04X", item.character)
	default:
		return item.abbreviation
	}
}

func (state *model) codeRepresentations() []components.CodeRepresentation {
	if state.representationsReady {
		return state.representations
	}
	state.representations = state.representations[:0]
	nonPrinting := nonPrintingCharacters[:]
	if !state.showNonPrinting {
		nonPrinting = nil
	}
	controls := controlUnicodeCharacters[:]
	for len(nonPrinting) > 0 || len(controls) > 0 {
		if len(controls) == 0 || len(nonPrinting) > 0 && nonPrinting[0].character < controls[0].character {
			entry := nonPrinting[0]
			state.representations = append(state.representations, components.CodeRepresentation{Character: entry.character, Label: state.symbolLabel(entry, false), Blob: true})
			nonPrinting = nonPrinting[1:]
			continue
		}
		entry := controls[0]
		label := state.symbolLabel(entry, true)
		if !state.showControls {
			label = ""
		}
		state.representations = append(state.representations, components.CodeRepresentation{Character: entry.character, Label: label, Blob: label != ""})
		controls = controls[1:]
	}
	state.representationsReady = true
	return state.representations
}
