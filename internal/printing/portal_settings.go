package printing

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

type portalPrintResult struct {
	token  uint32
	device DeviceMetrics
	choice PrintChoice
}

func parsePortalPrintResult(job Job, result map[string]any) (portalPrintResult, error) {
	token, ok := result["token"].(uint32)
	if !ok || token == 0 {
		return portalPrintResult{}, fmt.Errorf("printing: portal did not return a print token")
	}
	settings, ok := result["settings"].(map[string]any)
	if !ok {
		return portalPrintResult{}, fmt.Errorf("printing: portal did not return print settings")
	}
	setup, ok := result["page-setup"].(map[string]any)
	if !ok {
		return portalPrintResult{}, fmt.Errorf("printing: portal did not return page setup")
	}
	const pointsPerMM = 72.0 / 25.4
	field := func(key string) (float64, error) {
		value, ok := setup[key].(float64)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return 0, fmt.Errorf("printing: invalid portal page setup %s", key)
		}
		return value * pointsPerMM, nil
	}
	var values [6]float64
	for index, key := range []string{"Width", "Height", "MarginLeft", "MarginTop", "MarginRight", "MarginBottom"} {
		value, err := field(key)
		if err != nil {
			return portalPrintResult{}, err
		}
		values[index] = value
	}
	width, height, left, top, right, bottom := values[0], values[1], values[2], values[3], values[4], values[5]
	switch setup["Orientation"] {
	case nil, "portrait":
	case "reverse-portrait":
		left, right, top, bottom = right, left, bottom, top
	case "landscape":
		width, height, left, top, right, bottom = height, width, top, left, bottom, right
	case "reverse-landscape":
		width, height, left, top, right, bottom = height, width, bottom, right, top, left
	default:
		return portalPrintResult{}, fmt.Errorf("printing: unsupported portal orientation")
	}
	if width <= 0 || height <= 0 || left+right >= width || top+bottom >= height {
		return portalPrintResult{}, fmt.Errorf("printing: portal page has no printable area")
	}
	choice := PrintChoice{FirstPage: 1, LastPage: maxPrintPages, Copies: 1}
	if copies, ok := settings["n-copies"]; ok {
		value, err := strconv.Atoi(fmt.Sprint(copies))
		if err != nil || value < 1 || value > 9999 {
			return portalPrintResult{}, fmt.Errorf("printing: invalid portal copy count")
		}
		choice.Copies = value
	}
	switch settings["print-pages"] {
	case nil, "all":
	case "selection":
		if !job.Interactive || job.Selection.End <= job.Selection.Start {
			return portalPrintResult{}, fmt.Errorf("printing: portal selection is unavailable")
		}
		choice.Scope = ScopeSelection
	case "ranges":
		raw, ok := settings["page-ranges"].(string)
		if !ok {
			return portalPrintResult{}, fmt.Errorf("printing: missing portal page ranges")
		}
		ranges, err := parsePortalRanges(raw)
		if err != nil {
			return portalPrintResult{}, err
		}
		choice.Ranges = ranges
	default:
		return portalPrintResult{}, fmt.Errorf("printing: unsupported portal page selection")
	}
	return portalPrintResult{token: token, device: DeviceMetrics{
		PaperWidth: width, PaperHeight: height,
		PhysicalMargins: Insets{Left: left, Top: top, Right: right, Bottom: bottom},
	}, choice: choice}, nil
}

func parsePortalRanges(value string) ([]PageRange, error) {
	if value == "" {
		return nil, fmt.Errorf("printing: empty portal page ranges")
	}
	var ranges []PageRange
	for _, part := range strings.Split(value, ",") {
		start, end, hasEnd := strings.Cut(part, "-")
		first, err := strconv.Atoi(start)
		if err != nil || first < 0 || first >= maxPrintPages {
			return nil, fmt.Errorf("printing: invalid portal page ranges")
		}
		pageRange := PageRange{First: first + 1, Last: first + 1}
		if hasEnd && end == "-1" {
			pageRange.Last = pageRangeThroughEnd
		} else if hasEnd {
			last, err := strconv.Atoi(end)
			if err != nil || last < first || last >= maxPrintPages {
				return nil, fmt.Errorf("printing: invalid portal page ranges")
			}
			pageRange.Last = last + 1
		}
		ranges = append(ranges, pageRange)
	}
	// GTK returns ranges in entry order. Render the selected pages in document order.
	slices.SortFunc(ranges, func(a, b PageRange) int { return cmp.Compare(a.First, b.First) })
	previous := 0
	for _, pageRange := range ranges {
		if previous == pageRangeThroughEnd || pageRange.First <= previous {
			return nil, fmt.Errorf("printing: invalid or overlapping portal page ranges")
		}
		previous = pageRange.Last
	}
	return ranges, nil
}
