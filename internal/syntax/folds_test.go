package syntax

import (
	"reflect"
	"strings"
	"testing"
)

func TestFoldRegionsIgnoreBracesInsideStringsAndComments(t *testing.T) {
	value := "func main() {\n" +
		"  if true {\n" +
		"    println(\"}\")\n" +
		"  } // { not an opener\n" +
		"}\n" +
		"tail"
	want := []FoldRegion{{Header: 0, LastLine: 4, Level: 1}, {Header: 1, LastLine: 3, Level: 2}}
	if got := FoldRegions(".go", value); !reflect.DeepEqual(got, want) {
		t.Fatalf("Go fold regions = %+v, want %+v", got, want)
	}
	if got := FoldRegions(".go", "func main() {}\ntail"); len(got) != 0 {
		t.Fatalf("single-line braces folded: %+v", got)
	}
}

func TestFoldRegionsHandleNestedJSONAndEdits(t *testing.T) {
	value := "{\n  \"items\": [\n    {\"value\": 1}\n  ]\n}\n"
	want := []FoldRegion{{Header: 0, LastLine: 4, Level: 1}, {Header: 1, LastLine: 3, Level: 2}}
	if got := FoldRegions(".json", value); !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON fold regions = %+v, want %+v", got, want)
	}
	if got := FoldRegions(".json", "{\"items\": []}\n"); len(got) != 0 {
		t.Fatalf("fold region survived a flattened edit: %+v", got)
	}
}

func TestFoldRegionsFollowPythonIndentationAndSkipBlankComments(t *testing.T) {
	value := "def outer():\n    if flag:\n        work()\n\n    # note\n    return 1\nafter()"
	want := []FoldRegion{{Header: 0, LastLine: 5, Level: 1}, {Header: 1, LastLine: 4, Level: 2}}
	if got := FoldRegions(".py", value); !reflect.DeepEqual(got, want) {
		t.Fatalf("Python fold regions = %+v, want %+v", got, want)
	}
	if got := FoldRegions(".html", "<div>\n  body\n</div>"); len(got) != 0 {
		t.Fatalf("unsupported grammar advertised folds: %+v", got)
	}
}

func TestIntelHEXFoldRegionsFollowExtendedAddresses(t *testing.T) {
	value := strings.Join([]string{
		"preamble",
		":0100000001FE",
		":020000020100FB",
		":0100000001FE",
		"",
		"non-record between data lines",
		":0100010002FC",
		":0400000500000000F7",
		":0100000001FE",
		":020000040002F8",
		":02000004",
		":0100000001FE",
		":020000040003F7",
		":0100000001FE",
		"", // The terminal newline does not add a folded child.
	}, "\n")
	want := []FoldRegion{{Header: 2, LastLine: 6, Level: 1}, {Header: 12, LastLine: 13, Level: 1}}
	if got := FoldRegions(".HEX", value); !reflect.DeepEqual(got, want) {
		t.Fatalf("Intel HEX fold regions = %+v, want %+v", got, want)
	}
	for _, mode := range []string{"", ".srec", ".tek"} {
		if got := FoldRegions(mode, value); len(got) > 0 {
			t.Fatalf("mode %q unexpectedly folded Intel HEX: %+v", mode, got)
		}
	}
}

func TestIntelHEXFoldRegionsRequireCompleteAddressRecords(t *testing.T) {
	for _, test := range []struct {
		name   string
		header string
		fold   bool
	}{
		{"missing checksum padded with spaces", ":020000040002  ", false},
		{"incomplete checksum", ":020000040002F ", false},
		{"nonhex address", ":02oops040002F8", false},
		{"extra segment address byte", ":020000020100FB00", false},
		{"invalid suffix after linear address", ":020000040002F8oops", false},
		{"well formed checksum mismatch", ":02000004000200", true},
		{"complete segment address", "\t:020000020100fb", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			text := test.header + "\n:0100000001FE\n:00000001FF\n"
			var want []FoldRegion
			if test.fold {
				want = []FoldRegion{{Header: 0, LastLine: 1, Level: 1}}
			}
			if got := FoldRegions(".hex", text); !reflect.DeepEqual(got, want) {
				t.Fatalf("Intel HEX fold regions = %+v, want %+v", got, want)
			}
		})
	}
}
