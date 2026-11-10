package session

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNPPSessionXMLRoundTripWithViewsAndEscapedPaths(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "one&two.txt"), filepath.Join(root, "chinese.txt")
	want := NPPState{
		Main:                []NPPFile{{Path: first, Encoding: -1, Language: "go", StartPos: 2, EndPos: 5, Marks: []int{0, 3}, ReadOnly: true, Pinned: true}},
		Sub:                 []NPPFile{{Path: second, Encoding: 54936, StartPos: 0, EndPos: 3}},
		ActiveView:          1,
		FileBrowserIncluded: true,
		FolderSelected:      filepath.Join(root, "one&two", "src", "main.go"),
		FolderRoots: []NPPFolderRoot{
			{Path: filepath.Join(root, "one&two"), Expanded: []string{filepath.Join(root, "one&two"), filepath.Join(root, "one&two", "src")}},
			{Path: filepath.Join(root, "other")},
		},
	}
	path := filepath.Join(root, "session.xml")
	if err := SaveNPP(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `<NotepadPlus>`) || !strings.Contains(string(data), `one&amp;two.txt`) ||
		!strings.Contains(string(data), `activeView="1"`) || !strings.Contains(string(data), `userReadOnly="yes"`) || !strings.Contains(string(data), `tabPinned="yes"`) ||
		!strings.Contains(string(data), `latestSelectedItem="`) || !strings.Contains(string(data), `<root foldername="`) || !strings.Contains(string(data), `<expanded path="`) {
		t.Fatalf("exported NPP XML = %q, %v", data, err)
	}
	got, err := LoadNPP(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded NPP session = %+v, %v, want %+v", got, err, want)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("NPP session file mode = %+v, %v", info, err)
	}
}

func TestNPPColorIDsRoundTripAndRejectInvalidAttributes(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first.txt"), filepath.Join(root, "second.txt")
	path := filepath.Join(root, "session.xml")
	want := NPPState{Main: []NPPFile{{Path: first, Encoding: -1, Color: 5}, {Path: second, Encoding: -1}}, Sub: []NPPFile{{Path: first, Encoding: -1, Color: 5}}}
	if err := SaveNPP(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `tabColourId="4"`) || !strings.Contains(string(data), `tabColourId="-1"`) {
		t.Fatalf("missing original color IDs: %s, %v", data, err)
	}
	if got, err := LoadNPP(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("NPP color round trip = %+v, %v", got, err)
	}
	for _, value := range []string{"-2", "5", "unknown"} {
		bad := `<NotepadPlus><Session><mainView><File filename="` + first + `" tabColourId="` + value + `"/></mainView><subView/></Session></NotepadPlus>`
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadNPP(path); err == nil {
			t.Fatalf("accepted NPP tabColourId=%s", value)
		}
	}
}

func TestNPPSessionParsesOfficialOptionalAttributes(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "source.go")
	content := `<?xml version="1.0" encoding="UTF-8"?>
<NotepadPlus><Session activeView="0"><mainView activeIndex="0">
<File filename="` + file + `" lang="go" firstVisibleLine="12" startPos="4" endPos="7" xOffset="0" scrollWidth="1" selMode="0" backupFilePath="" userReadOnly="no" tabPinned="no"/>
</mainView><subView activeIndex="0"/></Session></NotepadPlus>`
	path := filepath.Join(root, "session.xml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadNPP(path)
	if err != nil || len(got.Main) != 1 || got.Main[0].Path != file || got.Main[0].Encoding != -1 || got.Main[0].StartPos != 4 || got.Main[0].EndPos != 7 {
		t.Fatalf("official-style session = %+v, %v", got, err)
	}
}

