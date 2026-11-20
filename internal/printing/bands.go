package printing

import (
	"strconv"
	"strings"

	"vivapad/internal/runcommand"
)

func prepareBand(band TextBand, variables PageVariables) TextBand {
	expand := func(value string) string {
		if value == "" {
			return ""
		}
		for _, replacement := range [][2]string{
			{"$(SHORT_DATE)", variables.ShortDate},
			{"$(LONG_DATE)", variables.LongDate},
			{"$(TIME)", variables.Time},
		} {
			value = strings.Replace(value, replacement[0], replacement[1], 1)
		}
		return limitPrintBand(runcommand.ExpandVariables(value, variables.NPP))
	}
	return TextBand{Left: expand(band.Left), Middle: expand(band.Middle), Right: expand(band.Right)}
}

func numberBand(band TextBand, page int) TextBand {
	const token = "$(CURRENT_PRINTING_PAGE)"
	if !strings.Contains(band.Left, token) && !strings.Contains(band.Middle, token) && !strings.Contains(band.Right, token) {
		return band
	}
	number := strconv.Itoa(page)
	return TextBand{
		Left:   strings.Replace(band.Left, token, number, 1),
		Middle: strings.Replace(band.Middle, token, number, 1),
		Right:  strings.Replace(band.Right, token, number, 1),
	}
}

func limitPrintBand(value string) string {
	units := 0
	for offset, character := range value {
		width := 1
		if character > 0xffff {
			width = 2
		}
		if units+width > 255 {
			return value[:offset]
		}
		units += width
	}
	return value
}
