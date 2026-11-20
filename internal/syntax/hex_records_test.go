package syntax

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestHexRecordLexersDistinguishFieldsAndCheckIntegrity(t *testing.T) {
	for _, tc := range []struct {
		name, valid, badCount, badSum string
		fields                        []string
		countIndex, sumIndex          int
	}{
		{"Intel HEX", ":020000000102FB", ":030000000102FB", ":020000000102FA", []string{":", "02", "0000", "00", "01", "02", "FB"}, 1, 6},
		{"S-Record", "S10512340102B1", "S10612340102B1", "S10512340102B0", []string{"S", "1", "05", "1234", "01", "02", "B1"}, 2, 6},
		{"Tektronix extended HEX", "%126148000000000102", "%136148000000000102", "%126158000000000102", []string{"%", "12", "6", "14", "800000000", "01", "02"}, 1, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			language, ok := LanguageByLabel(tc.name)
			if !ok {
				t.Fatal("language is absent from the catalog")
			}
			lexer := lexers.Get(language.Lexer)
			if lexer == nil {
				t.Fatal("record lexer is not registered")
			}
			tokens := func(source string) []chroma.Token {
				t.Helper()
				iter, err := lexer.Tokenise(nil, source)
				if err != nil {
					t.Fatal(err)
				}
				got := iter.Tokens()
				var text strings.Builder
				for _, token := range got {
					text.WriteString(token.Value)
				}
				if text.String() != source {
					t.Fatalf("lexer dropped or changed bytes in %q", source)
				}
				return got
			}
			got := tokens(tc.valid)
			var fields []string
			for _, token := range got {
				fields = append(fields, token.Value)
				if token.Type == chroma.Error {
					t.Fatalf("valid record marked an error: %v", got)
				}
			}
			if !slices.Equal(fields, tc.fields) || got[tc.countIndex].Type == got[tc.sumIndex].Type {
				t.Fatalf("record fields not distinguished: %v", got)
			}
			if got := tokens(tc.badCount); len(got) <= tc.countIndex || got[tc.countIndex].Type != chroma.Error {
				t.Fatalf("incorrect length not marked: %v", got)
			}
			if got := tokens(tc.badSum); len(got) <= tc.sumIndex || got[tc.sumIndex].Type != chroma.Error {
				t.Fatalf("incorrect checksum not marked: %v", got)
			}
			for end := 0; end < len(tc.valid); end++ {
				tokens(tc.valid[:end])
			}
			tokens(tc.valid + "XYZ\r\n" + tc.valid + "\n")
			var highlighter Highlighter
			goodRuns := highlighter.Highlight(language.Mode, tc.valid, nil)
			badRuns := highlighter.Highlight(language.Mode, tc.badSum, nil)
			if len(goodRuns) != len(tc.fields) || len(badRuns) != len(tc.fields) ||
				goodRuns[tc.sumIndex].Color == badRuns[tc.sumIndex].Color ||
				goodRuns[tc.sumIndex].Start != badRuns[tc.sumIndex].Start ||
				goodRuns[tc.sumIndex].End != badRuns[tc.sumIndex].End {
				t.Fatalf("checksum styles do not reach the same editor field: %v / %v", goodRuns, badRuns)
			}
			if runs := highlighter.Highlight("", tc.valid, nil); len(runs) != 0 {
				t.Fatalf("Normal Text highlighted the record: %v", runs)
			}
		})
	}
}

func TestHexRecordLanguageMappings(t *testing.T) {
	for extension, nppName := range map[string]string{".hex": "ihex", ".mot": "srec", ".srec": "srec", ".tek": "tehex"} {
		byExtension, ok := LanguageByMode(extension)
		byNPP, known := LanguageByNPP(nppName)
		if !ok || !known || byExtension != byNPP || byExtension.LineComment != "" {
			t.Errorf("%s does not map to %s: %+v / %+v", extension, nppName, byExtension, byNPP)
		}
	}
}

func TestHexRecordHighlightDistinguishesHalfByteData(t *testing.T) {
	for _, tc := range []struct {
		name, mode, record string
		wantError          bool
	}{
		{"Intel incomplete byte", ".hex", ":020000000AB", true},
		{"Motorola incomplete byte", ".srec", "S10512340AB", true},
		{"Tektronix odd digit count", ".tek", "%116258000000000AB", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var highlighter Highlighter
			runs := highlighter.Highlight(tc.mode, tc.record, nil)
			invalid := highlighter.Highlight(tc.mode, tc.record+"?", nil)
			if len(runs) < 2 || len(invalid) == 0 {
				t.Fatalf("record did not produce data runs: %v / %v", runs, invalid)
			}
			badColor := invalid[len(invalid)-1].Color
			last := runs[len(runs)-1]
			complete := runs[len(runs)-2]
			if complete.Start != len(tc.record)-3 || complete.End != len(tc.record)-1 || complete.Color == badColor {
				t.Fatalf("complete byte lost its data style: %+v", complete)
			}
			if last.Start != len(tc.record)-1 || last.End != len(tc.record) || (last.Color == badColor) != tc.wantError {
				t.Fatalf("last digit highlight = %+v, want error %t", last, tc.wantError)
			}
		})
	}
}

