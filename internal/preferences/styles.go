package preferences

import (
	"fmt"
	"strconv"
)

type StyleColors [7]string

type Styles struct {
	Light   StyleColors `json:"classic-light,omitzero"`
	Dark    StyleColors `json:"modern-dark,omitzero"`
	Monokai StyleColors `json:"monokai,omitzero"`
}

func (styles Styles) For(theme Theme) StyleColors {
	return *styles.ForTheme(theme)
}

func (styles *Styles) ForTheme(theme Theme) *StyleColors {
	switch theme {
	case ModernDark:
		return &styles.Dark
	case Monokai:
		return &styles.Monokai
	default:
		return &styles.Light
	}
}

func ParseStyleColor(value string) (uint32, bool) {
	if len(value) != 7 || value[0] != '#' {
		return 0, false
	}
	for _, digit := range value[1:] {
		if !(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f' || digit >= 'A' && digit <= 'F') {
			return 0, false
		}
	}
	parsed, err := strconv.ParseUint(value[1:], 16, 24)
	return uint32(parsed), err == nil
}

func (colors StyleColors) Validate() error {
	for _, color := range colors {
		if _, ok := ParseStyleColor(color); color != "" && !ok {
			return fmt.Errorf("invalid color %q; expected #RRGGBB", color)
		}
	}
	return nil
}
