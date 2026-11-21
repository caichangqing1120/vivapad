package printing

import (
	"math"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
	"github.com/vivaui/viva/text"
	"vivapad/internal/runcommand"
)

func testDevice() DeviceMetrics {
	return DeviceMetrics{PaperWidth: 95, PaperHeight: 100, HeaderLineHeight: 10, FooterLineHeight: 10}
}

func testTextMetrics() TextMetrics {
	return TextMetrics{
		LineHeight: 20, TabWidth: 4,
		Advance: func(cluster string) float64 { return float64(uniseg.StringWidth(cluster)) * 10 },
	}
}

func TestPaginateKeepsGraphemesTabsAndSourceLineOffsets(t *testing.T) {
	job := Job{Text: "a\t🙂b\n猫字\n", Settings: Settings{LineNumbers: true, ColorMode: ColorOnWhite}}
	pages, err := Paginate(job, testDevice(), testTextMetrics(), PageVariables{})
	if err != nil || len(pages) != 1 || len(pages[0].Rows) != 3 {
		t.Fatalf("pages = %+v, %v", pages, err)
	}
	for index, want := range []Row{
		{Text: "a   ", StartByte: 0, EndByte: 2, LineNumber: 1},
		{Text: "🙂b", StartByte: 2, EndByte: 7},
		{Text: "猫字", StartByte: 8, EndByte: 14, LineNumber: 2},
	} {
		if pages[0].Rows[index] != want {
			t.Fatalf("row %d = %+v, want %+v", index, pages[0].Rows[index], want)
		}
	}
	if pages[0].Number != 1 || pages[0].Bounds.Left != 15 || pages[0].TextBounds.Left != 35 || pages[0].TextBounds.Right != 80 {
		t.Fatalf("page geometry = %+v", pages[0])
	}
}

func TestPaginateSelectionPrecedesPageRangeAndKeepsOriginalLineNumbers(t *testing.T) {
	job := Job{Text: "first\nsecond\nthird\nfourth", Interactive: true, Scope: ScopeSelection,
		Selection: text.Span{Start: 6, End: 18}, Settings: Settings{LineNumbers: true}}
	device := testDevice()
	device.PaperWidth = 200
	device.PaperHeight = 50
	pages, err := Paginate(job, device, testTextMetrics(), PageVariables{})
	if err != nil || len(pages) != 2 || len(pages[0].Rows) != 1 || len(pages[1].Rows) != 1 ||
		pages[0].Rows[0] != (Row{Text: "second", StartByte: 6, EndByte: 12, LineNumber: 2}) ||
		pages[1].Rows[0] != (Row{Text: "third", StartByte: 13, EndByte: 18, LineNumber: 3}) {
		t.Fatalf("selected pages = %+v, %v", pages, err)
	}
	job.Scope, job.Selection = ScopeAll, text.Span{}
	all, err := Paginate(job, device, testTextMetrics(), PageVariables{})
	if err != nil || len(all) != 4 {
		t.Fatalf("all pages = %+v, %v", all, err)
	}
	selected, err := SelectPages(all, 2, 3)
	if err != nil || len(selected) != 2 || selected[0].Number != 2 || selected[1].Number != 3 {
		t.Fatalf("page range = %+v, %v", selected, err)
	}
	for _, bounds := range [][2]int{{0, 2}, {3, 2}, {5, 6}} {
		if _, err := SelectPages(all, bounds[0], bounds[1]); err == nil {
			t.Fatalf("accepted invalid page range %v", bounds)
		}
	}
}

