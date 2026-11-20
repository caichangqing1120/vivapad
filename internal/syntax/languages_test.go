package syntax

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/lexers"
)

func TestLanguageCatalogHasOriginalIdentityAndRealLexers(t *testing.T) {
	specs := Languages()
	// IDs from the Notepad++ v8.9.8.1 language menu, independent of local plans.
	data, err := os.ReadFile("testdata/npp-language-ids.txt")
	if err != nil {
		t.Fatal(err)
	}
	sourceIDs := strings.Fields(string(data))
	if len(specs) < 40 {
		t.Fatalf("registered original languages = %d, want at least 40", len(specs))
	}
	first := []string{"Normal Text", "Go", "JavaScript", "TypeScript", "Python", "JSON", "HTML", "CSS", "Markdown", "SQL"}
	for index, label := range first {
		if specs[index].Label != label {
			t.Fatalf("language %d = %q, want %q", index, specs[index].Label, label)
		}
	}
	labels, modes, ids, npp := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, spec := range specs {
		if spec.Label == "" || spec.SourceID == "" && spec.Label != "Markdown" || labels[spec.Label] || spec.SourceID != "" && ids[spec.SourceID] || modes[spec.Mode] || npp[spec.NPPName] {
			t.Fatalf("duplicate or incomplete language %+v", spec)
		}
		labels[spec.Label], modes[spec.Mode], npp[spec.NPPName] = true, true, true
		if spec.SourceID != "" {
			ids[spec.SourceID] = true
			if !slices.Contains(sourceIDs, spec.SourceID) {
				t.Fatalf("language %q has no pinned source ID %s", spec.Label, spec.SourceID)
			}
		}
		if got, ok := LanguageByLabel(spec.Label); !ok || got.Mode != spec.Mode {
			t.Fatalf("label lookup %q = %+v, %v", spec.Label, got, ok)
		}
		if got, ok := LanguageByMode(spec.Mode); !ok || got.SourceID != spec.SourceID {
			t.Fatalf("mode lookup %q = %+v, %v", spec.Mode, got, ok)
		}
		if got, ok := LanguageByNPP(spec.NPPName); !ok || got.Mode != spec.Mode {
			t.Fatalf("NPP lookup %q = %+v, %v", spec.NPPName, got, ok)
		}
		if spec.Lexer != "" && lexers.Get(spec.Lexer) == nil {
			t.Fatalf("%q has no Chroma lexer %q", spec.Label, spec.Lexer)
		}
	}
	for _, missing := range []string{"BaanC", "Gui4Cli", "User-Defined"} {
		if _, ok := LanguageByLabel(missing); ok {
			t.Fatalf("unsupported lexer %q shown as functional", missing)
		}
	}
}

func TestExpandedLanguageModesHighlightWithSelectedLexer(t *testing.T) {
	var highlighter Highlighter
	for _, test := range []struct{ label, line string }{
		{"Rust", `fn main() { let value = 42; }`},
		{"Ruby", `def greet(name)`},
		{"C++", `int main() { return 0; }`},
		{"YAML", `enabled: true`},
		{"Shell", `echo "hello"`},
	} {
		spec, ok := LanguageByLabel(test.label)
		if !ok || len(highlighter.Highlight(spec.Mode, test.line, nil)) == 0 {
			t.Fatalf("%q has no highlighted tokens in %q", test.label, test.line)
		}
	}
}

func TestAdditionalOriginalLanguagesHighlight(t *testing.T) {
	var highlighter Highlighter
	for _, tc := range []struct{ name, id, line string }{
		{"ada", "IDM_LANG_ADA", "procedure Demo is"},
		{"autoit", "IDM_LANG_AU3", "Local $value = 42"},
		{"batch", "IDM_LANG_BATCH", "REM an example"},
		{"caml", "IDM_LANG_CAML", "let value = 42"},
		{"cobol", "IDM_LANG_COBOL", "       IDENTIFICATION DIVISION."},
		{"d", "IDM_LANG_D", "int value = 42;"},
		{"fortran77", "IDM_LANG_FORTRAN_77", "      INTEGER I"},
		{"forth", "IDM_LANG_FORTH", "\\ an example"},
		{"nsis", "IDM_LANG_NSIS", "Section \"Demo\""},
		{"props", "IDM_LANG_PROPS", "# an example"},
		{"registry", "IDM_LANG_REGISTRY", "[HKEY_CURRENT_USER\\Software\\Example]"},
		{"sas", "IDM_LANG_SAS", "/* an example */"},
		{"smalltalk", "IDM_LANG_SMALLTALK", "\"an example\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lang, ok := LanguageByNPP(tc.name)
			if !ok || lang.SourceID != tc.id {
				t.Fatalf("session language %s = %+v", tc.name, lang)
			}
			if runs := highlighter.Highlight(lang.Mode, tc.line, nil); len(runs) == 0 {
				t.Fatalf("%s did not highlight %q", lang.Lexer, tc.line)
			}
		})
	}
}

func TestYAMLExtensionAliasUsesRegisteredLexer(t *testing.T) {
	if spec, ok := LanguageByMode(".yml"); !ok || spec.Label != "YAML" {
		t.Fatalf("YAML file extension = %+v, %v", spec, ok)
	}
	var highlighter Highlighter
	if len(highlighter.Highlight(".yml", "enabled: true", nil)) == 0 {
		t.Fatal("YAML alias has no highlighted tokens")
	}
}
