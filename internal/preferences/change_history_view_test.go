package preferences

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The change-history view setting is version 6. Older files must migrate to the
// margin default, an explicit "off" must survive a reload, and an unknown value
// must be rejected instead of silently interpreted.

func TestChangeHistoryViewDefaultsToMarginForASavedState(t *testing.T) {
	if Default().ChangeHistoryView != "margin" {
		t.Fatalf("default view = %q, want margin", Default().ChangeHistoryView)
	}
}

func TestChangeHistoryViewMigratesOldFilesToMargin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// A version-5 file: no change_history_view field at all.
	base := Default()
	encoded, err := json.Marshal(struct {
		Version int `json:"version"`
		State
	}{Version: 5, State: base})
	if err != nil {
		t.Fatalf("encode old settings: %v", err)
	}
	// A version-5 writer never knew the field: drop it so the fixture really has
	// no change_history_view key and the migration branch is what supplies it.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode old settings: %v", err)
	}
	delete(fields, "change_history_view")
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("re-encode old settings: %v", err)
	}
	if bytes.Contains(data, []byte("change_history_view")) {
		t.Fatal("the old-settings fixture still carries the new field")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write old settings: %v", err)
	}
	stored, err := Load(path)
	if err != nil {
		t.Fatalf("load old settings: %v", err)
	}
	if stored.ChangeHistoryView != "margin" {
		t.Fatalf("migrated view = %q, want margin", stored.ChangeHistoryView)
	}
}

func TestChangeHistoryViewKeepsExplicitOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	state := Default()
	state.ChangeHistoryView = "off"
	if err := Save(path, state); err != nil {
		t.Fatalf("save: %v", err)
	}
	stored, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.ChangeHistoryView != "off" {
		t.Fatalf("reloaded view = %q, want an explicit off", stored.ChangeHistoryView)
	}
	// Every supported mode round-trips.
	for _, view := range []string{"margin", "indicator", "both", "off"} {
		state.ChangeHistoryView = view
		if err := Save(path, state); err != nil {
			t.Fatalf("save %q: %v", view, err)
		}
		stored, err := Load(path)
		if err != nil || stored.ChangeHistoryView != view {
			t.Fatalf("round trip %q = %q (%v)", view, stored.ChangeHistoryView, err)
		}
	}
}

func TestChangeHistoryViewRejectsUnknownValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	state := Default()
	state.ChangeHistoryView = "fancy"
	if err := Save(path, state); err == nil {
		t.Fatal("save accepted an unknown change-history view")
	}
}