func TestIntelHexAddressPayloadUsesAddressStyle(t *testing.T) {
	address, _ := tokenColor(chroma.NameFunction)
	checksum, _ := tokenColor(chroma.Comment)
	var highlighter Highlighter
	for _, record := range []string{
		":020000020100FB",
		":020000040002F8",
		":0400000300010002F6",
		":0400000500010002F4",
	} {
		end := len(record) - 2
		var addressRuns, checksumRuns int
		for _, run := range highlighter.Highlight(".hex", record, nil) {
			if run.Start >= 9 && run.End <= end {
				addressRuns++
				if run.Start != 9 || run.End != end || run.Color != address {
					t.Errorf("%s address payload was not one address field: %+v", record, run)
				}
			}
			if run.Start == end && run.End == len(record) && run.Color == checksum {
				checksumRuns++
			}
		}
		if addressRuns != 1 || checksumRuns != 1 {
			t.Errorf("%s has %d address payload runs and %d valid checksum runs", record, addressRuns, checksumRuns)
		}
	}
	const data = ":020000000102FB"
	runs := highlighter.Highlight(".hex", data, nil)
	if len(runs) != 7 || runs[4].Start != 9 || runs[5].End != len(data)-2 ||
		runs[4].Color == address || runs[5].Color == address || runs[4].Color == runs[5].Color {
		t.Fatalf("ordinary data lost its alternating byte colors: %+v", runs)
	}
}

func TestHexRecordHighlightKeepsMalformedUnicodeIntact(t *testing.T) {
	for _, tc := range []struct{ mode, valid string }{
		{".hex", ":020000000102FB"},
		{".srec", "S10512340102B1"},
		{".tek", "%126148000000000102"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			var highlighter Highlighter
			invalid := highlighter.Highlight(tc.mode, tc.valid+"?", nil)
			if len(invalid) == 0 || invalid[len(invalid)-1].Start != len(tc.valid) {
				t.Fatalf("malformed ASCII suffix did not get its own highlight: %v", invalid)
			}
			invalidColor := invalid[len(invalid)-1].Color
			for at := 1; at <= len(tc.valid); at++ {
				for _, malformed := range []string{"é", "界", "🙂"} {
					source := tc.valid[:at] + malformed + tc.valid[at:]
					var covered bool
					for _, run := range highlighter.Highlight(tc.mode, source, nil) {
						if !utf8.ValidString(source[run.Start:run.End]) {
							t.Fatalf("%q splits a character at [%d:%d]", source, run.Start, run.End)
						}
						if run.Start <= at && run.End >= at+len(malformed) {
							covered = true
							if run.Color != invalidColor {
								t.Fatalf("%q does not mark %q as invalid", source, malformed)
							}
						}
					}
					if !covered {
						t.Fatalf("%q did not highlight the whole invalid character %q", source, malformed)
					}
				}
			}
		})
	}
}

func TestHexRecordCountsFollowRecordType(t *testing.T) {
	for _, tc := range []struct {
		name, mode, record string
		countIndex         int
		valid              bool
	}{
		{"Intel EOF without data", "ihex", ":00000001FF", 1, true},
		{"Intel EOF with data", "ihex", ":0100000101FD", 1, false},
		{"Intel extended address", "ihex", ":020000020100FB", 1, true},
		{"Intel short extended address", "ihex", ":0100000201FC", 1, false},
		{"Intel start address", "ihex", ":0400000300010002F6", 1, true},
		{"Intel short start address", "ihex", ":020000030102F8", 1, false},
		{"S-Record count without data", "srec", "S5030001FB", 2, true},
		{"S-Record count with data", "srec", "S5040001AA50", 2, false},
		{"S-Record start without data", "srec", "S9031234B6", 2, true},
		{"S-Record start with data", "srec", "S70600001234AA09", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iterator, err := lexers.Get(tc.mode).Tokenise(nil, tc.record)
			if err != nil {
				t.Fatal(err)
			}
			got := iterator.Tokens()
			want := chroma.Error
			if tc.valid {
				want = chroma.LiteralNumber
			}
			if len(got) <= tc.countIndex || got[tc.countIndex].Type != want || got[len(got)-1].Type != chroma.Comment {
				t.Fatalf("count should be %v and checksum should remain valid: %v", want, got)
			}
		})
	}
}
