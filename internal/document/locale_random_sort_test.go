package document

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"golang.org/x/text/language"
)

func TestLocaleSortUsesCaseInsensitiveNumericKeysAndStableTies(t *testing.T) {
	doc := New("locale.txt")
	before := "item10\nItem2\nitem1\nITEM2\n"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	if !doc.SortLinesLocale(language.English, true) || doc.Editor.String() != "item1\nItem2\nITEM2\nitem10\n" {
		t.Fatalf("locale ascending = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatal("locale sort did not undo in one step")
	}
	if !doc.SortLinesLocale(language.English, false) || doc.Editor.String() != "item10\nItem2\nITEM2\nitem1\n" {
		t.Fatalf("locale descending = %q", doc.Editor.String())
	}
}

func TestLocaleSortHonorsLanguageAndSelectedLineBoundary(t *testing.T) {
	for _, test := range []struct {
		locale  language.Tag
		want    string
		changed bool
	}{
		{language.Swedish, "z\nä\noutside", true},
		{language.English, "ä\nz\noutside", true},
	} {
		doc := New("locale.txt")
		doc.Editor.Insert("ä\nz\noutside")
		doc.Editor.ClearHistory()
		doc.Editor.SetSelection(text.Range{Start: 0, End: len([]rune("ä\nz\n"))})
		if changed := doc.SortLinesLocale(test.locale, true); changed != test.changed || doc.Editor.String() != test.want {
			t.Fatalf("%s selected locale sort = %q", test.locale, doc.Editor.String())
		}
	}
}

func TestLocaleSortPreservesCRLFDiskFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locale.txt")
	if err := os.WriteFile(path, []byte("item10\r\nitem2\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.SortLinesLocale(language.English, true) {
		t.Fatal("locale sort did not edit")
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "item2\r\nitem10\r\n" {
		t.Fatalf("locale saved CRLF = %q, %v", data, err)
	}
}

func TestRandomizeLinesIsSeedReproducibleAndKeepsAllLines(t *testing.T) {
	before := "a\nb\nc\nd\ne\nf\n"
	shuffle := func() *Document {
		doc := New("random.txt")
		doc.Editor.Insert(before)
		doc.Editor.ClearHistory()
		if !doc.RandomizeLines(rand.New(rand.NewPCG(7, 11))) {
			t.Fatal("seeded shuffle did not change the document")
		}
		return doc
	}
	first, second := shuffle(), shuffle()
	if first.Editor.String() != second.Editor.String() || first.Editor.String() == before {
		t.Fatalf("seed reproducibility = %q and %q", first.Editor.String(), second.Editor.String())
	}
	got := strings.Split(first.Editor.String(), "\n")
	want := strings.Split(before, "\n")
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shuffle lost/duplicated lines: %v vs %v", got, want)
	}
	if !first.Editor.Undo() || first.Editor.String() != before || first.Editor.CanUndo() {
		t.Fatal("shuffle did not undo in one step")
	}
}

func TestRandomizeLinesKeepsUnselectedAndSingleLineUnchanged(t *testing.T) {
	doc := New("random.txt")
	doc.Editor.Insert("a\nb\nc\nd\noutside")
	doc.Editor.ClearHistory()
	doc.Editor.SetSelection(text.Range{Start: 0, End: len([]rune("a\nb\nc\nd\n"))})
	if !doc.RandomizeLines(rand.New(rand.NewPCG(3, 7))) || !strings.HasSuffix(doc.Editor.String(), "\noutside") {
		t.Fatalf("selected shuffle = %q", doc.Editor.String())
	}
	doc = New("single.txt")
	doc.Editor.Insert("only")
	doc.Editor.ClearHistory()
	if doc.RandomizeLines(rand.New(rand.NewPCG(3, 7))) || doc.Editor.CanUndo() || doc.RandomizeLines(nil) {
		t.Fatal("single-line or missing random source should be a no-op")
	}
}
