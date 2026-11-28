package batchconvert

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"vivapad/internal/document"
)

func fixture(t *testing.T, root, name, value string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func checkFile(t *testing.T, path, want string) {
	t.Helper()
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Fatalf("%s: got % x (%v), want % x", path, got, err, want)
	}
}

func TestPlanThenApplyMixedEncodings(t *testing.T) {
	root := t.TempDir()
	target := document.UTF8
	inputs := []struct {
		value    string
		encoding document.Encoding
		eol      EOL
	}{
		{"中\nA\n", document.UTF8, LF},
		{"\xef\xbb\xbf中\r\nA\r\n", document.UTF8BOM, CRLF},
		{"\xff\xfe\x2d\x4e\r\x00A\x00\r\x00", document.UTF16LE, CR},
		{"\xfe\xff\x4e\x2d\x00\r\x00A\x00\r\x00\n", document.UTF16BE, Mixed},
		{"\xd6\xd0\nA\n", document.GB18030, LF},
	}
	for index, input := range inputs {
		fixture(t, root, string(rune('a'+index))+".txt", input.value)
	}
	plan, err := Plan(t.Context(), Options{Root: root, TargetEncoding: &target, TargetEOL: LF})
	if err != nil {
		t.Fatal(err)
	}
	entries := plan.Entries()
	if len(entries) != len(inputs) {
		t.Fatalf("preview: %+v", entries)
	}
	for index, entry := range entries {
		wantAction := Convert
		if index == 0 {
			wantAction = Unchanged
		}
		if entry.SourceEncoding != inputs[index].encoding || entry.SourceEOL != inputs[index].eol || entry.TargetEncoding != target || entry.TargetEOL != LF || entry.Action != wantAction {
			t.Fatalf("entry %d: %+v", index, entry)
		}
		checkFile(t, entry.Path, inputs[index].value)
	}
	entries[1].Path = entries[0].Path
	result := Apply(t.Context(), plan)
	if result.Converted != 4 || result.Unchanged != 1 || result.Skipped != 0 || result.Failed != 0 {
		t.Fatalf("apply: %+v", result)
	}
	for _, entry := range plan.Entries() {
		checkFile(t, entry.Path, "中\nA\n")
	}
}

func TestKeepMixedEOLAndWriteAllTargetEncodings(t *testing.T) {
	for _, target := range []struct {
		encoding document.Encoding
		bytes    string
	}{
		{document.UTF8, "中\rA\r\nB\n"},
		{document.UTF8BOM, "\xef\xbb\xbf中\rA\r\nB\n"},
		{document.UTF16LE, "\xff\xfe\x2d\x4e\r\x00A\x00\r\x00\n\x00B\x00\n\x00"},
		{document.UTF16BE, "\xfe\xff\x4e\x2d\x00\r\x00A\x00\r\x00\n\x00B\x00\n"},
		{document.GB18030, "\xd6\xd0\rA\r\nB\n"},
	} {
		root := t.TempDir()
		source := "中\rA\r\nB\n"
		if target.encoding == document.UTF8 {
			source = "\xef\xbb\xbf" + source
		}
		path := fixture(t, root, "input", source)
		plan, err := Plan(t.Context(), Options{Root: root, TargetEncoding: &target.encoding})
		if err != nil {
			t.Fatal(err)
		}
		if result := Apply(t.Context(), plan); result.Converted != 1 {
			t.Fatalf("apply: %+v", result)
		}
		checkFile(t, path, target.bytes)
	}
}

