package completion

import (
	"encoding/xml"
	"sort"
	"strings"
	"unicode/utf8"
)

type Signature struct {
	Return string   `json:"return,omitempty"`
	Params []string `json:"params,omitempty"`
}

func (signature *Signature) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Return string `xml:"retVal,attr"`
		Params []struct {
			Name string `xml:"name,attr"`
		} `xml:"Param"`
	}
	if err := decoder.DecodeElement(&raw, &start); err != nil {
		return err
	}
	signature.Return = raw.Return
	for _, param := range raw.Params {
		signature.Params = append(signature.Params, param.Name)
	}
	return nil
}

type CallHint struct {
	Name      string
	Open      int
	Argument  int
	Overloads []Signature
}

type CallSyntax struct {
	LineComment         string
	TripleQuotes        bool
	NestedBlockComments bool
}

type callDelimiters struct{ open, separator byte }

func catalogDelimiters(open, close, separator string) callDelimiters {
	if open == "" {
		open = "("
	}
	if close == "" {
		close = ")"
	}
	if separator == "" {
		separator = ","
	}
	if (open+close != "()" && open+close != "[]" && open+close != "{}") || len(separator) != 1 || !strings.Contains(",;", separator) {
		return callDelimiters{open: 255}
	}
	return callDelimiters{open[0], separator[0]}
}

type callFrame struct {
	call        CallHint
	closer      byte
	rustNameEnd int
}

