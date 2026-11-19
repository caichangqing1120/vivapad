package printing

import (
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/vivaui/viva/text"
)

func TestPortalPrintUnwrapsDBusVariantDictionaries(t *testing.T) {
	result := portalValues(map[string]dbus.Variant{
		"token": dbus.MakeVariant(uint32(9)),
		"settings": dbus.MakeVariant(map[string]dbus.Variant{
			"print-pages": dbus.MakeVariant("all"),
		}),
		"page-setup": dbus.MakeVariant(map[string]dbus.Variant{
			"Width": dbus.MakeVariant(210.0), "Height": dbus.MakeVariant(297.0),
			"MarginLeft": dbus.MakeVariant(5.0), "MarginTop": dbus.MakeVariant(5.0),
			"MarginRight": dbus.MakeVariant(5.0), "MarginBottom": dbus.MakeVariant(5.0),
		}),
	})
	parsed, err := parsePortalPrintResult(Job{Interactive: true}, result)
	if err != nil || parsed.token != 9 || parsed.device.PaperWidth < 595 {
		t.Fatalf("D-Bus portal response = %+v, %v", parsed, err)
	}
}

func TestPortalPrintResultUsesPhysicalPageSelectionAndDisjointRanges(t *testing.T) {
	job := Job{Interactive: true, Text: "one\ntwo", Selection: text.Span{Start: 0, End: 3}}
	result, err := parsePortalPrintResult(job, map[string]any{
		"token":    uint32(73),
		"settings": map[string]any{"print-pages": "ranges", "page-ranges": "0-2,4,9-11", "n-copies": "2"},
		"page-setup": map[string]any{
			"Width": 210.0, "Height": 297.0, "Orientation": "portrait",
			"MarginLeft": 5.0, "MarginTop": 6.0, "MarginRight": 7.0, "MarginBottom": 8.0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.token != 73 || result.choice.Scope != ScopeAll || result.choice.Copies != 2 ||
		!reflect.DeepEqual(result.choice.Ranges, []PageRange{{1, 3}, {5, 5}, {10, 12}}) {
		t.Fatalf("portal print choice = %+v", result)
	}
	if result.device.PaperWidth < 595 || result.device.PaperWidth > 596 ||
		result.device.PaperHeight < 841 || result.device.PaperHeight > 843 ||
		result.device.PhysicalMargins.Left < 14 || result.device.PhysicalMargins.Left > 15 {
		t.Fatalf("portal paper dimensions = %+v", result.device)
	}
	pages := make([]Page, 12)
	for index := range pages {
		pages[index].Number = index + 1
	}
	selected, err := SelectPageRanges(pages, result.choice.Ranges)
	if err != nil || len(selected) != 7 || selected[3].Number != 5 || selected[6].Number != 12 {
		t.Fatalf("disjoint selection = %+v, %v", selected, err)
	}
}

func TestPortalPrintRejectsRangeEndingAfterLastPage(t *testing.T) {
	// The portal can return syntactically valid ranges before VivaPad paginates
	// against its chosen printer. The second range asks for a page we don't have.
	ranges, err := parsePortalRanges("0,2-4")
	if err != nil {
		t.Fatal(err)
	}
	pages := []Page{{Number: 1}, {Number: 2}, {Number: 3}, {Number: 4}}
	if selected, err := SelectPageRanges(pages, ranges); err == nil || len(selected) != 0 {
		t.Fatalf("out-of-bounds portal range was accepted: pages=%v, err=%v", selected, err)
	}
	if all, err := SelectPages(pages, 1, maxPrintPages); err != nil || !reflect.DeepEqual(all, pages) {
		t.Fatalf("default print-all bounds changed: pages=%v, err=%v", all, err)
	}
}

func TestPortalPrintSelectionAndLandscapeMargins(t *testing.T) {
	job := Job{Interactive: true, Text: "alpha", Selection: text.Span{Start: 0, End: 3}}
	result, err := parsePortalPrintResult(job, map[string]any{
		"token": uint32(8), "settings": map[string]any{"print-pages": "selection", "n-copies": "1"},
		"page-setup": map[string]any{
			"Width": 210.0, "Height": 297.0, "Orientation": "landscape",
			"MarginLeft": 1.0, "MarginTop": 2.0, "MarginRight": 3.0, "MarginBottom": 4.0,
		},
	})
	if err != nil || result.choice.Scope != ScopeSelection || result.device.PaperWidth < 841 || result.device.PaperHeight > 596 ||
		result.device.PhysicalMargins.Left < 5 || result.device.PhysicalMargins.Left > 6 ||
		result.device.PhysicalMargins.Top < 2 || result.device.PhysicalMargins.Top > 3 {
		t.Fatalf("landscape choice = %+v, %v", result, err)
	}
}

func TestPortalPrintRejectsInvalidSettingsBeforeSubmission(t *testing.T) {
	job := Job{Interactive: true, Text: "abc"}
	valid := map[string]any{
		"token": uint32(1), "settings": map[string]any{"print-pages": "all"},
		"page-setup": map[string]any{
			"Width": 210.0, "Height": 297.0, "MarginLeft": 5.0,
			"MarginTop": 5.0, "MarginRight": 5.0, "MarginBottom": 5.0,
		},
	}
	for _, change := range []map[string]any{
		{"token": uint32(0)},
		{"settings": map[string]any{"print-pages": "selection"}},
		{"settings": map[string]any{"print-pages": "ranges", "page-ranges": "1-0"}},
		{"settings": map[string]any{"print-pages": "ranges", "page-ranges": "0-2,2-3"}},
		{"settings": map[string]any{"print-pages": "current"}},
		{"page-setup": map[string]any{"Width": 210.0, "Height": 297.0, "MarginLeft": -1.0}},
	} {
		candidate := make(map[string]any, len(valid))
		for key, value := range valid {
			candidate[key] = value
		}
		for key, value := range change {
			candidate[key] = value
		}
		if _, err := parsePortalPrintResult(job, candidate); err == nil {
			t.Fatalf("invalid portal result accepted: %+v", change)
		}
	}
}
