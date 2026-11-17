package document

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func benchmarkGB18030Baseline(value string) ([]byte, error) {
	input := []byte(value)
	data, err := simplifiedchinese.GB18030.NewEncoder().Bytes(input)
	if err != nil {
		return nil, err
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
	if err != nil || !bytes.Equal(input, decoded) {
		return nil, fmt.Errorf("Unicode loss")
	}
	return data, nil
}

func BenchmarkGB18030Compatibility(b *testing.B) {
	for _, sample := range []struct {
		name, unit string
		baseline   bool
	}{
		{"ascii", "abc", true},
		{"cjk", "\u4e2d\u6587", true},
		{"pua", "\ue000\ue5e5", false},
		{"alternating", "a\ue000\u4e2d\uf014", false},
		{"supplementary", "\U0001f600", true},
	} {
		for _, size := range []int{256, 4096} {
			value := strings.Repeat(sample.unit, size)
			for _, mode := range []string{"current", "baseline"} {
				if mode == "baseline" && !sample.baseline {
					continue
				}
				b.Run(fmt.Sprintf("%s/%d/%s", sample.name, size, mode), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(value)))
					for b.Loop() {
						var err error
						if mode == "baseline" {
							_, err = benchmarkGB18030Baseline(value)
						} else {
							_, err = encodeText(value, GB18030)
						}
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}
