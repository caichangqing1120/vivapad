package printing

import (
	"math"
	"testing"
	"unsafe"
)

func TestCUPSMediaMetricsUseHundredthsOfMillimeters(t *testing.T) {
	if got := unsafe.Sizeof(cupsSize{}); got != 152 {
		t.Fatalf("CUPS media ABI size = %d", got)
	}
	media := cupsSize{Width: 21000, Length: 29700, Left: 500, Top: 600, Right: 700, Bottom: 800}
	device, err := cupsDeviceMetrics(media)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(device.PaperWidth-595.2756) > 0.01 || math.Abs(device.PaperHeight-841.8898) > 0.01 ||
		math.Abs(device.PhysicalMargins.Left-14.1732) > 0.01 || math.Abs(device.PhysicalMargins.Bottom-22.6772) > 0.01 {
		t.Fatalf("CUPS media metrics = %+v", device)
	}
}

func TestCUPSMediaRejectsMissingOrImpossiblePrintArea(t *testing.T) {
	for _, media := range []cupsSize{
		{},
		{Width: 21000, Length: 29700, Left: -1},
		{Width: 21000, Length: 29700, Left: 12000, Right: 9000},
		{Width: 21000, Length: 29700, Top: 20000, Bottom: 9700},
		{Width: 21000, Length: 29700, Left: math.MaxInt32, Right: math.MaxInt32},
	} {
		if _, err := cupsDeviceMetrics(media); err == nil {
			t.Fatalf("invalid CUPS media accepted: %+v", media)
		}
	}
}