func TestScanFiltersAndLimits(t *testing.T) {
	root := t.TempDir()
	fixture(t, root, ".git/hidden.txt", "ignore\r")
	fixture(t, root, "a.txt", "a\r")
	fixture(t, root, "b.txt", "b\x00")
	fixture(t, root, "c.md", "\x01\x02abc")
	fixture(t, root, "d.md", "123456789")
	fixture(t, root, "e.txt", "\xff\xfe\x00\xd8")
	fixture(t, root, "folder/one.md", "one\r")
	fixture(t, root, "folder/deep/two.txt", "two\r")
	fixture(t, root, "ignore.go", "ignore\r")
	options := Options{Root: root, Glob: "*.txt; *.md", MaxFileSize: 8, TargetEOL: CRLF}
	preview := func(want map[string]Action) {
		t.Helper()
		plan, err := Plan(t.Context(), options)
		if err != nil {
			t.Fatal(err)
		}
		got := make(map[string]Action)
		for _, entry := range plan.Entries() {
			got[filepath.ToSlash(entry.Relative)] = entry.Action
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("preview: %+v, want %+v", got, want)
		}
	}
	want := map[string]Action{".git": Skip, "a.txt": Convert, "b.txt": Skip, "c.md": Skip, "d.md": Skip, "e.txt": Skip}
	preview(want)
	options.Recursive, options.MaxDepth = true, 1
	want["folder/one.md"] = Convert
	preview(want)
	options.MaxDepth = 2
	want["folder/deep/two.txt"] = Convert
	preview(want)
	options.MaxFiles = 2
	plan, err := Plan(t.Context(), options)
	if err != nil || len(plan.Entries()) != 2 || !plan.Truncated() {
		t.Fatalf("limited preview = %+v, %v", plan, err)
	}
}

func TestApplyRejectsRevisionsAndMissingPreview(t *testing.T) {
	root := t.TempDir()
	changed := fixture(t, root, "a.txt", "a\r")
	cloned := fixture(t, root, "b.txt", "b\r")
	good := fixture(t, root, "c.txt", "c\r")
	plan, err := Plan(t.Context(), Options{Root: root, TargetEOL: CRLF})
	if err != nil {
		t.Fatal(err)
	}
	if result := Apply(t.Context(), nil); result.Err == nil {
		t.Fatal("apply without preview succeeded")
	}
	info, err := os.Stat(changed)
	if err != nil {
		t.Fatal(err)
	}
	fixture(t, root, "a.txt", "x\r")
	if err := os.Chtimes(changed, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(cloned, filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	fixture(t, root, "b.txt", "b\r")
	result := Apply(t.Context(), plan)
	if result.Failed != 2 || result.Converted != 1 || len(result.Errors) != 2 {
		t.Fatalf("apply: %+v", result)
	}
	checkFile(t, changed, "x\r")
	checkFile(t, cloned, "b\r")
	checkFile(t, good, "c\r\n")
}

func TestScanAndApplyDoNotFollowSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	target := fixture(t, outside, "outside.txt", "keep\r")
	if err := os.Symlink(target, filepath.Join(root, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	path := fixture(t, root, "child/outside.txt", "edit\r")
	plan, err := Plan(t.Context(), Options{Root: root, Recursive: true, TargetEOL: LF})
	if err != nil {
		t.Fatal(err)
	}
	if entries := plan.Entries(); len(entries) != 2 || entries[1].Relative != "link.txt" || entries[1].Action != Skip {
		t.Fatalf("preview: %+v", entries)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if result := Apply(t.Context(), plan); result.Failed != 1 || result.Skipped != 1 {
		t.Fatalf("apply: %+v", result)
	}
	checkFile(t, target, "keep\r")
}

func TestCanceledScanAndApply(t *testing.T) {
	root := t.TempDir()
	path := fixture(t, root, "file", "keep\r")
	options := Options{Root: root, TargetEOL: LF}
	plan, err := Plan(t.Context(), options)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Plan(ctx, options); err != context.Canceled {
		t.Fatalf("plan: %v", err)
	}
	if result := Apply(ctx, plan); result.Err != context.Canceled || result.Converted != 0 {
		t.Fatalf("apply: %+v", result)
	}
	checkFile(t, path, "keep\r")
}
