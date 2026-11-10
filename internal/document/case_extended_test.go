package document

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestProperCaseForceBlendAndApostrophes(t *testing.T) {
	doc := New("draft")
	before := "pre jOHN'S O’NEILL 42RD post"
	doc.Editor.Insert(before)
	doc.Editor.ClearHistory()
	first := doc.Editor.RuneIndex(len("pre "))
	last := doc.Editor.RuneIndex(len("pre jOHN'S O’NEILL 42RD"))
	doc.Editor.SetSelection(text.Range{Start: first, End: last})
	if !doc.ConvertCase(CaseProperForce) || doc.Editor.String() != "pre John's O’neill 42rd post" {
		t.Fatalf("proper force = %q", doc.Editor.String())
	}
	if !doc.Editor.Undo() || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("proper force Undo = %q", doc.Editor.String())
	}
	doc.Editor.SetSelection(text.Range{Start: first, End: last})
	if !doc.ConvertCase(CaseProperBlend) || doc.Editor.String() != "pre JOHN'S O’NEILL 42RD post" {
		t.Fatalf("proper blend = %q", doc.Editor.String())
	}
	unicode := New("unicode")
	unicode.Editor.Insert("élÈVe")
	unicode.Editor.SetSelection(text.Range{Start: 0, End: 5})
	if !unicode.ConvertCase(CaseProperForce) || unicode.Editor.String() != "Élève" {
		t.Fatalf("Unicode proper = %q", unicode.Editor.String())
	}
}

func TestSentenceCaseForceBlendPunctuationBlankLinesAndStandaloneI(t *testing.T) {
	doc := New("draft")
	doc.Editor.Insert("hELLO. wORLD!nEXT? yES\nline\n\nnew i end")
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	if !doc.ConvertCase(CaseSentenceForce) || doc.Editor.String() != "Hello. World!next? Yes\nline\n\nNew I end" {
		t.Fatalf("sentence force = %q", doc.Editor.String())
	}
	blend := New("blend")
	blend.Editor.Insert("hELLO. wORLD!nEXT")
	blend.Editor.SetSelection(text.Range{Start: 0, End: blend.Editor.RuneCount()})
	if !blend.ConvertCase(CaseSentenceBlend) || blend.Editor.String() != "HELLO. WORLD!nEXT" {
		t.Fatalf("sentence blend = %q", blend.Editor.String())
	}
	noSpace := New("no-space")
	noSpace.Editor.Insert("hello.world")
	noSpace.Editor.SetSelection(text.Range{Start: 0, End: noSpace.Editor.RuneCount()})
	if !noSpace.ConvertCase(CaseSentenceForce) || noSpace.Editor.String() != "Hello.world" {
		t.Fatalf("adjacent period = %q", noSpace.Editor.String())
	}
}

func TestRandomCaseUsesIndependentChoicesAndNoSelectionIsNoOp(t *testing.T) {
	upper := true
	converted := transformCase([]rune("aBc 42"), CaseRandom, func() bool {
		choice := upper
		upper = !upper
		return choice
	})
	if converted != "AbC 42" {
		t.Fatalf("random case decisions = %q", converted)
	}
	doc := New("draft")
	doc.Editor.Insert("cat")
	doc.Editor.ClearHistory()
	if doc.ConvertCase(CaseRandom) || doc.Editor.String() != "cat" || doc.Editor.CanUndo() {
		t.Fatal("unselected random case edited text")
	}
}

func TestProperCasePreservesLegacyEncodingOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "western.txt")
	if err := os.WriteFile(path, []byte{'c', 'A', 'F', 0xc9, ' ', 'j', 'O', 'H', 'N'}, 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := OpenWithEncoding(path, Windows1252)
	if err != nil {
		t.Fatal(err)
	}
	doc.Editor.SetSelection(text.Range{Start: 0, End: doc.Editor.RuneCount()})
	if !doc.ConvertCase(CaseProperForce) || doc.Editor.String() != "Café John" {
		t.Fatalf("legacy proper = %q", doc.Editor.String())
	}
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string([]byte{'C', 'a', 'f', 0xe9, ' ', 'J', 'o', 'h', 'n'}) {
		t.Fatalf("legacy proper save = %x, %v", data, err)
	}
}
