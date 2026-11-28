package document

import "strings"

var editorNewlines = strings.NewReplacer("\r\n", "\n", "\r", "\n")

func MultiCursorPasteParts(value string, cursors int) []string {
	if strings.ContainsRune(value, '\r') {
		value = editorNewlines.Replace(value)
	}
	if cursors > 1 && strings.Count(value, "\n") == cursors-1 {
		return strings.Split(value, "\n")
	}
	return []string{value}
}
