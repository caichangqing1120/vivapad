package syntax

import (
	"slices"
	"strings"
	"testing"

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
