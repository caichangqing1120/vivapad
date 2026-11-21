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
	call   CallHint
	closer byte
}

// CallAt returns the innermost open call with a known signature. Caret and
// Open are UTF-8 byte offsets. Separators count only in the frame that owns them.
func (catalog *FunctionCatalog) CallAt(source string, caret int, lineComment string, tripleQuotes bool) CallHint {
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
	for i := 0; i < len(source); i++ {
		ch := source[i]
		switch {
		case lineComment != "" && strings.HasPrefix(source[i:], lineComment):
			end := strings.IndexAny(source[i:], "\r\n")
			if end < 0 {
				i = len(source)
			} else {
				i += end
			}
		case !tripleQuotes && ch == '/' && strings.HasPrefix(source[i:], "/*"):
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				i = len(source)
			} else {
				i += end + 3
			}
		case ch == '\'' || ch == '"' || ch == '`':
			i = endQuoted(source, i, tripleQuotes) - 1
		case ch == '(' || ch == '[' || ch == '{':
			if len(stack) == 2048 {
				return CallHint{}
			}
			frame := callFrame{closer: ch + 2}
			if ch == '(' {
				frame.closer = ')'
			}
			if ch == marks.open {
				frame.call = catalog.callAtOpen(source, i)
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

func (catalog *FunctionCatalog) callAtOpen(source string, open int) CallHint {
	end := len(strings.TrimRight(source[:open], " \t\n\r"))
	start := end
	for start > 0 && end-start <= maxAPINameBytes {
		r, size := utf8.DecodeLastRuneInString(source[:start])
		if !wordCharacter(r) && !strings.ContainsRune(catalog.additional, r) {
			break
		}
		start -= size
	}
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
