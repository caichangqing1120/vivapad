package document

import (
	"bytes"
	"errors"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

//go:generate go run ../../tools/gb18030maps -out gb18030_table_generated.go

type gb18030PUARange struct {
	scalar, pointer, count uint16
}

func gb18030PUAEncode(value rune) (uint16, bool) {
	if value < 0xe000 || value > 0xf8ff {
		return 0, false
	}
	low, high := 0, len(gb18030PUARanges)
	for low < high {
		middle := (low + high) / 2
		if rune(gb18030PUARanges[middle].scalar) <= value {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low == 0 {
		return 0, false
	}
	interval := gb18030PUARanges[low-1]
	offset := value - rune(interval.scalar)
	if offset >= rune(interval.count) {
		return 0, false
	}
	pointer := int(interval.pointer) + int(offset)
	trail := pointer%190 + 0x40
	if trail >= 0x7f {
		trail++
	}
	return uint16(pointer/190+0x81)<<8 | uint16(trail), true
}

func gb18030PUADecode(lead, trail byte) (rune, bool) {
	if lead < 0xa1 || lead > 0xaf && lead != 0xd7 && lead < 0xf8 {
		return 0, false
	}
	pointer := int(lead-0x81)*190 + int(trail) - 0x40
	if trail >= 0x80 {
		pointer--
	}
	low, high := 0, len(gb18030PUAByPointer)
	for low < high {
		middle := (low + high) / 2
		if int(gb18030PUARanges[gb18030PUAByPointer[middle]].pointer) <= pointer {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low == 0 {
		return 0, false
	}
	interval := gb18030PUARanges[gb18030PUAByPointer[low-1]]
	offset := pointer - int(interval.pointer)
	if offset >= int(interval.count) {
		return 0, false
	}
	return rune(interval.scalar) + rune(offset), true
}

func encodeGB18030(input []byte) ([]byte, error) {
	encoder := simplifiedchinese.GB18030.NewEncoder()
	// Every repaired scalar is U+E000..U+E864, whose UTF-8 lead is 0xEE.
	if bytes.IndexByte(input, 0xee) < 0 {
		if !utf8.Valid(input) {
			return nil, errors.New("invalid UTF-8 text")
		}
		return encoder.Bytes(input)
	}
	var result []byte
	start := 0
	for at := 0; at < len(input); {
		value, width := rune(input[at]), 1
		if value >= utf8.RuneSelf {
			value, width = utf8.DecodeRune(input[at:])
			if value == utf8.RuneError && width == 1 {
				return nil, errors.New("invalid UTF-8 text")
			}
		}
		if pair, ok := gb18030PUAEncode(value); ok {
			if result == nil {
				result = make([]byte, 0, len(input))
			}
			if at > start {
				updated, _, err := transform.Append(encoder, result, input[start:at])
				if err != nil {
					return nil, err
				}
				result = updated
			}
			result = append(result, byte(pair>>8), byte(pair))
			start = at + width
		}
		at += width
	}
	if result == nil {
		return encoder.Bytes(input)
	}
	if start < len(input) {
		updated, _, err := transform.Append(encoder, result, input[start:])
		if err != nil {
			return nil, err
		}
		result = updated
	}
	return result, nil
}

func decodeGB18030(input []byte) ([]byte, error) {
	decoder := simplifiedchinese.GB18030.NewDecoder()
	ascii := true
	for _, value := range input {
		if value >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return decoder.Bytes(input)
	}
	var result []byte
	start := 0
	for at := 0; at < len(input); {
		width, err := gb18030UnitSize(input[at:])
		if err != nil {
			return nil, err
		}
		if width == 2 {
			if value, ok := gb18030PUADecode(input[at], input[at+1]); ok {
				if result == nil {
					result = make([]byte, 0, len(input))
				}
				if at > start {
					updated, _, err := transform.Append(decoder, result, input[start:at])
					if err != nil {
						return nil, err
					}
					result = updated
				}
				result = utf8.AppendRune(result, value)
				start = at + width
			}
		}
		at += width
	}
	if result == nil {
		return decoder.Bytes(input)
	}
	if start < len(input) {
		updated, _, err := transform.Append(decoder, result, input[start:])
		if err != nil {
			return nil, err
		}
		result = updated
	}
	return result, nil
}

// Complete units keep PUA byte pairs from matching across adjacent characters.
func gb18030UnitSize(input []byte) (int, error) {
	if input[0] <= 0x80 {
		return 1, nil
	}
	if input[0] < 0xff && len(input) >= 2 {
		second := input[1]
		if second >= 0x40 && second <= 0xfe && second != 0x7f {
			return 2, nil
		}
		if second >= 0x30 && second <= 0x39 && len(input) >= 4 && input[2] >= 0x81 && input[2] <= 0xfe && input[3] >= 0x30 && input[3] <= 0x39 {
			return 4, nil
		}
	}
	return 0, errors.New("invalid GB18030 sequence")
}
