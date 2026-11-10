package printing

import (
	"fmt"
	"math"
)

type Insets struct {
	Left, Top, Right, Bottom float64
}

type DeviceMetrics struct {
	PaperWidth, PaperHeight float64
	PhysicalMargins         Insets
	HeaderLineHeight        float64
	FooterLineHeight        float64
}

type PageBounds struct {
	Left, Top, Right, Bottom float64
}

func BodyBounds(job Job, device DeviceMetrics) (PageBounds, error) {
	valid := func(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 }
	physical := device.PhysicalMargins
	user := job.Settings.MarginMM
	if !valid(device.PaperWidth) || !valid(device.PaperHeight) || device.PaperWidth == 0 || device.PaperHeight == 0 ||
		!valid(device.HeaderLineHeight) || !valid(device.FooterLineHeight) || device.HeaderLineHeight == 0 || device.FooterLineHeight == 0 ||
		!valid(physical.Left) || !valid(physical.Top) || !valid(physical.Right) || !valid(physical.Bottom) ||
		user.Left < 0 || user.Top < 0 || user.Right < 0 || user.Bottom < 0 {
		return PageBounds{}, fmt.Errorf("printing: invalid printer or margin metrics")
	}
	const pointsPerMM = 72.0 / 25.4
	padding := device.FooterLineHeight * 1.5
	bounds := PageBounds{
		Left:   max(physical.Left, float64(user.Left)*pointsPerMM) + padding,
		Top:    max(physical.Top, float64(user.Top)*pointsPerMM) + padding,
		Right:  device.PaperWidth - max(physical.Right, float64(user.Right)*pointsPerMM) - padding,
		Bottom: device.PaperHeight - max(physical.Bottom, float64(user.Bottom)*pointsPerMM) - padding,
	}
	if job.Settings.Header != (TextBand{}) {
		bounds.Top += device.HeaderLineHeight * 1.5
	}
	if job.Settings.Footer != (TextBand{}) {
		bounds.Bottom -= device.FooterLineHeight * 1.5
	}
	if bounds.Right <= bounds.Left || bounds.Bottom <= bounds.Top {
		return PageBounds{}, fmt.Errorf("printing: printer has no printable body area")
	}
	return bounds, nil
}
