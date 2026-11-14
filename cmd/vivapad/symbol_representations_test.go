package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSourceNonPrintingAndControlTablesHaveExactCardinalityAndOrder(t *testing.T) {
	for _, test := range []struct {
		name string
		rows []symbolRepresentation
		want int
	}{
		{"non-printing", nonPrintingCharacters[:], 49},
		{"control-and-Unicode-EOL", controlUnicodeCharacters[:], 64},
	} {
		if len(test.rows) != test.want {
			t.Errorf("%s count = %d, want %d", test.name, len(test.rows), test.want)
		}
		for index, entry := range test.rows {
			if index > 0 && entry.character <= test.rows[index-1].character {
				t.Errorf("%s is not strictly sorted at %d: U+%04X", test.name, index, entry.character)
			}
			if entry.abbreviation == "" || !utf8.ValidString(entry.abbreviation) {
				t.Errorf("%s has invalid label at U+%04X", test.name, entry.character)
			}
		}
	}
}

func TestEverySourceRepresentationMatchesPinnedCPP(t *testing.T) {
	path := filepath.Join("testdata", "source-symbols.tsv")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 3
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 114 || strings.Join(rows[0], "\t") != "kind\tcodepoint\tlabel" {
		t.Fatalf("pinned symbol rows = %d, %v", len(rows), err)
	}
	indices := map[string]int{}
	for _, row := range rows[1:] {
		codepoint, err := strconv.ParseUint(strings.TrimPrefix(row[1], "U+"), 16, 32)
		if err != nil || !strings.HasPrefix(row[1], "U+") {
			t.Fatalf("bad source codepoint %q: %v", row[1], err)
		}
		var table []symbolRepresentation
		switch row[0] {
		case "control":
			table = controlUnicodeCharacters[:]
		case "nonprinting":
			table = nonPrintingCharacters[:]
		default:
			t.Fatalf("unknown source table %q", row[0])
		}
		index := indices[row[0]]
		if index >= len(table) || table[index] != (symbolRepresentation{character: rune(codepoint), abbreviation: row[2]}) {
			t.Errorf("%s entry %d differs from source U+%04X %q", row[0], index, codepoint, row[2])
		}
		indices[row[0]]++
	}
	if indices["control"] != len(controlUnicodeCharacters) || indices["nonprinting"] != len(nonPrintingCharacters) {
		t.Fatalf("source table counts = %+v", indices)
	}
}

func TestSourceRepresentationsSeparatePhysicalBreaksAndUnicodeEOL(t *testing.T) {
	for _, sample := range []struct {
		character rune
		label     string
	}{
		{'\u00a0', "NBSP"}, {'\u200b', "ZWSP"}, {'\u202e', "RLO"}, {'\u2061', "(FA)"}, {'\ufeff', "ZWNBSP"},
	} {
		if label, found := abbreviationOf(nonPrintingCharacters[:], sample.character); !found || label != sample.label {
			t.Errorf("nonprinting U+%04X = %q, %v; want %q", sample.character, label, found, sample.label)
		}
	}
	for _, sample := range []struct {
		character rune
		label     string
	}{
		{'\x00', "NUL"}, {'\x1b', "ESC"}, {'\x7f', "DEL"}, {'\u0085', "NEL"}, {'\u009f', "APC"}, {'\u2028', "LS"}, {'\u2029', "PS"},
	} {
		if label, found := abbreviationOf(controlUnicodeCharacters[:], sample.character); !found || label != sample.label {
			t.Errorf("control U+%04X = %q, %v; want %q", sample.character, label, found, sample.label)
		}
	}
	for _, physical := range []rune{'\t', '\r', '\n'} {
		if _, found := abbreviationOf(controlUnicodeCharacters[:], physical); found {
			t.Errorf("physical Tab/CR/LF U+%04X incorrectly represented", physical)
		}
	}
}