// CallAt returns the innermost open call with a known signature. Caret and
// Open are UTF-8 byte offsets. Separators count only in the frame that owns them.
func (catalog *FunctionCatalog) CallAt(source string, caret int, syntax CallSyntax) CallHint {
	if catalog == nil || len(catalog.signatures) == 0 || caret <= 0 || caret > len(source) {
		return CallHint{}
	}
	marks := catalog.delimiters
	if marks.open == 255 {
		return CallHint{}
	}
	if marks.open == 0 {
		marks = callDelimiters{'(', ','}
	}
	source = source[:caret]
	var stack []callFrame
	var rustArgsEnd, rustNameEnd int
	for i := 0; i < len(source); i++ {
		ch := source[i]
		switch {
		case syntax.LineComment != "" && strings.HasPrefix(source[i:], syntax.LineComment):
			end := strings.IndexAny(source[i:], "\r\n")
			if end < 0 {
				i = len(source)
			} else {
				i += end
			}
		case !syntax.TripleQuotes && ch == '/' && strings.HasPrefix(source[i:], "/*"):
			i = endBlockComment(source, i+2, syntax.NestedBlockComments) - 1
		case ch == '\'' || ch == '"' || ch == '`':
			end := 0
			if catalog.rustNames {
				switch ch {
				case '"':
					end = endRustRawString(source, i)
				case '\'':
					end = endRustLifetime(source, i)
				}
			}
			if end == 0 {
				end = endQuoted(source, i, syntax.TripleQuotes)
			}
			i = end - 1
		case catalog.rustNames && ch == '<':
			left := strings.TrimRight(source[:i], " \t\n\r")
			var nameEnd int
			if strings.HasSuffix(left, "::") {
				nameEnd = len(strings.TrimRight(left[:len(left)-2], " \t\n\r"))
			} else if len(stack) == 0 || stack[len(stack)-1].closer != '>' {
				continue
			}
			if len(stack) == 2048 {
				return CallHint{}
			}
			stack = append(stack, callFrame{closer: '>', rustNameEnd: nameEnd})
		case catalog.rustNames && ch == '>' && len(stack) != 0 && stack[len(stack)-1].closer == '>' && source[i-1] != '-':
			if end := stack[len(stack)-1].rustNameEnd; end != 0 {
				rustNameEnd, rustArgsEnd = end, i+1
			}
			stack = stack[:len(stack)-1]
		case ch == '(' || ch == '[' || ch == '{':
			if len(stack) == 2048 {
				return CallHint{}
			}
			frame := callFrame{closer: ch + 2}
			if ch == '(' {
				frame.closer = ')'
			}
			if ch == marks.open {
				end := len(strings.TrimRight(source[:i], " \t\n\r"))
				if rustArgsEnd != 0 && end == rustArgsEnd {
					end = rustNameEnd
				}
				frame.call = catalog.callAtOpen(source, i, end)
			}
			stack = append(stack, frame)
		case ch == ')' || ch == ']' || ch == '}':
			if len(stack) != 0 {
				if stack[len(stack)-1].closer == ch {
					stack = stack[:len(stack)-1]
				} else {
					stack = stack[:0]
				}
			}
		case ch == marks.separator && len(stack) != 0:
			stack[len(stack)-1].call.Argument++
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if len(stack[i].call.Overloads) != 0 {
			return stack[i].call
		}
	}
	return CallHint{}
}

// endBlockComment starts after the opening token and returns the first byte
// outside the comment, or the caret when the comment is still open.
func endBlockComment(source string, start int, nested bool) int {
	if !nested {
		if end := strings.Index(source[start:], "*/"); end >= 0 {
			return start + end + 2
		}
		return len(source)
	}
	depth := 1
	for i := start; i+1 < len(source); i++ {
		switch source[i : i+2] {
		case "/*":
			depth++
			i++
		case "*/":
			depth--
			if depth == 0 {
				return i + 2
			}
			i++
		}
	}
	return len(source)
}

func (catalog *FunctionCatalog) callAtOpen(source string, open, end int) CallHint {
	start := catalog.nameStart(source, end)
	if start == end || end-start > maxAPINameBytes {
		return CallHint{}
	}
	key := catalog.key(source[start:end])
	if overloads := catalog.signatures[key]; len(overloads) != 0 {
		index := sort.Search(len(catalog.entries), func(i int) bool { return catalog.entries[i].key >= key })
		if index < len(catalog.entries) && catalog.entries[index].key == key {
			return CallHint{Name: catalog.entries[index].name, Open: open, Overloads: overloads}
		}
	}
	return CallHint{}
}

// Lifetimes and loop labels have no closing apostrophe. Leave closed character
// literals and escape sequences to the quoted-text scanner.
func endRustLifetime(source string, quote int) int {
	start := quote + 1
	if strings.HasPrefix(source[start:], "r#") {
		start += 2
	}
	end := start
	for end < len(source) {
		r, size := utf8.DecodeRuneInString(source[end:])
		if !wordCharacter(r) {
			break
		}
		end += size
	}
	if end == start || end < len(source) && source[end] == '\'' {
		return 0
	}
	return end
}

// endRustRawString recognizes r, br and cr raw delimiters at an opening quote.
// It returns zero for an ordinary quote, or the caret for an unfinished raw string.
func endRustRawString(source string, quote int) int {
	hashStart := quote
	for hashStart > 0 && source[hashStart-1] == '#' {
		hashStart--
		if quote-hashStart > 255 {
			return 0
		}
	}
	if hashStart == 0 || source[hashStart-1] != 'r' {
		return 0
	}
	prefix := hashStart - 1
	if prefix > 0 && (source[prefix-1] == 'b' || source[prefix-1] == 'c') {
		prefix--
	}
	if before, _ := utf8.DecodeLastRuneInString(source[:prefix]); wordCharacter(before) {
		return 0
	}
	hashes := source[hashStart:quote]
	for start := quote + 1; start < len(source); {
		next := strings.IndexByte(source[start:], '"')
		if next < 0 {
			break
		}
		start += next + 1
		if strings.HasPrefix(source[start:], hashes) {
			return start + len(hashes)
		}
	}
	return len(source)
}

func endQuoted(source string, start int, tripleQuotes bool) int {
	quote := source[start]
	delimiter := source[start : start+1]
	if tripleQuotes && quote != '`' && len(source)-start >= 3 && source[start+1] == quote && source[start+2] == quote {
		delimiter = source[start : start+3]
	}
	for i := start + len(delimiter); i < len(source); i++ {
		if strings.HasPrefix(source[i:], delimiter) {
			return i + len(delimiter)
		}
		if source[i] == '\\' && quote != '`' && i+1 < len(source) {
			i++
		} else if (source[i] == '\n' || source[i] == '\r') && quote != '`' && len(delimiter) == 1 {
			return i
		}
	}
	return len(source)
}
