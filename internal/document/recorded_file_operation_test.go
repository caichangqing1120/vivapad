package document

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordedFileOperationsRefuseRedirectedParent(t *testing.T) {
	for _, op := range []struct {
		name    string
		refusal error
		prepare func(*testing.T, *Document, string, *int) (func() error, error)
	}{
		{"rename", ErrRenameSourceChanged, func(_ *testing.T, doc *Document, target string, _ *int) (func() error, error) {
			plan, err := doc.PrepareRename(target)
			if err != nil {
				return nil, err
			}
			return func() error { return plan.Apply(true) }, nil
		}},
		{"trash", ErrTrashSourceChanged, func(t *testing.T, doc *Document, target string, calls *int) (func() error, error) {
			plan, err := doc.PrepareTrash()
			if err != nil {
				return nil, err
			}
			t.Cleanup(func() { _ = plan.Close() })
			return func() error {
				return plan.Apply(func(path string) error {
					(*calls)++
					return os.Rename(path, target)
				})
			}, nil
		}},
	} {
		for _, pending := range []bool{false, true} {
			stage := "before preparation"
			if pending {
				stage = "after preparation"
			}
			t.Run(op.name+"/"+stage, func(t *testing.T) {
				root := t.TempDir()
				original, peer, moved := filepath.Join(root, "original"), filepath.Join(root, "peer"), filepath.Join(root, "moved")
				for _, dir := range []string{original, peer} {
					if err := os.Mkdir(dir, 0700); err != nil {
						t.Fatal(err)
					}
				}
				name := "notes.txt"
				source, target := filepath.Join(original, name), filepath.Join(root, "target.txt")
				for path, text := range map[string]string{source: "saved\r\n", filepath.Join(peer, name): "saved\r\n", target: "keep target"} {
					if err := os.WriteFile(path, []byte(text), 0600); err != nil {
						t.Fatal(err)
					}
				}
				doc, err := Open(source)
				if err != nil {
					t.Fatal(err)
				}
				bound, digest := doc.Path, doc.DiskDigest()
				doc.Editor.Insert("draft ")
				draft := doc.Editor.String()
				calls := 0
				redirect := peer
				var apply func() error
				if pending {
					apply, err = op.prepare(t, doc, target, &calls)
					if err != nil {
						t.Fatalf("prepare at original path: %v", err)
					}
					// Keep file identity identical so the pending operation must reject the path itself.
					redirect = moved
				}
				if err := os.Rename(original, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(redirect, original); err != nil {
					if os.IsPermission(err) {
						t.Skipf("directory symlinks unavailable: %v", err)
					}
					t.Fatal(err)
				}
				if pending {
					err = apply()
				} else {
					_, err = op.prepare(t, doc, target, &calls)
				}
				if !errors.Is(err, op.refusal) || calls != 0 {
					t.Fatalf("redirected %s: %v, trash calls=%d", op.name, err, calls)
				}
				if doc.Path != bound || doc.DiskDigest() != digest || !doc.Dirty() || doc.Editor.String() != draft {
					t.Fatal("refusal changed the document or its saved baseline")
				}
				for path, want := range map[string]string{filepath.Join(moved, name): "saved\r\n", filepath.Join(peer, name): "saved\r\n", target: "keep target"} {
					if got, err := os.ReadFile(path); err != nil || string(got) != want {
						t.Fatalf("file %q changed on refusal: %q, %v", path, got, err)
					}
				}
				if err := os.Remove(original); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(moved, original); err != nil {
					t.Fatal(err)
				}
				apply, err = op.prepare(t, doc, target, &calls)
				if err != nil {
					t.Fatalf("prepare after restoring original path: %v", err)
				}
				if err := apply(); err != nil {
					t.Fatalf("apply after restoring original path: %v", err)
				}
				if got, err := os.ReadFile(target); err != nil || string(got) != "saved\r\n" {
					t.Fatalf("restored operation moved wrong bytes: %q, %v", got, err)
				}
			})
		}
	}
}
