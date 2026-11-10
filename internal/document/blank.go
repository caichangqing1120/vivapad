package document

import "strings"

type BlankOperation uint8

const (
	BlankTrimTrailing BlankOperation = iota + 1
	BlankTrimLeading
	BlankTrimBoth
	BlankEOLToSpace
	BlankTrimAndJoin
	BlankTabToSpace
	BlankSpaceToTabAll
	BlankSpaceToTabLeading
)

func joinNewlines(value string, previous byte) string {
	var result strings.Builder
	result.Grow(len(value))
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '\n' {
			if previous != ' ' {
				result.WriteByte(' ')
				previous = ' '
			}
			continue
		}
		result.WriteByte(character)
		previous = character
	}
	return result.String()
}

func tabsToSpaces(line string, width int) string {
	var output strings.Builder
	output.Grow(len(line))
	column := 0
	for _, character := range line {
		if character == '\t' {
			spaces := width - column%width
			output.WriteString(strings.Repeat(" ", spaces))
			column += spaces
		} else {
			output.WriteRune(character)
			column++
		}
	}
	return output.String()
}
