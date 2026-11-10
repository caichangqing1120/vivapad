//go:build !windows

package document

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func BenchmarkLocaleOrderingKeys(benchmark *testing.B) {
	for _, width := range []int{16, 1024} {
		values := make([]string, 256)
		for index := range values {
			values[index] = strings.Repeat("x", width) + fmt.Sprintf(" item%d", len(values)-index)
		}
		for _, materialized := range []bool{false, true} {
			name := "CompareString"
			if materialized {
				name = "OwnedKeys"
			}
			benchmark.Run(fmt.Sprintf("%s/%d", name, width), func(benchmark *testing.B) {
				benchmark.ReportAllocs()
				for range benchmark.N {
					ordered := make([]int, len(values))
					for index := range ordered {
						ordered[index] = index
					}
					if materialized {
						keys := newLocaleKeys(language.English)
						encoded := make([][]byte, len(values))
						for index, value := range values {
							key, err := keys.key(value)
							if err != nil {
								benchmark.Fatal(err)
							}
							encoded[index] = key
						}
						sort.SliceStable(ordered, func(first, second int) bool {
							return compareLocaleKeys(encoded[ordered[first]], encoded[ordered[second]]) < 0
						})
					} else {
						comparator := collate.New(language.English, collate.IgnoreCase, collate.Numeric)
						sort.SliceStable(ordered, func(first, second int) bool {
							return comparator.CompareString(values[ordered[first]], values[ordered[second]]) < 0
						})
					}
					if !strings.HasSuffix(values[ordered[0]], " item1") {
						benchmark.Fatal("Locale key benchmark order differs")
					}
				}
			})
		}
	}
}
