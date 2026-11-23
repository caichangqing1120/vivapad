package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextDirectionRoundTripsInBothSessionFormats(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "text.txt")
	native := filepath.Join(dir, "session.json")
	if err := Save(native, State{Paths: []string{doc}, Active: doc, RTL: map[string]bool{doc: true}}); err != nil {
		t.Fatal(err)
	}
	state, err := Load(native)
	if err != nil || !state.RTL[doc] {
		t.Fatalf("native direction lost: %+v, %v", state, err)
	}
	legacy, err := decodeSession(strings.NewReader(`{"version":12,"paths":[]}`))
	if err != nil || len(legacy.RTL) != 0 {
		t.Fatalf("legacy session should default to LTR: %+v, %v", legacy, err)
	}
	npp := filepath.Join(dir, "session.xml")
	if err := SaveNPP(npp, NPPState{Main: []NPPFile{{Path: doc, Encoding: -1, RTL: true}}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(npp)
	if err != nil || !strings.Contains(string(data), `RTL="yes"`) {
		t.Fatalf("missing Notepad++ RTL attribute: %s, %v", data, err)
	}
	imported, err := LoadNPP(npp)
	if err != nil || len(imported.Main) != 1 || !imported.Main[0].RTL {
		t.Fatalf("NPP direction lost: %+v, %v", imported, err)
	}
}
