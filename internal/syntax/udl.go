package syntax

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/vivaui/viva/components"
	"vivapad/internal/privatejson"
)

const maxUDLBytes = 256 << 10

type UDL struct {
	Name        string   `json:"name"`
	Extensions  []string `json:"extensions,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	LineComment string   `json:"line_comment,omitempty"`
	BlockStart  string   `json:"block_start,omitempty"`
	BlockEnd    string   `json:"block_end,omitempty"`
	Quotes      []string `json:"quotes,omitempty"`
}

func (definition UDL) Mode() string { return "udl:" + strings.ToLower(definition.Name) }

func ValidUDLMode(mode string) bool {
	name, ok := strings.CutPrefix(mode, "udl:")
	return ok && validUDLName(name) && name == strings.ToLower(name)
}

func validUDLName(name string) bool {
	return name != "" && name == strings.TrimSpace(name) && len(name) <= 96 && !strings.ContainsFunc(name, unicode.IsControl)
}

func LoadUDLs(path string) ([]UDL, error) {
	var definitions []UDL
	if err := privatejson.Load(path, maxUDLBytes, &definitions); err != nil {
		return nil, err
	}
	return normalizeUDLs(definitions)
}

func SaveUDLs(path string, definitions []UDL) error {
	normalized, err := normalizeUDLs(definitions)
	if err != nil {
		return err
	}
	return privatejson.Save(path, maxUDLBytes, normalized)
}

func normalizeUDLs(definitions []UDL) ([]UDL, error) {
	if len(definitions) > 64 {
		return nil, fmt.Errorf("at most 64 user languages are supported")
	}
	names, extensions := make(map[string]bool), make(map[string]bool)
	result := slices.Clone(definitions)
	for index, definition := range result {
		definition.Name = strings.TrimSpace(definition.Name)
		if !validUDLName(definition.Name) || names[definition.Mode()] {
			return nil, fmt.Errorf("language names must be non-empty and unique (up to 96 bytes)")
		}
		names[definition.Mode()] = true
		definition.Extensions = slices.Clone(definition.Extensions)
		for extIndex, extension := range definition.Extensions {
			extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
			if extension == "" || len(extension) > 32 || strings.ContainsFunc(extension, func(value rune) bool {
				return !udlWord(value) && value != '-' && value != '+'
			}) || extensions[extension] {
				return nil, fmt.Errorf("invalid or repeated extension %q", extension)
			}
			extensions[extension] = true
			definition.Extensions[extIndex] = extension
		}
		for _, keyword := range definition.Keywords {
			if keyword == "" || strings.ContainsFunc(keyword, func(value rune) bool { return !udlWord(value) }) {
				return nil, fmt.Errorf("invalid keyword %q", keyword)
			}
		}
		if (definition.BlockStart == "") != (definition.BlockEnd == "") {
			return nil, fmt.Errorf("block comments need both delimiters")
		}
		seen := make(map[string]bool)
		for _, marker := range append([]string{definition.LineComment, definition.BlockStart}, definition.Quotes...) {
			if marker == "" {
				continue
			}
			if !validUDLMarker(marker) || seen[marker] {
				return nil, fmt.Errorf("invalid or repeated delimiter %q", marker)
			}
			seen[marker] = true
		}
		if definition.BlockEnd != "" && !validUDLMarker(definition.BlockEnd) || slices.Contains(definition.Quotes, "") || len(definition.Quotes) > 8 {
			return nil, fmt.Errorf("invalid block end or string delimiters")
		}
		result[index] = definition
	}
	return result, nil
}

func validUDLMarker(marker string) bool {
	return len(marker) <= 24 && !strings.ContainsFunc(marker, func(value rune) bool { return unicode.IsSpace(value) || unicode.IsControl(value) })
}

func udlWord(value rune) bool {
	return value == '_' || unicode.IsLetter(value) || unicode.IsDigit(value) || unicode.IsMark(value)
}

func (definition UDL) Highlighter(value string) func(string, int, string, []components.CodeEditorRun) []components.CodeEditorRun {
	keywords := make(map[string]bool, len(definition.Keywords))
	for _, keyword := range definition.Keywords {
		keywords[keyword] = true
	}
	keywordColor, _ := tokenColor(chroma.Keyword)
	commentColor, _ := tokenColor(chroma.Comment)
	stringColor, _ := tokenColor(chroma.LiteralString)
	scan := func(line string, inBlock, emit bool, dst []components.CodeEditorRun) ([]components.CodeEditorRun, bool) {
		for offset := 0; offset < len(line); {
			start, color := offset, commentColor
			paint := true
			source := line[offset:]
			if inBlock {
				if end := strings.Index(source, definition.BlockEnd); end >= 0 {
					offset += end + len(definition.BlockEnd)
					inBlock = false
				} else {
					offset = len(line)
				}
			} else {
				block, lineComment, quote := "", "", ""
				if definition.BlockStart != "" && strings.HasPrefix(source, definition.BlockStart) {
					block = definition.BlockStart
				}
				if definition.LineComment != "" && strings.HasPrefix(source, definition.LineComment) {
					lineComment = definition.LineComment
				}
				for _, candidate := range definition.Quotes {
					if len(candidate) > len(quote) && strings.HasPrefix(source, candidate) {
						quote = candidate
					}
				}
				switch {
				case len(quote) > max(len(block), len(lineComment)):
					color = stringColor
					offset = udlQuoteEnd(line, offset+len(quote), quote)
				case len(block) > len(lineComment):
					offset += len(block)
					if end := strings.Index(line[offset:], definition.BlockEnd); end >= 0 {
						offset += end + len(definition.BlockEnd)
					} else {
						offset, inBlock = len(line), true
					}
				case lineComment != "":
					offset = len(line)
				default:
					value, size := utf8.DecodeRuneInString(source)
					offset += size
					paint = false
					if udlWord(value) {
						for offset < len(line) {
							value, size := utf8.DecodeRuneInString(line[offset:])
							if !udlWord(value) {
								break
							}
							offset += size
						}
						paint, color = keywords[line[start:offset]], keywordColor
					}
				}
			}
			if emit && paint {
				dst = append(dst, components.CodeEditorRun{Start: start, End: offset, Color: color})
			}
		}
		return dst, inBlock
	}
	lineStates := []bool{false}
	remaining := value
	return func(_ string, index int, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
		if index < 0 || len(line) > components.CodeEditorMaxHighlightedLine {
			return dst
		}
		if definition.BlockStart != "" {
			for len(lineStates) <= index && remaining != "" {
				prior, next, _ := strings.Cut(remaining, "\n")
				_, inBlock := scan(prior, lineStates[len(lineStates)-1], false, nil)
				lineStates, remaining = append(lineStates, inBlock), next
			}
		}
		result, _ := scan(line, index < len(lineStates) && lineStates[index], true, dst)
		return result
	}
}

func udlQuoteEnd(line string, offset int, quote string) int {
	for offset < len(line) {
		if strings.HasPrefix(line[offset:], quote) {
			return offset + len(quote)
		}
		if line[offset] == '\\' {
			offset++
			if offset == len(line) {
				break
			}
		}
		_, size := utf8.DecodeRuneInString(line[offset:])
		offset += size
	}
	return offset
}
