package printing

import (
	"math"
	"testing"
)

func TestBodyBoundsFollowPhysicalMarginsAndSourceInnerSpacing(t *testing.T) {
	job := Job{Settings: Settings{LineNumbers: true, ColorMode: ColorOnWhite}}
	device := DeviceMetrics{
		PaperWidth: 612, PaperHeight: 792,
		PhysicalMargins:  Insets{Left: 18, Top: 24, Right: 18, Bottom: 30},
		HeaderLineHeight: 10, FooterLineHeight: 9,
	}
	bounds, err := BodyBounds(job, device)
	if err != nil || bounds != (PageBounds{Left: 31.5, Top: 37.5, Right: 580.5, Bottom: 748.5}) {
		t.Fatalf("default body bounds = %+v, %v", bounds, err)
	}
	job.Settings.Header.Left = "file name"
	job.Settings.Footer.Right = "page"
	bounds, err = BodyBounds(job, device)
	if err != nil || bounds != (PageBounds{Left: 31.5, Top: 52.5, Right: 580.5, Bottom: 735}) {
		t.Fatalf("header/footer body bounds = %+v, %v", bounds, err)
	}
}

func TestBodyBoundsUsesLargerUserMarginsAndRejectsNoPrintableArea(t *testing.T) {
	job := Job{Settings: Settings{MarginMM: Margins{Left: 25, Top: 10, Right: 20, Bottom: 15}}}
	device := DeviceMetrics{
		PaperWidth: 200, PaperHeight: 200,
		PhysicalMargins:  Insets{Left: 12, Top: 30, Right: 14, Bottom: 8},
		HeaderLineHeight: 10, FooterLineHeight: 10,
	}
	bounds, err := BodyBounds(job, device)
	if err != nil || math.Abs(bounds.Left-(25*72/25.4+15)) > 0.001 || bounds.Top != 45 ||
		math.Abs(bounds.Right-(200-20*72/25.4-15)) > 0.001 ||
		math.Abs(bounds.Bottom-(200-15*72/25.4-15)) > 0.001 {
		t.Fatalf("user margin bounds = %+v, %v", bounds, err)
	}
	for _, invalid := range []DeviceMetrics{
		{PaperWidth: 30, PaperHeight: 30, HeaderLineHeight: 10, FooterLineHeight: 10},
		{PaperWidth: math.NaN(), PaperHeight: 200, HeaderLineHeight: 10, FooterLineHeight: 10},
		{PaperWidth: 200, PaperHeight: 200, PhysicalMargins: Insets{Left: -1}, HeaderLineHeight: 10, FooterLineHeight: 10},
		{PaperWidth: 200, PaperHeight: 200, HeaderLineHeight: 0, FooterLineHeight: 10},
	} {
		if _, err := BodyBounds(job, invalid); err == nil {
			t.Fatalf("accepted unprintable device %+v", invalid)
		}
	}
	job.Settings.MarginMM.Left = -1
	if _, err := BodyBounds(job, device); err == nil {
		t.Fatal("accepted negative user margin")
	}
}
