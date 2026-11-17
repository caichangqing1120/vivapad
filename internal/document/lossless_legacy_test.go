package document

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestGB18030LossyOutputRefusesEveryPrewritePath(t *testing.T) {
	for _, action := range []string{"validate", "length", "save", "copy"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			source, target := filepath.Join(root, "source.txt"), filepath.Join(root, "copy.txt")
			original := []byte("A\uE000\r\n")
			if err := os.WriteFile(source, original, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := Open(source)
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := filepath.EvalSymlinks(source)
			if err != nil {
				t.Fatal(err)
			}
			doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
			doc.Editor.Insert("!")
			if !doc.Editor.Undo() || !doc.Editor.CanRedo() {
				t.Fatal("missing fixture redo")
			}
			doc.Editor.SetSelection(text.Range{Start: 2, End: 0})
			selection, caret := doc.Editor.Selection(), doc.Editor.Caret()
			doc.SetEncoding(GB18030)
			switch action {
			case "validate":
				err = doc.ValidateEncoding()
			case "length":
				_, err = GB18030.EncodedLength(doc.Editor.String())
			case "save":
				err = doc.Save("")
			case "copy":
				err = doc.SaveCopy(target)
			}
			if err == nil || !strings.Contains(err.Error(), "GB18030") {
				t.Fatalf("%s accepted lossy U+E000: %v", action, err)
			}
			if doc.Path != canonical || doc.Encoding() != GB18030 || doc.SavedEncoding() != UTF8 ||
				!doc.Dirty() || !doc.Editor.AtSavePoint() || !doc.Editor.CanRedo() || doc.Editor.Selection() != selection || doc.Editor.Caret() != caret || doc.Editor.String() != "A\uE000\n" {
				t.Fatal("refusal changed source, selection, saved boundary or future Undo")
			}
			if disk, err := os.ReadFile(source); err != nil || !bytes.Equal(disk, original) {
				t.Fatalf("source bytes changed: %x, %v", disk, err)
			}
			if disk, err := os.ReadFile(target); err != nil || string(disk) != "existing" {
				t.Fatalf("destination bytes changed: %x, %v", disk, err)
			}
			if !doc.Editor.Redo() || doc.Editor.String() != "A\uE000\n!" {
				t.Fatal("refusal consumed the future edit")
			}
			doc.SetEncoding(UTF8)
			if err := doc.Save(""); err != nil {
				t.Fatalf("lossless retry failed: %v", err)
			}
			if disk, err := os.ReadFile(source); err != nil || !bytes.Equal(disk, []byte("A\uE000\r\n!")) || doc.Dirty() {
				t.Fatalf("lossless retry changed text: %x, %v", disk, err)
			}
		})
	}
}

func TestGB18030LosslessOutputStillMatchesByteLength(t *testing.T) {
	value := "中文\r\n"
	encoded, err := encodeText(value, GB18030)
	if err != nil || !bytes.Equal(encoded, []byte{0xd6, 0xd0, 0xce, 0xc4, '\r', '\n'}) {
		t.Fatalf("lossless GB18030 output: %x, %v", encoded, err)
	}
	length, err := GB18030.EncodedLength(value)
	if err != nil || length != len(encoded) {
		t.Fatalf("lossless byte length: %d, %v", length, err)
	}
}
