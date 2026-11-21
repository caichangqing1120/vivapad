package runcommand

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestNamedCommandsPrivateRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "run.json")
	want := []Named{{Name: "Open logs", Template: `echo "$(FULL_CURRENT_PATH)"`}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("named Run commands = %+v, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || !privateMode(info.Mode()) {
		t.Fatalf("named Run file mode = %+v, %v", info, err)
	}
	for _, invalid := range [][]Named{
		{{Name: "same", Template: "echo ok"}, {Name: "SAME", Template: "echo again"}},
		{{Name: " bad ", Template: "echo ok"}},
		{{Name: "broken", Template: `"unclosed`}},
	} {
		if err := Save(path, invalid); err == nil {
			t.Fatalf("saved invalid commands %+v", invalid)
		}
	}
	if after, err := Load(path); err != nil || !reflect.DeepEqual(after, want) {
		t.Fatalf("failed save changed prior commands: %+v, %v", after, err)
	}
	if err := os.WriteFile(path, []byte(`{"version":2,"commands":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("accepted future named-Run version")
	}
}

// privateMode reports whether mode carries the private-file metadata this
// platform supports: POSIX 0600 on macOS and Linux, or a writable regular file
// on Windows, where Go reports synthetic permission bits and VivaPad keeps the
// privacy of its stores in the profile directory ACL (a separate gate).
func privateMode(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return mode.IsRegular() && mode.Perm()&0200 != 0
	}
	return mode.Perm() == 0600
}
