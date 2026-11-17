package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

func TestAlwaysOnTopMenuKeepsDocumentAndOnlyChecksSuccessfulRequests(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("unsaved text")
	doc.Editor.ClearHistory()
	before := doc.Editor.String()
	var requests []bool
	state.requestAlwaysOnTop = func(enabled bool) error {
		requests = append(requests, enabled)
		return nil
	}
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, 26)
	row := syncScrollHit(t, driver, menuItemScope.At(26))
	if row.Y < menuHeight || row.Y+row.Height > 400-statusHeight {
		t.Fatalf("topmost command clipped at 640x400: %+v", row)
	}
	clickID(t, driver, menuItemScope.At(26))
	if !state.alwaysOnTop || len(requests) != 1 || !requests[0] || doc.Editor.String() != before || doc.Editor.CanUndo() {
		t.Fatalf("topmost command changed document or failed: state=%v requests=%v text=%q", state.alwaysOnTop, requests, doc.Editor.String())
	}
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, 26)
	checked := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(26) {
			checked = hit.Checked
		}
	}
	if !checked {
		t.Fatal("successful topmost request is not checked")
	}
	failed := errors.New("window manager denied")
	state.requestAlwaysOnTop = func(bool) error { return failed }
	clickID(t, driver, menuItemScope.At(26))
	if !state.alwaysOnTop || !strings.Contains(state.notice, failed.Error()) || doc.Editor.String() != before {
		t.Fatalf("rejected request cleared topmost: %v, %q", state.alwaysOnTop, state.notice)
	}
	state.requestAlwaysOnTop = func(enabled bool) error { requests = append(requests, enabled); return nil }
	state.command("Always on Top")
	if state.alwaysOnTop || requests[len(requests)-1] || doc.Editor.String() != before {
		t.Fatalf("topmost did not turn off: %v, %v", state.alwaysOnTop, requests)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}

func TestAlwaysOnTopWithoutHostDoesNotClaimSuccess(t *testing.T) {
	state := newModel()
	state.command("Always on Top")
	if state.alwaysOnTop || !strings.Contains(state.notice, "unavailable") {
		t.Fatalf("unsupported topmost = %v, %q", state.alwaysOnTop, state.notice)
	}
}

func TestCaptureAlwaysOnTopMenuWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_TOPMOST_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_TOPMOST_SHOTS to capture the View menu")
	}
	state := newModel()
	state.requestAlwaysOnTop = func(bool) error { return nil }
	web, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := web.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		clickID(t, web.Driver(), menuScope.At(3))
		row := revealViewMenuItem(t, web.Driver(), 26)
		if row.Y < menuHeight || row.Y+row.Height > float32(size.height)-statusHeight {
			t.Fatalf("%s topmost row clipped: %+v", size.name, row)
		}
		if problems := headless.CheckLayout(web.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s topmost menu layout: %+v", size.name, problems)
		}
		if err := web.Screenshot(filepath.Join(output, "always-on-top-menu-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, web.Driver(), menuItemScope.At(26))
	}
}
