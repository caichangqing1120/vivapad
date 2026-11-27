package preferences

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThemeSettingsMigrateAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := `{"version":7,"font_size":17,"sub_font_size":15,"tab_width":8,"wrap":true,"indent_mode":"spaces","change_history_view":"both","print_settings":{"line_numbers":true,"color_mode":1}}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	old, err := Load(path)
	if err != nil || old.Theme != ClassicLight || old.FontSize != 17 || old.SubFontSize != 15 || old.TabWidth != 8 || !old.Wrap || old.IndentMode != "spaces" || old.ChangeHistoryView != "both" {
		t.Fatalf("legacy settings = %+v, %v", old, err)
	}
	for _, theme := range []Theme{ClassicLight, ModernDark, Monokai, FollowSystem} {
		want := old
		want.Theme = theme
		if err := Save(path, want); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil || got != want {
			t.Fatalf("reopened %q = %+v, %v; want %+v", theme, got, err, want)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"theme":"follow-system"`) {
		t.Fatalf("saved settings = %s, %v", data, err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), `"follow-system"`, `"unknown-theme"`)), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got != old {
		t.Fatalf("unknown theme should fall back without losing settings: %+v, %v", got, err)
	}
}
