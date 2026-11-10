package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type entry struct {
	kind      string
	character rune
	label     string
}

var declaration = regexp.MustCompile(`^static constexpr std::array<std::array<const char\*, 3>,\s*([0-9]+)>\s+(g_nonPrintingChars|g_ccUniEolChars)\{\s*\{`)
var initializer = regexp.MustCompile(`^\s*\{\s*"((?:\\.|[^"\\])*)"\s*,\s*"([^"]+)"\s*,\s*"U\+([0-9A-Fa-f]{4,6})"\s*\}\s*,?\s*(?://.*)?$`)

func parseSymbols(source io.Reader) ([]entry, error) {
	scanner := bufio.NewScanner(source)
	var entries []entry
	counts := map[string]int{}
	seen := map[string]map[rune]bool{}
	kind, expected, count := "", 0, 0
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if match := declaration.FindStringSubmatch(text); match != nil {
			if kind != "" {
				return nil, fmt.Errorf("line %d: nested character table", line)
			}
			kind = "nonprinting"
			if match[2] == "g_ccUniEolChars" {
				kind = "control"
			}
			if counts[kind] != 0 {
				return nil, fmt.Errorf("line %d: repeated %s table", line, kind)
			}
			var err error
			expected, err = strconv.Atoi(match[1])
			if err != nil || expected <= 0 {
				return nil, fmt.Errorf("line %d: invalid table length", line)
			}
			seen[kind] = make(map[rune]bool, expected)
			count = 0
			continue
		}
		if kind == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(text), "} };") {
			if count != expected {
				return nil, fmt.Errorf("line %d: %s has %d entries, declared %d", line, kind, count, expected)
			}
			counts[kind] = count
			kind = ""
			continue
		}
		match := initializer.FindStringSubmatch(text)
		if match == nil {
			if strings.HasPrefix(strings.TrimSpace(text), "{") {
				return nil, fmt.Errorf("line %d: malformed initializer", line)
			}
			continue
		}
		bytes, err := strconv.Unquote(`"` + match[1] + `"`)
		if err != nil || !utf8.ValidString(bytes) || utf8.RuneCountInString(bytes) != 1 {
			return nil, fmt.Errorf("line %d: invalid UTF-8 source character", line)
		}
		character, _ := utf8.DecodeRuneInString(bytes)
		codepoint, err := strconv.ParseUint(match[3], 16, 32)
		if err != nil || character != rune(codepoint) || seen[kind][character] {
			return nil, fmt.Errorf("line %d: mismatched or repeated codepoint", line)
		}
		seen[kind][character] = true
		entries = append(entries, entry{kind: kind, character: character, label: match[2]})
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if kind != "" || len(entries) == 0 {
		return nil, fmt.Errorf("incomplete or missing source character tables")
	}
	slices.SortFunc(entries, func(left, right entry) int {
		if order := strings.Compare(left.kind, right.kind); order != 0 {
			return order
		}
		return int(left.character - right.character)
	})
	return entries, nil
}

func renderTSV(entries []entry, source string) string {
	var output strings.Builder
	fmt.Fprintf(&output, "# %s\nkind\tcodepoint\tlabel\n", source)
	for _, item := range entries {
		fmt.Fprintf(&output, "%s\tU+%04X\t%s\n", item.kind, item.character, item.label)
	}
	return output.String()
}

func main() {
	source := flag.String("source", "", "pinned source identifier")
	output := flag.String("output", "", "generated TSV path")
	flag.Parse()
	if *source == "" || *output == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: nppsymbols -source REF -output FILE < ScintillaEditView.cpp")
		os.Exit(2)
	}
	entries, err := parseSymbols(os.Stdin)
	if err == nil {
		err = os.WriteFile(*output, []byte(renderTSV(entries, *source)), 0644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
