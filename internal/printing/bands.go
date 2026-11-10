package printing

import (
	"strconv"
	"strings"

	"vivapad/internal/runcommand"
)

func expandBand(band TextBand, page int, variables PageVariables) TextBand {
	expand := func(value string) string {
		for _, replacement := range [][2]string{
			{"$(SHORT_DATE)", variables.ShortDate},
			{"$(LONG_DATE)", variables.LongDate},
			{"$(TIME)", variables.Time},
		} {
			value = strings.Replace(value, replacement[0], replacement[1], 1)
		}
		value = runcommand.ExpandVariables(value, variables.NPP)
		value = limitPrintBand(value)
		return strings.Replace(value, "$(CURRENT_PRINTING_PAGE)", strconv.Itoa(page), 1)
	}
	return TextBand{Left: expand(band.Left), Middle: expand(band.Middle), Right: expand(band.Right)}
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
