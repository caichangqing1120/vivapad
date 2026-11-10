package document

import "strings"

func convertBlankLine(line string, width int, operation BlankOperation, caret int) (string, int) {
	if operation == BlankTabToSpace {
		if !strings.ContainsRune(line, '\t') {
			return line, 0
		}
		position := 0
		if caret >= 0 {
			position = len(tabsToSpaces(line[:min(caret, len(line))], width))
		}
		return tabsToSpaces(line, width), position
	}
	return spacesToTabsAt(line, width, operation == BlankSpaceToTabLeading, caret)
}

func spacesToTabsAt(line string, width int, leadingOnly bool, caret int) (string, int) {
	var output strings.Builder
	output.Grow(len(line))
	column, tabStop, counter, position := 0, width-1, 0, 0
	nonSpaceFound := false
	characterAt := func(index int) byte {
		if index >= len(line) {
			return 0
		}
		return line[index]
	}
	for index := 0; index < len(line); index++ {
		nextCharacter := false
		if !nonSpaceFound {
			for characterAt(index+counter) == ' ' {
				if column+counter != tabStop {
					counter++
					continue
				}
				tabStop += width
				if counter >= 1 {
					output.WriteByte('\t')
					index += counter
					column += counter + 1
					counter = 0
					nextCharacter = true
					if index <= caret {
						position++
					}
					break
				}
				if characterAt(index+1) == ' ' || characterAt(index+1) == '\t' {
					output.WriteByte('\t')
					index++
					column++
					counter = 0
					if index <= caret {
						position++
					}
				} else {
					output.WriteByte(line[index])
					column++
					counter = 0
					nextCharacter = true
					if index <= caret {
						position++
					}
					break
				}
			}
			if nextCharacter {
				continue
			}
			if characterAt(index) == ' ' && characterAt(index+counter) == '\t' {
				output.WriteByte('\t')
				index += counter
				column = tabStop + 1
				tabStop += width
				counter = 0
				if index <= caret {
					position++
				}
				continue
			}
		}
		if leadingOnly && !nonSpaceFound {
			nonSpaceFound = true
		}
		character := line[index]
		output.WriteByte(character)
		switch character {
		case '\n', '\r':
			column, tabStop, nonSpaceFound = 0, width-1, false
		case '\t':
			column = tabStop + 1
			tabStop += width
			counter = 0
		default:
			counter = 0
			if character&0xc0 != 0x80 {
				column++
				if column > 0 && column%width == 0 {
					tabStop += width
				}
			}
		}
		if index < caret {
			position++
		}
	}
	return output.String(), position
}
