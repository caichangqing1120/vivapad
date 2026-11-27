package texttools

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

const xmlNamespace = "http://www.w3.org/XML/1998/namespace"
const xmlnsNamespace = "http://www.w3.org/2000/xmlns/"

var xmlDeclaration = regexp.MustCompile(`^version[ \t\r\n]*=[ \t\r\n]*(?:"1\.0"|'1\.0')(?:[ \t\r\n]+encoding[ \t\r\n]*=[ \t\r\n]*(?:"[a-zA-Z][a-zA-Z0-9._-]*"|'[a-zA-Z][a-zA-Z0-9._-]*'))?(?:[ \t\r\n]+standalone[ \t\r\n]*=[ \t\r\n]*(?:"(?:yes|no)"|'(?:yes|no)'))?[ \t\r\n]*$`)

type xmlPart struct {
	raw   string
	node  *xmlElement
	space bool
}

type xmlElement struct {
	parent     *xmlElement
	name       xml.Name
	bindings   map[string]string
	open, end  string
	parts      []xmlPart
	structural bool
	verbatim   bool
	preserve   bool
}

func FormatXML(src, indent string) (string, error) {
	if len(src) > maxFormattedBytes {
		return "", fmt.Errorf("XML exceeds the formatting size limit")
	}
	if !utf8.ValidString(src) {
		return "", positionError("XML", src, 0, "expected decoded UTF-8 text")
	}
	bom := ""
	if strings.HasPrefix(src, "\xef\xbb\xbf") {
		bom, src = src[:3], src[3:]
	}
	decoder := xml.NewDecoder(strings.NewReader(src))
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	var top []xmlPart
	var current *xmlElement
	seenRoot, depth := false, 0
	doctype := ""
	for {
		start := int(decoder.InputOffset())
		token, err := decoder.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", positionError("XML", src, int(decoder.InputOffset()), err.Error())
		}
		raw := src[start:decoder.InputOffset()]
		fail := func(message string) (string, error) { return "", positionError("XML", src, start, message) }
		appendPart := func(part xmlPart) {
			if current == nil {
				top = append(top, part)
			} else {
				current.parts = append(current.parts, part)
			}
		}
		switch token := token.(type) {
		case xml.StartElement:
			depth++
			if depth > 512 {
				return fail("element nesting exceeds the formatting limit")
			}
			if !separatedXMLAttributes(raw) {
				return fail("attributes must be separated by whitespace")
			}
			if current == nil {
				if seenRoot {
					return fail("multiple root elements")
				}
				seenRoot = true
				name := token.Name.Local
				if token.Name.Space != "" {
					name = token.Name.Space + ":" + name
				}
				if doctype != "" && doctype != name {
					return fail("root element does not match DOCTYPE")
				}
			} else {
				current.structural = true
			}
			element := &xmlElement{parent: current, name: token.Name, open: raw, preserve: current != nil && current.preserve}
			if problem := element.validate(token.Attr); problem != "" {
				return fail(problem)
			}
			appendPart(xmlPart{node: element})
			current = element
		case xml.EndElement:
			if current == nil || current.name != token.Name {
				return fail("mismatched closing tag: " + token.Name.Local)
			}
			current.end, current = raw, current.parent
			depth--
		case xml.CharData:
			space := strings.Trim(raw, " \t\r\n") == ""
			if current == nil {
				if !space {
					return fail("character data outside the root element")
				}
			} else {
				current.verbatim = current.verbatim || !space
				appendPart(xmlPart{raw: raw, space: space})
			}
		case xml.Directive:
			body := string(token)
			if seenRoot || doctype != "" || !strings.HasPrefix(body, "DOCTYPE") || len(body) <= len("DOCTYPE") || !xmlWhitespace(body[len("DOCTYPE")]) {
				return fail("invalid or misplaced directive")
			}
			body = strings.TrimLeft(body[len("DOCTYPE"):], " \t\r\n")
			end := strings.IndexAny(body, " \t\r\n[")
			if end < 0 {
				end = len(body)
			}
			doctype = body[:end]
			if doctype == "" {
				return fail("DOCTYPE is missing its root name")
			}
			appendPart(xmlPart{raw: raw})
		case xml.ProcInst:
			if strings.EqualFold(token.Target, "xml") && (start != 0 || token.Target != "xml" || !xmlDeclaration.Match(token.Inst)) {
				return fail("invalid or misplaced XML declaration")
			}
			appendPart(xmlPart{raw: raw})
		default:
			appendPart(xmlPart{raw: raw})
		}
	}
	if current != nil || !seenRoot {
		return "", positionError("XML", src, len(src), "expected a complete root element")
	}
	var output strings.Builder
	output.Grow(len(src) + len(bom))
	output.WriteString(bom)
	for index, part := range top {
		if index > 0 {
			output.WriteByte('\n')
		}
		if !part.write(&output, indent, 0, false) {
			return "", fmt.Errorf("formatted XML exceeds the size limit")
		}
	}
	return output.String(), nil
}

func xmlWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}

func separatedXMLAttributes(tag string) bool {
	var quote byte
	for index := 0; index < len(tag); index++ {
		char := tag[index]
		if quote == 0 && (char == '\'' || char == '"') {
			quote = char
		} else if quote != 0 && char == quote {
			quote = 0
			if next := tag[index+1]; next != '>' && next != '/' && !xmlWhitespace(next) {
				return false
			}
		}
	}
	return true
}

func (element *xmlElement) namespace(prefix string) (string, bool) {
	if prefix == "xml" {
		return xmlNamespace, true
	}
	for scope := element; scope != nil; scope = scope.parent {
		if uri, ok := scope.bindings[prefix]; ok {
			return uri, uri != "" || prefix == ""
		}
	}
	return "", prefix == ""
}

func (element *xmlElement) validate(attrs []xml.Attr) string {
	for _, attr := range attrs {
		name := attr.Name
		if name.Space != "xmlns" && name != (xml.Name{Local: "xmlns"}) {
			continue
		}
		prefix := ""
		if name.Space == "xmlns" {
			prefix = name.Local
		}
		if prefix == "xmlns" || (prefix == "xml") != (attr.Value == xmlNamespace) || attr.Value == xmlnsNamespace || prefix != "" && attr.Value == "" {
			return "invalid namespace declaration: " + name.Local
		}
		if element.bindings == nil {
			element.bindings = make(map[string]string)
		}
		if _, exists := element.bindings[prefix]; exists {
			return "duplicate namespace declaration: " + name.Local
		}
		element.bindings[prefix] = attr.Value
	}
	if prefix := element.name.Space; prefix != "" {
		if _, ok := element.namespace(prefix); !ok {
			return "undeclared namespace prefix: " + prefix
		}
	}
	var seen map[xml.Name]bool
	if len(attrs) > 1 {
		seen = make(map[xml.Name]bool, len(attrs))
	}
	for _, attr := range attrs {
		name := attr.Name
		if name.Space == "xmlns" || name == (xml.Name{Local: "xmlns"}) {
			continue
		}
		if name.Space != "" {
			uri, ok := element.namespace(name.Space)
			if !ok {
				return "undeclared namespace prefix: " + name.Space
			}
			name.Space = uri
		}
		if seen != nil {
			if seen[name] {
				return "duplicate attribute: " + name.Local
			}
			seen[name] = true
		}
		if name == (xml.Name{Space: xmlNamespace, Local: "space"}) {
			switch attr.Value {
			case "preserve", "default":
				element.preserve = attr.Value == "preserve"
			default:
				return "invalid xml:space value"
			}
		}
	}
	return ""
}

func (part xmlPart) write(output *strings.Builder, indent string, depth int, exact bool) bool {
	if part.node == nil {
		output.WriteString(part.raw)
		return output.Len() <= maxFormattedBytes
	}
	node := part.node
	output.WriteString(node.open)
	exact = exact || node.verbatim || node.preserve || depth >= 256
	pretty := !exact && node.structural
	padding := func(level int) bool {
		if int64(output.Len())+1+int64(level)*int64(len(indent)) > maxFormattedBytes {
			return false
		}
		output.WriteByte('\n')
		for index := 0; index < level; index++ {
			output.WriteString(indent)
		}
		return true
	}
	for _, child := range node.parts {
		if pretty {
			if child.space {
				continue
			}
			if !padding(depth + 1) {
				return false
			}
		}
		if !child.write(output, indent, depth+1, exact) {
			return false
		}
	}
	if pretty && !padding(depth) {
		return false
	}
	output.WriteString(node.end)
	return output.Len() <= maxFormattedBytes
}
