package syntax

import (
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

type FoldRegion struct {
	Header   int
	LastLine int
	Level    int
}

type foldStart struct {
	line   int
	level  int
	closer rune
}

func FoldRegions(mode, value string) []FoldRegion {
	if len(value) > 8<<20 {
		return nil
	}
	if strings.EqualFold(mode, ".hex") {
		return intelHEXFoldRegions(value)
	}
	if strings.EqualFold(mode, ".iss") {
		return innoFoldRegions(value)
	}
	if csoundMode(mode) {
		return csoundFoldRegions(value)
	}
	switch strings.ToLower(mode) {
	case ".reb", ".r2", ".r3":
		return rebolFoldRegions(value)
	}
	if mode == ".py" || mode == ".yaml" || mode == ".yml" {
		return indentFoldRegions(value, mode == ".py")
	}
	if !braceFoldMode(mode) {
		return nil
	}
	spec, ok := LanguageByMode(mode)
	if !ok {
		return nil
	}
	lexer := lexers.Get(spec.Lexer)
	if lexer == nil {
		return nil
	}
	iterator, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root"}, value)
	if err != nil {
		return nil
	}
	var starts []foldStart
	var regions []FoldRegion
	line := 0
	for token := iterator(); token != chroma.EOF; token = iterator() {
		skip := token.Type.InCategory(chroma.Comment) || token.Type.InCategory(chroma.LiteralString)
		for _, character := range token.Value {
			if !skip {
				closer := rune(0)
				switch character {
				case '{':
					closer = '}'
				case '[':
					if mode == ".json" {
						closer = ']'
					}
				}
				if closer != 0 {
					starts = append(starts, foldStart{line: line, level: len(starts) + 1, closer: closer})
				} else if len(starts) != 0 && character == starts[len(starts)-1].closer {
					start := starts[len(starts)-1]
					starts = starts[:len(starts)-1]
					if line > start.line {
						regions = append(regions, FoldRegion{Header: start.line, LastLine: line, Level: start.level})
					}
				}
			}
			if character == '\n' {
				line++
			}
		}
	}
	return sortedFoldRegions(regions)
}

func innoFoldRegions(value string) []FoldRegion {
	var regions []FoldRegion
	var state innoState
	header, line := -1, 0
	finish := func() {
		if header >= 0 && header < line-1 {
			regions = append(regions, FoldRegion{Header: header, LastLine: line - 1, Level: 1})
		}
	}
	for value != "" {
		end, next := syntaxLineBounds(value)
		var isHeader bool
		state, isHeader = scanInnoLine(value[:end], state, nil)
		if isHeader {
			finish()
			header = line
		}
		value = value[next:]
		line++
	}
	finish()
	return regions
}

func intelHEXFoldRegions(value string) []FoldRegion {
	var regions []FoldRegion
	header, last := -1, -1
	finish := func() {
		if header >= 0 && last > header {
			regions = append(regions, FoldRegion{Header: header, LastLine: last, Level: 1})
		}
		header = -1
	}
	lineNumber := 0
	for line := range strings.SplitSeq(strings.TrimSuffix(value, "\n"), "\n") {
		line = strings.TrimLeft(line, " \t")
		if strings.HasPrefix(line, ":") {
			kind, valid := hexByte(line, 7)
			if !valid || kind != 0 {
				finish()
				// A header must match its declared width, including the checksum
				// digits. Highlighting handles checksum mismatches.
				count, known := hexByte(line, 1)
				if valid && (kind == 2 || kind == 4) && known && count == 2 &&
					len(line) == 15 && hexDigits(line, 3, len(line)) {
					header = lineNumber
				}
			}
		}
		last = lineNumber
		lineNumber++
	}
	finish()
	return regions
}

func braceFoldMode(mode string) bool {
	switch mode {
	case ".go", ".js", ".ts", ".json", ".rs", ".c", ".cpp", ".cs", ".java", ".css", ".php", ".swift", ".m":
		return true
	}
	return false
}

type indentStart struct {
	line   int
	indent int
	level  int
}

func indentFoldRegions(value string, python bool) []FoldRegion {
	lines := strings.Split(value, "\n")
	var starts []indentStart
	var regions []FoldRegion
	previousLine, previousIndent := -1, 0
	for line, content := range lines {
		trimmed := strings.TrimSpace(content)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := 0
		for _, character := range content {
			switch character {
			case ' ':
				indent++
			case '\t':
				indent += 4 - indent%4
			default:
				goto measured
			}
		}
	measured:
		for len(starts) != 0 && indent <= starts[len(starts)-1].indent {
			start := starts[len(starts)-1]
			starts = starts[:len(starts)-1]
			if line-1 > start.line {
				regions = append(regions, FoldRegion{Header: start.line, LastLine: line - 1, Level: start.level})
			}
		}
		if previousLine >= 0 && indent > previousIndent && (!python || strings.HasSuffix(strings.TrimSpace(lines[previousLine]), ":")) {
			starts = append(starts, indentStart{line: previousLine, indent: previousIndent, level: len(starts) + 1})
		}
		previousLine, previousIndent = line, indent
	}
	for len(starts) != 0 {
		start := starts[len(starts)-1]
		starts = starts[:len(starts)-1]
		if previousLine > start.line {
			regions = append(regions, FoldRegion{Header: start.line, LastLine: previousLine, Level: start.level})
		}
	}
	return sortedFoldRegions(regions)
}

func sortedFoldRegions(regions []FoldRegion) []FoldRegion {
	sort.Slice(regions, func(i, j int) bool {
		if regions[i].Header == regions[j].Header {
			return regions[i].LastLine > regions[j].LastLine
		}
		return regions[i].Header < regions[j].Header
	})
	result := regions[:0]
	for _, region := range regions {
		if len(result) == 0 || result[len(result)-1].Header != region.Header {
			result = append(result, region)
		}
	}
	return result
}