func TestPaginateFormFeedAndHeaderFooterVariables(t *testing.T) {
	job := Job{Title: "/tmp/notes.txt", Text: "A\fB\nC", Settings: Settings{
		FormFeedPageBreak: true,
		Header: TextBand{
			Left:   "$(SHORT_DATE) $(FILE_NAME) page $(CURRENT_PRINTING_PAGE)",
			Middle: "$(CURRENT_WORD)", Right: "$(SHORT_DATE) / $(SHORT_DATE)",
		},
		Footer: TextBand{Right: "$(TIME) / $(CURRENT_PRINTING_PAGE)"},
	}}
	device := testDevice()
	device.PaperHeight = 100
	pages, err := Paginate(job, device, testTextMetrics(), PageVariables{
		ShortDate: "11/7/26", Time: "10:30",
		NPP: runcommand.Variables{Path: job.Title, Word: "p $(CURRENT_PRINTING_PAGE)"},
	})
	if err != nil || len(pages) != 2 || pages[0].Rows[0].Text != "A" ||
		len(pages[1].Rows) != 2 || pages[1].Rows[0].Text != "B" || pages[1].Rows[1].Text != "C" {
		t.Fatalf("form-feed pages = %+v, %v", pages, err)
	}
	if pages[0].Header.Left != "11/7/26 notes.txt page 1" || pages[1].Header.Left != "11/7/26 notes.txt page 2" ||
		pages[0].Header.Middle != "p 1" || pages[1].Header.Middle != "p 2" ||
		pages[0].Header.Right != "11/7/26 / $(SHORT_DATE)" || pages[1].Footer.Right != "10:30 / 2" {
		t.Fatalf("expanded print bands = %+v / %+v", pages[0], pages[1])
	}
}

func TestPaginateRejectsInvalidSelectionAndUnprintableAdvance(t *testing.T) {
	job := Job{Text: "猫", Interactive: true, Scope: ScopeSelection, Selection: text.Span{Start: 1, End: 3}}
	if _, err := Paginate(job, testDevice(), testTextMetrics(), PageVariables{}); err == nil {
		t.Fatal("accepted a selection starting inside a UTF-8 rune")
	}
	job.Scope, job.Selection = ScopeAll, text.Span{}
	bad := testTextMetrics()
	for _, change := range []func(*TextMetrics){
		func(metrics *TextMetrics) { metrics.LineHeight = 0 },
		func(metrics *TextMetrics) { metrics.Advance = nil },
		func(metrics *TextMetrics) { metrics.Advance = func(string) float64 { return 0 } },
		func(metrics *TextMetrics) { metrics.Advance = func(string) float64 { return math.NaN() } },
		func(metrics *TextMetrics) { metrics.Advance = func(string) float64 { return 1000 } },
	} {
		metrics := bad
		change(&metrics)
		if _, err := Paginate(job, testDevice(), metrics, PageVariables{}); err == nil {
			t.Fatalf("accepted invalid text metrics %+v", metrics)
		}
	}
	if pages, err := Paginate(Job{Text: ""}, testDevice(), testTextMetrics(), PageVariables{}); err != nil || len(pages) != 0 {
		t.Fatalf("empty source pages = %+v, %v", pages, err)
	}
}

func TestPaginatePreservesInitialAndConsecutiveFormFeedPages(t *testing.T) {
	job := Job{Text: "\f\fA", Settings: Settings{FormFeedPageBreak: true}}
	pages, err := Paginate(job, testDevice(), testTextMetrics(), PageVariables{})
	if err != nil || len(pages) != 3 || len(pages[0].Rows) != 0 || len(pages[1].Rows) != 0 ||
		len(pages[2].Rows) != 1 || pages[2].Rows[0] != (Row{Text: "A", StartByte: 2, EndByte: 3, LineNumber: 1}) {
		t.Fatalf("form-feed-only pages = %+v, %v", pages, err)
	}
}

func TestPaginateWrapsAtWordBreakWithoutSplittingEmojiFamily(t *testing.T) {
	device := testDevice()
	device.PaperWidth = 70
	pages, err := Paginate(Job{Text: "one two"}, device, testTextMetrics(), PageVariables{})
	if err != nil || len(pages) != 1 || len(pages[0].Rows) != 2 ||
		pages[0].Rows[0].Text != "one " || pages[0].Rows[1].Text != "two" {
		t.Fatalf("word-wrap rows = %+v, %v", pages, err)
	}
	family := "👨‍👩‍👧‍👦"
	device.PaperWidth = 55
	pages, err = Paginate(Job{Text: "a" + family + "b"}, device, testTextMetrics(), PageVariables{})
	if err != nil || len(pages) != 1 || len(pages[0].Rows) != 3 ||
		pages[0].Rows[1].Text != family || pages[0].Rows[1].StartByte != 1 || pages[0].Rows[1].EndByte != 1+len(family) {
		t.Fatalf("emoji cluster rows = %+v, %v", pages, err)
	}
}

