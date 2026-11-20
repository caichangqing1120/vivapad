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

func TestPathsOffersOnlyNamesThatFitTheCurrentToken(t *testing.T) {
	root := t.TempDir()
	names := []string{"alpha.txt", "alpha space.txt", "alpha[box].txt", "alpha'draft.txt"}
	if runtime.GOOS != "windows" {
		names = append(names, `alpha"draft.txt`, "alpha\nnext.txt", "alpha\rnext.txt")
	}
	if runtime.GOOS == "linux" {
		names = append(names, "alpha\xff.txt")
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	bare := []string{"alpha.txt"}
	doubleQuoted := []string{"alpha space.txt", "alpha'draft.txt", "alpha.txt", "alpha[box].txt"}
	singleQuoted := []string{"alpha space.txt", "alpha.txt", "alpha[box].txt"}
	if runtime.GOOS != "windows" {
		singleQuoted = []string{"alpha space.txt", `alpha"draft.txt`, "alpha.txt", "alpha[box].txt"}
	}
	for _, tc := range []struct {
		prefix, suffix string
		want           []string
	}{
		{"open alp", " tail", bare},
		{`open "alp`, `" tail`, doubleQuoted},
		{"open 'alp", "' tail", singleQuoted},
	} {
		result, err := Paths(tc.prefix+tc.suffix, len(tc.prefix), root)
		if err != nil || result.End != len(tc.prefix) || !reflect.DeepEqual(result.Candidates, tc.want) {
			t.Errorf("Paths(%q) = %+v, %v; want candidates %q ending at %d", tc.prefix+tc.suffix, result, err, tc.want, len(tc.prefix))
		}
	}
}

func TestPathsIgnoresApostrophesInsideWordsBeforeQuotedPath(t *testing.T) {
	root := t.TempDir()
	const target = "alpha draft.txt"
	if err := os.WriteFile(filepath.Join(root, target), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, left := range []string{`it's ready: "`, `l'éditeur: '`} {
		quote := left[len(left)-1:]
		const typed, tail = "alpha dr", "-old.txt"
		value := left + typed + tail + quote + " now"
		start := len([]rune(left))
		result, err := Paths(value, start+len(typed), root)
		if err != nil || result.Start != start || result.End != start+len(typed+tail) || !reflect.DeepEqual(result.Candidates, []string{target}) {
			t.Errorf("Paths(%q) = %+v, %v; want %q in %d:%d", value, result, err, target, start, start+len(typed+tail))
		}
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

func TestPathsTreatsTrailingSeparatorAsDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{filepath.Join(root, "alpha.txt"), filepath.Join(root, "src", "main.go")} {
		if err := os.WriteFile(file, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, input string
		want        []string
		wantErr     bool
	}{
		{"missing directory", "al/", nil, true},
		{"regular file", "alpha.txt/", nil, true},
		{"existing directory", "./src/", []string{"./src/main.go"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Paths(tc.input, len(tc.input), root)
			if (err != nil) != tc.wantErr || !reflect.DeepEqual(result.Candidates, tc.want) {
				t.Fatalf("Paths(%q) = %v, %v; want %v, error=%t", tc.input, result.Candidates, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestPathsClassifiesSymlinksByTheirTargets(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "target")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "child.txt")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, link := range []struct{ name, target string }{
		{"link-dir", directory},
		{"link-file", file},
		{"link-missing", filepath.Join(root, "missing")},
	} {
		if err := os.Symlink(link.target, filepath.Join(root, link.name)); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("cannot create symlink: %v", err)
			}
			t.Fatal(err)
		}
	}
	sep := string(filepath.Separator)
	for _, tc := range []struct {
		input string
		want  []string
	}{
		{"li", []string{"link-dir" + sep, "link-file"}},
		{"link-dir" + sep + "ch", []string{"link-dir" + sep + "child.txt"}},
	} {
		result, err := Paths(tc.input, len(tc.input), root)
		if err != nil || !reflect.DeepEqual(result.Candidates, tc.want) {
			t.Errorf("Paths(%q) = %v, %v; want %v", tc.input, result.Candidates, err, tc.want)
		}
	}
}

func TestPathsLetsFilesystemResolveParentAfterSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows applies different path normalization rules")
	}
	root := t.TempDir()
	base, target := filepath.Join(root, "base"), filepath.Join(root, "target")
	nested := filepath.Join(target, "nested")
	for _, dir := range []string{base, nested} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(base, "match-wrong"), filepath.Join(target, "match-file")} {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(nested, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nested, filepath.Join(target, "match-dir")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", base)
	for _, prefix := range []string{"link/../", base + "/link/../", "~/link/../"} {
		t.Run(prefix, func(t *testing.T) {
			input := prefix + "ma"
			want := []string{prefix + "match-dir/", prefix + "match-file"}
			result, err := Paths(input, len([]rune(input)), base)
			if err != nil || !reflect.DeepEqual(result.Candidates, want) {
				t.Fatalf("Paths(%q) = %v, %v; want %v", input, result.Candidates, err, want)
			}
			for _, candidate := range result.Candidates {
				if available, err := PathCandidateAvailable(candidate, base); err != nil || !available {
					t.Errorf("candidate %q = %v, %v; want available", candidate, available, err)
				}
			}
			if exists, err := PathNamesFile(want[1], base); err != nil || !exists {
				t.Errorf("existing file %q was not recognized: %v, %v", want[1], exists, err)
			}
		})
	}
	const missingParent = "missing/../ma"
	if _, err := Paths(missingParent, len(missingParent), base); err == nil {
		t.Fatal("completed a path through a missing directory")
	}
}

func TestPathCandidateUnavailableWhenFileBecomesDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "target")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Paths("tar", 3, root)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"target"}) {
		t.Fatalf("expected a regular-file candidate, got %v, %v", result.Candidates, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if available, err := PathCandidateAvailable(result.Candidates[0], root); err != nil || available {
		t.Fatalf("old file candidate = %v, %v; want unavailable", available, err)
	}
	result, err = Paths("tar", 3, root)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"target" + string(filepath.Separator)}) {
		t.Fatalf("expected a refreshed directory candidate, got %v, %v", result.Candidates, err)
	}
	if available, err := PathCandidateAvailable(result.Candidates[0], root); err != nil || !available {
		t.Fatalf("refreshed directory candidate = %v, %v; want available", available, err)
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

func TestPathPrefixMatchesCaseFoldWithoutAllocating(t *testing.T) {
	for _, tc := range []struct {
		name, prefix string
		want         bool
	}{
		{"alpha", "", true},
		{"ALPHA", "alpha", true},
		{"\u212aite.txt", "ki", true},
		{"Kite.txt", "\u212ai", true},
		{"\u00c9tude.txt", "\u00e9t", true},
		{"\u00e9.txt", "e\u0301", false},
		{"no", "none", false},
	} {
		if got := pathPrefix(tc.name, tc.prefix); got != tc.want {
			t.Errorf("pathPrefix(%q, %q) = %v, want %v", tc.name, tc.prefix, got, tc.want)
		}
	}
	name, prefix := strings.Repeat("Quarter", 25)+".txt", strings.Repeat("quarter", 20)
	if allocs := testing.AllocsPerRun(100, func() {
		if !pathPrefix(name, prefix) {
			t.Fatal("long mixed-case prefix did not match")
		}
	}); allocs != 0 {
		t.Errorf("matching a long filename prefix allocated %.1f times", allocs)
	}
}

func BenchmarkPathsLargeMixedCaseDirectory(b *testing.B) {
	root := b.TempDir()
	for index := range 2000 {
		name := fmt.Sprintf("Report-%04d-Quarterly-Summary-Archive.txt", index)
		if err := os.WriteFile(filepath.Join(root, name), nil, 0600); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		result, err := Paths("report-", len("report-"), root)
		if err != nil || len(result.Candidates) != 2000 {
			b.Fatalf("got %d candidates, err=%v", len(result.Candidates), err)
		}
	}
}