func TestNPPSessionRejectsUnsupportedOrUnsafeInput(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "safe.txt")
	path := filepath.Join(root, "session.xml")
	for name, content := range map[string]string{
		"malformed":               "<NotepadPlus>",
		"wrong root":              `<Other><Session/></Other>`,
		"relative":                `<NotepadPlus><Session><mainView><File filename="relative.txt"/></mainView></Session></NotepadPlus>`,
		"unknown codepage":        `<NotepadPlus><Session><mainView><File filename="` + file + `" encoding="9999"/></mainView></Session></NotepadPlus>`,
		"backup":                  `<NotepadPlus><Session><mainView><File filename="` + file + `" backupFilePath="backup.bin"/></mainView></Session></NotepadPlus>`,
		"unknown pinned":          `<NotepadPlus><Session><mainView><File filename="` + file + `" tabPinned="sometimes"/></mainView></Session></NotepadPlus>`,
		"unknown read-only":       `<NotepadPlus><Session><mainView><File filename="` + file + `" userReadOnly="sometimes"/></mainView></Session></NotepadPlus>`,
		"duplicate bookmarks":     `<NotepadPlus><Session><mainView><File filename="` + file + `"><Mark line="1"/><Mark line="1"/></File></mainView></Session></NotepadPlus>`,
		"negative bookmark":       `<NotepadPlus><Session><mainView><File filename="` + file + `"><Mark line="-1"/></File></mainView></Session></NotepadPlus>`,
		"missing bookmark line":   `<NotepadPlus><Session><mainView><File filename="` + file + `"><Mark/></File></mainView></Session></NotepadPlus>`,
		"folder missing name":     `<NotepadPlus><Session><FileBrowser><root/></FileBrowser></Session></NotepadPlus>`,
		"folder relative":         `<NotepadPlus><Session><FileBrowser><root foldername="relative"/></FileBrowser></Session></NotepadPlus>`,
		"folder escaped":          `<NotepadPlus><Session><FileBrowser><root foldername="` + root + `"><expanded path="` + filepath.Join(filepath.Dir(root), "outside") + `"/></root></FileBrowser></Session></NotepadPlus>`,
		"folder selected outside": `<NotepadPlus><Session><FileBrowser latestSelectedItem="` + filepath.Dir(root) + `"><root foldername="` + root + `"/></FileBrowser></Session></NotepadPlus>`,
		"folder unknown child":    `<NotepadPlus><Session><FileBrowser><extra/></FileBrowser></Session></NotepadPlus>`,
		"trailing document":       `<NotepadPlus><Session/></NotepadPlus><NotepadPlus/>`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadNPP(path); err == nil {
				t.Fatal("unsupported session was accepted")
			}
		})
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", 1<<20)+`<NotepadPlus/>`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadNPP(path); err == nil {
		t.Fatal("oversized NPP session was accepted")
	}
}

func TestNPPSessionRoundTripsRegisteredCodePages(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "registered.xml")
	want := NPPState{Main: []NPPFile{
		{Path: filepath.Join(root, "japanese.txt"), Encoding: 932, StartPos: 2, EndPos: 4},
		{Path: filepath.Join(root, "cyrillic.txt"), Encoding: 1251},
		{Path: filepath.Join(root, "arabic.txt"), Encoding: 720},
		{Path: filepath.Join(root, "greek.txt"), Encoding: 869},
	}}
	if err := SaveNPP(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadNPP(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("registered NPP code pages = %+v, %v", got, err)
	}
}

func TestNPPSessionFoldElementsRoundTripAndRejectMalformed(t *testing.T) {
	root := t.TempDir()
	file, path := filepath.Join(root, "fold.go"), filepath.Join(root, "session.xml")
	want := NPPState{Main: []NPPFile{{Path: file, Encoding: -1, Folds: []int{0, 3}}}}
	if err := SaveNPP(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `<Fold line="0"`) || !strings.Contains(string(data), `<Fold line="3"`) {
		t.Fatalf("NPP fold XML = %q, %v", data, err)
	}
	if got, err := LoadNPP(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("NPP fold round-trip = %+v, %v", got, err)
	}
	for _, content := range []string{
		`<NotepadPlus><Session><mainView><File filename="` + file + `"><Fold/></File></mainView></Session></NotepadPlus>`,
		`<NotepadPlus><Session><mainView><File filename="` + file + `"><Fold line="1"/><Fold line="1"/></File></mainView></Session></NotepadPlus>`,
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadNPP(path); err == nil {
			t.Fatalf("malformed NPP folds accepted: %q", content)
		}
	}
}

func TestNPPInvalidExportLeavesExistingFileUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.xml")
	if err := os.WriteFile(path, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveNPP(path, NPPState{ActiveView: 1}); err == nil {
		t.Fatal("sub-view without files was exported as active")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "untouched" {
		t.Fatalf("invalid export overwrote file: %q, %v", data, err)
	}
}
