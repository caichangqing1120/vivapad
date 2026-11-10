package main

import (
	"strings"
	"testing"
)

func TestParseSourceRepresentationsDecodesBytesAndSortsUnicodeEOL(t *testing.T) {
	source := `static constexpr std::array<std::array<const char*, 3>, 3> g_ccUniEolChars{ {
    {"\x00", "NUL", "U+0000"},
    //{"\x09", "HT", "U+0009"},
    {"\xE2\x80\xA8", "LS", "U+2028"},
    {"\xC2\x85", "NEL", "U+0085"}
} };
static constexpr std::array<std::array<const char*, 3>, 2> g_nonPrintingChars{ {
    {"\xE2\x80\x8B", "ZWSP", "U+200B"},
    {"\xC2\xA0", "NBSP", "U+00A0"}
} };`
	entries, err := parseSymbols(strings.NewReader(source))
	if err != nil || len(entries) != 5 {
		t.Fatalf("parsed entries = %+v, %v", entries, err)
	}
	for index, want := range []entry{
		{kind: "control", character: 0x00, label: "NUL"},
		{kind: "control", character: 0x85, label: "NEL"},
		{kind: "control", character: 0x2028, label: "LS"},
		{kind: "nonprinting", character: 0xa0, label: "NBSP"},
		{kind: "nonprinting", character: 0x200b, label: "ZWSP"},
	} {
		if entries[index] != want {
			t.Errorf("entry %d = %+v, want %+v", index, entries[index], want)
		}
	}
	if got := renderTSV(entries, "pinned-source"); !strings.HasPrefix(got, "# pinned-source\nkind\tcodepoint\tlabel\n") ||
		!strings.Contains(got, "control\tU+0085\tNEL\n") {
		t.Fatalf("TSV = %q", got)
	}
}

func TestParseSourceRepresentationsRejectsTruncatedAndWrongCodepoint(t *testing.T) {
	for _, source := range []string{
		`static constexpr std::array<std::array<const char*, 3>, 2> g_nonPrintingChars{ {
    {"\xC2\xA0", "NBSP", "U+00A0"}
} };`,
		`static constexpr std::array<std::array<const char*, 3>, 1> g_nonPrintingChars{ {
    {"\xC2\xA0", "NBSP", "U+00A1"}
} };`,
		`static constexpr std::array<std::array<const char*, 3>, 1> g_nonPrintingChars{ {
    {"\xC2\xA0", "NBSP", "U+00A0"},
    {"\xC2\xA0", "NBSP", "U+00A0"}
} };`,
	} {
		if _, err := parseSymbols(strings.NewReader(source)); err == nil {
			t.Fatalf("invalid source accepted: %q", source)
		}
	}
}
