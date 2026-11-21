package printing

import (
	"fmt"
	"strings"

	"github.com/rivo/uniseg"
	"github.com/vivaui/viva/gfx"
)

type Segment struct {
	Text  string
	Color gfx.Color
}

func printBodyBackground(settings Settings) bool {
	return settings.ColorMode == ColorInvertLight
}

func Segments(job Job, row Row, styles []ColorRun, tabWidth int) ([]Segment, error) {
	if tabWidth < 1 || tabWidth > 16 || row.StartByte < 0 || row.EndByte < row.StartByte || row.EndByte > len(job.Text) ||
		!runeBoundary(job.Text, row.StartByte) || !runeBoundary(job.Text, row.EndByte) {
		return nil, fmt.Errorf("printing: invalid styled row")
	}
	last := row.StartByte
	for _, style := range styles {
		if style.StartByte < last || style.EndByte <= style.StartByte || style.EndByte > row.EndByte ||
			!runeBoundary(job.Text, style.StartByte) || !runeBoundary(job.Text, style.EndByte) {
			return nil, fmt.Errorf("printing: invalid row color span")
		}
		last = style.EndByte
	}
	defaultColor := gfx.RGB(0, 0, 0)
	var segments []Segment
	var current strings.Builder
	var currentColor gfx.Color
	hasCurrent := false
	flush := func() {
		if hasCurrent {
			segments = append(segments, Segment{Text: current.String(), Color: currentColor})
			current.Reset()
		}
	}
	columns, styleIndex := 0, 0
	for graphemes := uniseg.NewGraphemes(job.Text[row.StartByte:row.EndByte]); graphemes.Next(); {
		start, _ := graphemes.Positions()
		absolute := row.StartByte + start
		for styleIndex < len(styles) && styles[styleIndex].EndByte <= absolute {
			styleIndex++
		}
		color := defaultColor
		if styleIndex < len(styles) && styles[styleIndex].StartByte <= absolute {
			color = styles[styleIndex].Color
		}
		printed, nextColumns := expandCluster(graphemes.Str(), columns, tabWidth)
		columns = nextColumns
		if !hasCurrent || currentColor != color {
			flush()
			currentColor, hasCurrent = color, true
		}
		current.WriteString(printed)
	}
	flush()
	var rendered strings.Builder
	for _, segment := range segments {
		rendered.WriteString(segment.Text)
	}
	if rendered.String() != row.Text {
		return nil, fmt.Errorf("printing: styled text differs from paginated row")
	}
	for index := range segments {
		switch job.Settings.ColorMode {
		case ColorBlackOnWhite:
			segments[index].Color = gfx.RGB(0, 0, 0)
		case ColorInvertLight:
			color := segments[index].Color
			segments[index].Color = gfx.Color{Red: 1 - color.Red, Green: 1 - color.Green, Blue: 1 - color.Blue, Alpha: color.Alpha}
		}
	}
	return segments, nil
}
