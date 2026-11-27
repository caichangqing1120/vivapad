package syntax

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestUDLStorageAndMultilineHighlight(t *testing.T) {
	path := filepath.Join(t.TempDir(), "udl.json")
	definition := UDL{Name: "Demo", Extensions: []string{".DEMO"}, Keywords: []string{"let"}, LineComment: "//", BlockStart: "(*", BlockEnd: "*)", Quotes: []string{"\""}}
	if err := SaveUDLs(path, []UDL{definition}); err != nil {
		t.Fatal(err)
	}
	definitions, err := LoadUDLs(path)
	if err != nil || len(definitions) != 1 || definitions[0].Extensions[0] != "demo" {
		t.Fatalf("load = %+v, %v", definitions, err)
	}
	value := "let \"(*\" (* comment\nlet *) let // let"
	highlight := definitions[0].Highlighter(value)
	runs := highlight("", 1, "let *) let // let", nil)
	comment, _ := tokenColor(chroma.Comment)
	keyword, _ := tokenColor(chroma.Keyword)
	if len(runs) != 3 || runs[0].Color != comment || runs[0].End != 6 || runs[1].Color != keyword || runs[1].Start != 7 || runs[2].Color != comment {
		t.Fatalf("line 2 runs = %+v", runs)
	}
	if err := SaveUDLs(path, []UDL{definition, {Name: "Other", Extensions: []string{"demo"}}}); err == nil {
		t.Fatal("ambiguous extensions accepted")
	}
	if loaded, err := LoadUDLs(path); err != nil || len(loaded) != 1 {
		t.Fatalf("invalid save replaced prior data: %+v, %v", loaded, err)
	}
}

func TestUDLRejectsDamagedConfigurationWithoutPartialRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "udl.json")
	for _, value := range []string{`[{"name":"Valid"},{"name":"Bad","block_start":"/*"}]`, `[{"name":"Valid"}] trailing`, `[{"name":`} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if definitions, err := LoadUDLs(path); err == nil || len(definitions) != 0 {
			t.Fatalf("loaded damaged configuration %q: %+v, %v", value, definitions, err)
		}
	}
}
