package completion

import (
	"slices"
	"strings"
	"testing"
)

func TestFunctionCatalogCompletesQualifiedName(t *testing.T) {
	const source = `<NotepadPlus><AutoComplete><Environment ignoreCase="yes" additionalWordChar="."/><KeyWord name="api.Close" func="yes"/><KeyWord name="api.Copy" func="yes"/><KeyWord name="api.Constant"/><KeyWord name="API.CLOSE" func="yes"/></AutoComplete></NotepadPlus>`
	catalog, err := ParseFunctionCatalog(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	got := catalog.Complete("x = API.Cotail", len("x = API.Co"))
	if got.Start != 4 || got.End != 14 || !slices.Equal(got.Candidates, []string{"api.Copy"}) {
		t.Fatalf("qualified completion = %+v", got)
	}
	got = catalog.Complete("aPI.C", len("aPI.C"))
	if !slices.Equal(got.Candidates, []string{"api.Close", "api.Copy"}) {
		t.Fatalf("case or duplicate handling = %+v", got)
	}
}

func TestImportedFunctionCatalogUsesNotepadPlusPlusCaseDefault(t *testing.T) {
	const entry = `<KeyWord name="append" func="yes"><Overload><Param name="slice"/></Overload></KeyWord>`
	for _, tc := range []struct {
		name, environment string
		want              bool
	}{
		{"missing environment", "", true},
		{"missing attribute", `<Environment startFunc="("/>`, true},
		{"explicit yes", `<Environment ignoreCase="yes"/>`, true},
		{"explicit no", `<Environment ignoreCase="no"/>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `<NotepadPlus><AutoComplete>` + tc.environment + entry + `</AutoComplete></NotepadPlus>`
			catalog, err := ParseFunctionCatalog(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			candidates := catalog.Complete("APP", 3).Candidates
			hint := catalog.CallAt("APPEND(", 7, CallSyntax{})
			if slices.Equal(candidates, []string{"append"}) != tc.want || (hint.Name == "append") != tc.want {
				t.Fatalf("candidate=%q, hint=%q, matching expected=%t", candidates, hint.Name, tc.want)
			}
		})
	}
}

func TestImportedAutoItCatalogAcceptsAtWordCharacter(t *testing.T) {
	const source = `<?xml version="1.0" encoding="UTF-8" ?>
<NotepadPlus><AutoComplete>
<Environment ignoreCase="yes" startFunc="(" stopFunc=")" paramSeparator="," terminal=";" additionalWordChar="@"/>
<KeyWord name="Abs" func="yes"><Overload><Param name="expression"/></Overload></KeyWord>
<KeyWord name="@AppDataDir"/>
</AutoComplete></NotepadPlus>`
	catalog, err := ParseFunctionCatalog(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if got := catalog.Complete("ab", 2); !slices.Equal(got.Candidates, []string{"Abs"}) {
		t.Fatalf("AutoIt completion = %+v", got)
	}
	if got := catalog.Complete("@App", 4); len(got.Candidates) != 0 {
		t.Fatalf("non-callable macro leaked into function completion: %+v", got)
	}
}

func TestFunctionCatalogRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{
		`<AutoComplete/>`, `<NotepadPlus/>`, `<NotepadPlus><AutoComplete/></NotepadPlus><other/>`,
		`<NotepadPlus><AutoComplete><KeyWord name="x(" func="yes"/></AutoComplete></NotepadPlus>`,
		strings.Repeat(" ", (1<<20)+1),
	} {
		if _, err := ParseFunctionCatalog(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted invalid catalog of %d bytes", len(input))
		}
	}
}
