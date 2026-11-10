package macro

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vivaui/viva/app"
)

func TestNamedMacrosRoundTripAndRejectInvalidWithoutOverwriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "named-macros.json")
	want := []Named{{Name: "Insert 日本", Steps: []Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "日本"}}},
		{Name: "Header", Steps: []Step{{Kind: TextStep, TextKind: app.TextInsert, Text: "# "}}}}
	if err := SaveNamed(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadNamed(path); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("named macros = %+v, %v", got, err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]Named{
		{{Name: " ", Steps: want[0].Steps}},
		{{Name: "bad\nname", Steps: want[0].Steps}},
		{{Name: "empty"}},
		{want[1], {Name: "header", Steps: want[0].Steps}},
		{{Name: "invalid", Steps: []Step{{Kind: LineStep, Line: "unknown"}}}},
	} {
		if err := SaveNamed(path, invalid); err == nil {
			t.Fatalf("invalid named macros accepted: %+v", invalid)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != string(original) {
			t.Fatalf("invalid save changed store: %q, %v", data, err)
		}
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("named store permissions = %+v, %v", info, err)
	}
}

func TestNamedMacrosRejectCorruptOrUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "named-macros.json")
	for _, content := range []string{
		`{"version":2,"macros":[]}`,
		`{"version":1,"macros":[{"name":"one","steps":[]}]}`,
		`{"version":1,"macros":[],"extra":true}`,
		`{"version":1,"macros":[]} trailing`,
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := LoadNamed(path); err == nil {
			t.Fatalf("invalid store accepted: %+v (%s)", got, content)
		}
	}
}
