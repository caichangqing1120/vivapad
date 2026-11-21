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
