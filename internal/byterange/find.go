package byterange

import "fmt"

func Find(value string, caret, start, end int, backward, wrap bool) (int, bool, error) {
	if start < 0 || end > 255 || start > end {
		return 0, false, fmt.Errorf("enter a byte range from 0 to 255")
	}
	if caret < 0 || caret > len(value) {
		return 0, false, fmt.Errorf("caret is outside the document")
	}
	if len(value) == 0 {
		return 0, false, nil
	}
	matches := func(index int) bool { return int(value[index]) >= start && int(value[index]) <= end }
	if backward {
		for index := caret - 1; index >= 0; index-- {
			if matches(index) {
				return index, true, nil
			}
		}
		if wrap {
			for index := len(value) - 1; index >= 0; index-- {
				if matches(index) {
					return index, true, nil
				}
			}
		}
		return 0, false, nil
	}
	for index := caret; index < len(value); index++ {
		if matches(index) {
			return index, true, nil
		}
	}
	if wrap {
		for index := 0; index < len(value); index++ {
			if matches(index) {
				return index, true, nil
			}
		}
	}
	return 0, false, nil
}
