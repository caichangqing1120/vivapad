package completion

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestPathsPreservesAbsoluteQuotedAndRelativeForms(t *testing.T) {
	root := t.TempDir()
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha.txt", "alphabet.txt"} {
		if err := os.WriteFile(filepath.Join(canonical, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"alpha dir", "src"} {
		if err := os.Mkdir(filepath.Join(canonical, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(canonical, "src", "main.go"), []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	raw := filepath.Join(canonical, "alp")
	value := "open \"" + raw
	result, err := Paths(value, len([]rune(value)), canonical)
	want := []string{filepath.Join(canonical, "alpha dir") + string(filepath.Separator), filepath.Join(canonical, "alpha.txt"), filepath.Join(canonical, "alphabet.txt")}
	if err != nil || result.Start != len([]rune("open \"")) || result.End != len([]rune(value)) || !reflect.DeepEqual(result.Candidates, want) {
		t.Fatalf("absolute path candidates = %+v, %v; want %v", result, err, want)
	}
	value = "open \"" + filepath.Join(canonical, "alpha d")
	result, err = Paths(value, len([]rune(value)), canonical)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{filepath.Join(canonical, "alpha dir") + string(filepath.Separator)}) {
		t.Fatalf("quoted path with spaces = %+v, %v", result, err)
	}
	value = "open src/ma"
	result, err = Paths(value, len([]rune(value)), canonical)
	if err != nil || result.Start != len([]rune("open ")) || !reflect.DeepEqual(result.Candidates, []string{"src/main.go"}) {
		t.Fatalf("relative path candidates = %+v, %v", result, err)
	}
	value = "open ma"
	result, err = Paths(value, len([]rune(value)), filepath.Join(canonical, "src"))
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"main.go"}) {
		t.Fatalf("bare relative path candidates = %+v, %v", result, err)
	}
	value = "open " + filepath.Join(canonical, "src")
	result, err = Paths(value, len([]rune(value)), canonical)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{filepath.Join(canonical, "src", "main.go")}) {
		t.Fatalf("existing directory candidates = %+v, %v", result, err)
	}
	value = "open " + filepath.Join(canonical, "alpha.txt")
	result, err = Paths(value, len([]rune(value)), canonical)
	if err != nil || len(result.Candidates) != 0 {
		t.Fatalf("existing file was completed: %+v, %v", result, err)
	}
}

func TestPathsExpandTildeWithoutChangingCandidateText(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("HOME environment is platform-specific on Windows")
	}
	home := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(home)
	if err := os.Mkdir(filepath.Join(canonical, "Documents"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", canonical)
	value := "open ~/Doc"
	result, err := Paths(value, len([]rune(value)), canonical)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"~/Documents/"}) {
		t.Fatalf("tilde path candidates = %+v, %v", result, err)
	}
}

func TestPathsReportsMissingParentAndFollowsExplicitDirectoryLink(t *testing.T) {
	root := t.TempDir()
	canonical, _ := filepath.EvalSymlinks(root)
	if result, err := Paths("ordinary", len([]rune("ordinary")), canonical); err != nil || len(result.Candidates) != 0 {
		t.Fatalf("ordinary word became path: %+v, %v", result, err)
	}
	missing := "open " + filepath.Join(canonical, "missing", "part")
	if _, err := Paths(missing, len([]rune(missing)), canonical); err == nil {
		t.Fatal("missing parent directory was not reported")
	}
	if runtime.GOOS != "windows" {
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "part.txt"), []byte("text"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(canonical, "link")); err != nil {
			t.Fatal(err)
		}
		linkedPath := filepath.Join(canonical, "link", "pa")
		linked := "open " + linkedPath
		result, err := Paths(linked, len([]rune(linked)), canonical)
		if err != nil || !reflect.DeepEqual(result.Candidates, []string{filepath.Join(canonical, "link", "part.txt")}) {
			t.Fatalf("explicit linked path candidates = %+v, %v", result, err)
		}
	}
	if raw, start := pathAtCaret([]rune("before C:\\Wind"), len("before C:\\Wind")); raw != `C:\Wind` || start != len("before ") {
		t.Fatalf("Windows drive extraction = %q at %d", raw, start)
	}
}

func TestPathsBoundsDirectoryEntries(t *testing.T) {
	root := t.TempDir()
	for index := range 2001 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%04d.txt", index)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	canonical, _ := filepath.EvalSymlinks(root)
	value := "open " + canonical + string(filepath.Separator) + "file-"
	result, err := Paths(value, len([]rune(value)), canonical)
	if err != nil || len(result.Candidates) != 2000 || !result.Truncated {
		t.Fatalf("bounded path completion = %d candidates, truncated=%v, err=%v", len(result.Candidates), result.Truncated, err)
	}
	if !strings.HasPrefix(result.Candidates[0], canonical) {
		t.Fatalf("candidate changed base: %q", result.Candidates[0])
	}
}
