package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"

	"vivapad/internal/document"
	"vivapad/internal/searchfiles"
)

func TestCompactDirectoryReplaceShowsNonRegularTargetCause(t *testing.T) {
	root := filepath.Join(t.TempDir(), strings.Repeat("long-directory-", 10))
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	first, changed := filepath.Join(root, "a.txt"), filepath.Join(root, "z.txt")
	for _, path := range []string{first, changed} {
		if err := os.WriteFile(path, []byte("cat"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := searchfiles.PreviewReplace(t.Context(), root, "cat", "dog", document.SearchOptions{}, searchfiles.Options{Pattern: "*.txt"})
	if err != nil || len(plan.Changes) != 2 {
		t.Fatalf("expected two preview targets: %v, %v", plan, err)
	}
	if err := os.Remove(changed); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(changed, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	state.replacePlan, state.replaceConfirmOpen = plan, true
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, replaceConfirmID)
	for deadline := time.Now().Add(5 * time.Second); state.filesApplying && time.Now().Before(deadline); {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if state.filesApplying || state.replaceConfirmOpen {
		t.Fatal("replacement did not finish")
	}
	if data, err := os.ReadFile(first); err != nil || string(data) != "cat" {
		t.Fatalf("refused batch changed the first file: %q, %v", data, err)
	}
	for _, label := range driver.Snapshot().Texts {
		if strings.Contains(label.Value, "not a regular file; no files written") {
			return
		}
	}
	t.Fatalf("compact status hid the refusal reason or write outcome: %q", state.notice)
}
