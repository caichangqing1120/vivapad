package datetime

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
)

type Kind uint8

const (
	Short Kind = iota + 1
	Long
	Customized
)

const DefaultFormat = "yyyy-MM-dd HH:mm:ss"

func ValidatePattern(pattern string) error {
	if pattern == "" || utf8.RuneCountInString(pattern) > 128 || !utf8.ValidString(pattern) {
		return fmt.Errorf("date time format must contain 1 to 128 valid characters")
	}
	quoted := false
	runes := []rune(pattern)
	for index := 0; index < len(runes); index++ {
		character := runes[index]
		if unicode.IsControl(character) {
			return fmt.Errorf("date time format contains a control character")
		}
		if character == '\'' {
			if quoted && index+1 < len(runes) && runes[index+1] == '\'' {
				index++
			} else {
				quoted = !quoted
			}
		}
	}
	if quoted {
		return fmt.Errorf("date time format has an unclosed quote")
	}
	return nil
}

func Format(now time.Time, kind Kind, custom string, reverse bool, locale language.Tag) (string, error) {
	if kind == Customized {
		if err := ValidatePattern(custom); err != nil {
			return "", err
		}
		return formatCustom(now, custom, locale), nil
	}
	if kind != Short && kind != Long {
		return "", fmt.Errorf("unsupported date time format %d", kind)
	}
	date, clock, err := standardParts(now, kind, locale)
	if err != nil {
		return "", err
	}
	if reverse {
		return date + " " + clock, nil
	}
	return clock + " " + date, nil
}

func formatCustom(now time.Time, pattern string, locale language.Tag) string {
	var output strings.Builder
	runes := []rune(pattern)
	quoted := false
	for index := 0; index < len(runes); {
		character := runes[index]
		if character == '\'' {
			if quoted && index+1 < len(runes) && runes[index+1] == '\'' {
				output.WriteRune('\'')
				index += 2
			} else {
				quoted = !quoted
				index++
			}
			continue
		}
		if quoted || !strings.ContainsRune("yMdHhmst", character) {
			output.WriteRune(character)
			index++
			continue
		}
		end := index + 1
		for end < len(runes) && runes[end] == character {
			end++
		}
		output.WriteString(formatToken(now, character, end-index, locale))
		index = end
	}
	return output.String()
}

func formatToken(now time.Time, token rune, count int, locale language.Tag) string {
	pad := func(value int) string {
		if count > 1 {
			return fmt.Sprintf("%02d", value)
		}
		return fmt.Sprint(value)
	}
	switch token {
	case 'y':
		if count <= 2 {
			return pad(now.Year() % 100)
		}
		return fmt.Sprintf("%04d", now.Year())
	case 'M':
		if count == 3 {
			return now.Format("Jan")
		}
		if count >= 4 {
			return now.Format("January")
		}
		return pad(int(now.Month()))
	case 'd':
		if count == 3 {
			return now.Format("Mon")
		}
		if count >= 4 {
			return now.Format("Monday")
		}
		return pad(now.Day())
	case 'H':
		return pad(now.Hour())
	case 'h':
		hour := now.Hour() % 12
		if hour == 0 {
			hour = 12
		}
		return pad(hour)
	case 'm':
		return pad(now.Minute())
	case 's':
		return pad(now.Second())
	case 't':
		value := "AM"
		if now.Hour() >= 12 {
			value = "PM"
		}
		base, _ := locale.Base()
		if base.String() == "zh" {
			if now.Hour() >= 12 {
				value = "下午"
			} else {
				value = "上午"
			}
		}
		if count == 1 {
			return string([]rune(value)[:1])
		}
		return value
	}
	return ""
}
