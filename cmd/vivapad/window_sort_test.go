package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
)

func TestWindowNaturalSortMatchesOriginalNumericAndASCIICaseRules(t *testing.T) {
	for _, test := range []struct {
		first, second string
		want          int
	}{
		{"file2", "file10", -1},
		{"file02", "file2", -1},
		{"FILE2", "file2", 0},
		{"a9", "a10", -1},
		{"z", "A", 1},
	} {
		if got := windowNaturalCompare(test.first, test.second); got != test.want {
			t.Fatalf("compare(%q,%q) = %d, want %d", test.first, test.second, got, test.want)
		}
	}
}

func BenchmarkWindowKeyAfterEdit(b *testing.B) {
	state := newModel()
	doc := state.space.Current()
	doc.Editor = text.NewEditor(strings.Repeat("\u6c34\U0001f680\n", 1<<18))
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		start := text.Range{Start: 0, End: 1}
		doc.Editor.ReplaceRange(start, "\u706b")
		doc.Editor.ReplaceRange(start, "\u6c34")
		b.StartTimer()
		key, err := state.windowKey(doc, false)
		if err != nil || key.length != 2<<20 {
			b.Fatalf("window key length = %d, %v", key.length, err)
		}
	}
}

func TestWindowSortByAllOriginalColumnsPreservesPinsAndActiveDocument(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "file10.go"), filepath.Join(root, "a", "file2.txt"), filepath.Join(root, "z", "file02.py")}
	contents := []string{"pinned", "ab", "hello"}
	for index, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents[index]), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(int64(1000+index*100), 0)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles(paths[:1])
	docs := []*document.Document{state.space.Current()}
	for _, path := range paths[1:] {
		if err := state.space.Open(path); err != nil {
			t.Fatal(err)
		}
		docs = append(docs, state.space.Current())
	}
	state.pinned[docs[0]] = true
	state.space.Select(state.documentIndex(docs[1]))
	docs[1].Editor.Insert("!")
	state.toggleBookmark()
	for _, test := range []struct {
		command     string
		second, end *document.Document
	}{
		{"Name A to Z", docs[2], docs[1]},
		{"Name Z to A", docs[1], docs[2]},
		{"Path A to Z", docs[1], docs[2]},
		{"Path Z to A", docs[2], docs[1]},
		{"Type A to Z", docs[1], docs[2]},
		{"Type Z to A", docs[2], docs[1]},
		{"Content Length Ascending", docs[1], docs[2]},
		{"Content Length Descending", docs[2], docs[1]},
		{"Modified Time Ascending", docs[1], docs[2]},
		{"Modified Time Descending", docs[2], docs[1]},
	} {
		state.command(test.command)
		if !reflect.DeepEqual(state.space.Documents, []*document.Document{docs[0], test.second, test.end}) || state.space.Current() != docs[1] {
			t.Fatalf("%s order = %+v active=%p", test.command, state.space.Documents, state.space.Current())
		}
		if docs[1].Editor.String() != "!ab" || !docs[1].Dirty() || !docs[1].Editor.CanUndo() || len(state.bookmarkLines(docs[1])) != 1 {
			t.Fatalf("%s changed dirty document", test.command)
		}
	}
	before := append([]*document.Document(nil), state.space.Documents...)
	if err := os.Remove(paths[2]); err != nil {
		t.Fatal(err)
	}
	state.command("Modified Time Ascending")
	if !reflect.DeepEqual(state.space.Documents, before) || !strings.Contains(state.notice, "file02.py") {
		t.Fatalf("failed metadata sort changed order: %+v notice=%q", state.space.Documents, state.notice)
	}
}

func TestWindowSortOnlyReordersTheActiveSplitView(t *testing.T) {
	state, _ := startDriver(t)
	leftZ := state.space.Current()
	leftZ.Name = "z.txt"
	rightB := state.space.NewDocument()
	rightB.Name = "b.txt"
	leftA := state.space.NewDocument()
	leftA.Name = "a.txt"
	rightA := state.space.NewDocument()
	rightA.Name = "a-right.txt"
	state.paneOwner[leftZ], state.paneOwner[leftA] = 0, 0
	state.paneOwner[rightB], state.paneOwner[rightA] = 1, 1
	state.paneDocs = [2]*document.Document{leftZ, rightB}
	state.splitEnabled, state.activePane = true, 0
	state.space.Select(state.documentIndex(leftZ))
	rightBefore := append([]*document.Document(nil), state.paneTabs(1)...)
	state.command("Name A to Z")
	if got := state.paneTabs(0); !reflect.DeepEqual(got, []*document.Document{leftA, leftZ}) || !reflect.DeepEqual(state.paneTabs(1), rightBefore) || state.space.Current() != leftZ {
		t.Fatalf("split sort = left:%+v right:%+v active:%p", state.paneTabs(0), state.paneTabs(1), state.space.Current())
	}
}
