package texttools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

const maxFormattedBytes = 32 << 20

func FormatJSON(src, indent string) (string, error) {
	if !jsonIndentFits(src, indent) {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(src), &raw); err != nil {
			return "", jsonError(src, err)
		}
		return "", fmt.Errorf("formatted JSON exceeds the size limit")
	}
	var output bytes.Buffer
	if err := json.Indent(&output, []byte(src), "", indent); err != nil {
		return "", jsonError(src, err)
	}
	return output.String(), nil
}

func jsonIndentFits(src, indent string) bool {
	projected, depth := int64(len(src)), 0
	inString, escaped := false, false
	for index := 0; index < len(src); index++ {
		char := src[index]
		if inString {
			if !escaped && char == '"' {
				inString = false
			}
			escaped = !escaped && char == '\\'
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{', '[':
			depth++
			projected += 1 + int64(depth)*int64(len(indent))
		case '}', ']':
			depth = max(0, depth-1)
			projected += 1 + int64(depth)*int64(len(indent))
		case ',':
			projected += 1 + int64(depth)*int64(len(indent))
		case ':':
			projected++
		}
		if projected > maxFormattedBytes {
			return false
		}
	}
	return projected <= maxFormattedBytes
}

func MinifyJSON(src string) (string, error) {
	var output bytes.Buffer
	if err := json.Compact(&output, []byte(src)); err != nil {
		return "", jsonError(src, err)
	}
	return output.String(), nil
}

type jsonValue struct {
	raw      string
	shape    json.Delim
	children []jsonValue
	fields   []jsonField
}

type jsonField struct {
	name, rawKey string
	value        jsonValue
}

func SortJSONKeys(src, indent string) (string, error) {
	compact, err := MinifyJSON(src)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(strings.NewReader(compact))
	decoder.UseNumber()
	budget := 500_000
	root, err := readJSONValue(decoder, compact, &budget)
	if err != nil {
		return "", err
	}
	var sorted strings.Builder
	sorted.Grow(len(compact))
	root.write(&sorted)
	return FormatJSON(sorted.String(), indent)
}

func readJSONValue(decoder *json.Decoder, src string, budget *int) (jsonValue, error) {
	*budget--
	if *budget < 0 {
		return jsonValue{}, fmt.Errorf("JSON is too complex to sort")
	}
	start := jsonTokenStart(decoder, src)
	token, err := decoder.Token()
	if err != nil {
		return jsonValue{}, err
	}
	shape, container := token.(json.Delim)
	if !container {
		return jsonValue{raw: src[start:decoder.InputOffset()]}, nil
	}
	result := jsonValue{shape: shape}
	for decoder.More() {
		if shape == '{' {
			keyStart := jsonTokenStart(decoder, src)
			key, err := decoder.Token()
			if err != nil {
				return jsonValue{}, err
			}
			rawKey := src[keyStart:decoder.InputOffset()]
			child, err := readJSONValue(decoder, src, budget)
			if err != nil {
				return jsonValue{}, err
			}
			result.fields = append(result.fields, jsonField{name: key.(string), rawKey: rawKey, value: child})
		} else {
			child, err := readJSONValue(decoder, src, budget)
			if err != nil {
				return jsonValue{}, err
			}
			result.children = append(result.children, child)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return jsonValue{}, err
	}
	slices.SortStableFunc(result.fields, func(left, right jsonField) int { return strings.Compare(left.name, right.name) })
	return result, nil
}

func jsonTokenStart(decoder *json.Decoder, compact string) int {
	start := int(decoder.InputOffset())
	if start < len(compact) && (compact[start] == ',' || compact[start] == ':') {
		start++
	}
	return start
}

func (value jsonValue) write(output *strings.Builder) {
	switch value.shape {
	case '{':
		output.WriteByte('{')
		for index, field := range value.fields {
			if index > 0 {
				output.WriteByte(',')
			}
			output.WriteString(field.rawKey)
			output.WriteByte(':')
			field.value.write(output)
		}
		output.WriteByte('}')
	case '[':
		output.WriteByte('[')
		for index, child := range value.children {
			if index > 0 {
				output.WriteByte(',')
			}
			child.write(output)
		}
		output.WriteByte(']')
	default:
		output.WriteString(value.raw)
	}
}

func jsonError(src string, err error) error {
	var syntax *json.SyntaxError
	if !errors.As(err, &syntax) {
		return err
	}
	offset := int(syntax.Offset) - 1
	if syntax.Error() == "unexpected end of JSON input" {
		offset = len(src)
	} else if offset < 0 {
		var raw json.RawMessage
		if validation := json.Unmarshal([]byte(src), &raw); errors.As(validation, &syntax) {
			offset = int(syntax.Offset) - 1
		}
	}
	return positionError("JSON", src, offset, syntax.Error())
}

func positionError(format, src string, offset int, message string) error {
	line, column := 1, 1
	for index := 0; index < min(offset, len(src)); {
		switch src[index] {
		case '\r':
			line, column = line+1, 1
			index++
			if index < len(src) && src[index] == '\n' {
				index++
			}
		case '\n':
			line, column = line+1, 1
			index++
		default:
			_, width := utf8.DecodeRuneInString(src[index:])
			index += width
			column++
		}
	}
	return fmt.Errorf("%s at line %d, column %d: %s", format, line, column, message)
}