func TestPaginateRejectsMalformedRangesAndTextMetrics(t *testing.T) {
	job := Job{Text: "abc", Interactive: true, Scope: ScopeSelection}
	for _, span := range []text.Span{{Start: -1, End: 2}, {Start: 2, End: 1}, {Start: 0, End: 4}} {
		job.Selection = span
		if _, err := Paginate(job, testDevice(), testTextMetrics(), PageVariables{}); err == nil {
			t.Fatalf("accepted invalid byte span %+v", span)
		}
	}
	job.Selection, job.Interactive = text.Span{Start: 0, End: 1}, false
	if _, err := Paginate(job, testDevice(), testTextMetrics(), PageVariables{}); err == nil {
		t.Fatal("Print Now accepted a selection scope")
	}
	job.Scope = Scope(255)
	if _, err := Paginate(job, testDevice(), testTextMetrics(), PageVariables{}); err == nil {
		t.Fatal("accepted unknown print scope")
	}
	job.Scope = ScopeAll
	for _, tabWidth := range []int{0, 17} {
		metrics := testTextMetrics()
		metrics.TabWidth = tabWidth
		if _, err := Paginate(job, testDevice(), metrics, PageVariables{}); err == nil {
			t.Fatalf("accepted tab width %d", tabWidth)
		}
	}
	job.Text = string([]byte{0xff})
	if _, err := Paginate(job, testDevice(), testTextMetrics(), PageVariables{}); err == nil {
		t.Fatal("accepted invalid UTF-8 source")
	}
}

func TestPaginateLongLogicalLineAdvancesByGraphemes(t *testing.T) {
	job := Job{Text: strings.Repeat("猫", 10000)}
	device := testDevice()
	device.PaperWidth = 70
	pages, err := Paginate(job, device, testTextMetrics(), PageVariables{})
	if err != nil || len(pages) == 0 || pages[0].Rows[0].Text != "猫猫" {
		t.Fatalf("long line pagination = %d pages, %v", len(pages), err)
	}
	last := pages[len(pages)-1].Rows
	if last[len(last)-1].EndByte != len(job.Text) {
		t.Fatal("long line lost its final UTF-8 bytes")
	}
}

func TestPaginateStopsMeasuringLongLineAtPageLimit(t *testing.T) {
	device := testDevice()
	device.PaperWidth = 45  // One character per row.
	device.PaperHeight = 50 // One row per page.
	metrics := testTextMetrics()
	measured := 0
	metrics.Advance = func(string) float64 {
		measured++
		return 10
	}
	job := Job{Text: strings.Repeat("a", 4*maxPrintPages)}
	pages, err := Paginate(job, device, metrics, PageVariables{})
	if err == nil || !strings.Contains(err.Error(), "too many pages") || len(pages) != 0 {
		t.Fatalf("oversized print job returned %d pages, err=%v", len(pages), err)
	}
	if measured > 2*(maxPrintPages+1) {
		t.Fatalf("measured %d glyphs for a job limited to %d one-glyph pages", measured, maxPrintPages)
	}
}

func TestPaginateRecalculatesTabAfterWrappingAtWordBreak(t *testing.T) {
	device := testDevice()
	device.PaperWidth = 90
	pages, err := Paginate(Job{Text: "aa bc\tz\n\n"}, device, testTextMetrics(), PageVariables{})
	if err != nil || len(pages) != 1 || len(pages[0].Rows) != 3 {
		t.Fatalf("wrong page or row count: %+v, %v", pages, err)
	}
	for i, want := range []Row{
		{Text: "aa ", StartByte: 0, EndByte: 3, LineNumber: 1},
		{Text: "bc  z", StartByte: 3, EndByte: 7},
		{StartByte: 8, EndByte: 8, LineNumber: 2},
	} {
		if got := pages[0].Rows[i]; got != want {
			t.Errorf("row %d = %+v, want %+v", i, got, want)
		}
	}
}
