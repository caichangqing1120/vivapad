package windows

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
)

const DocumentPathMainText = "first document\r\n"
const DocumentPathSubText = "second document\r\n"

func DocumentPathFixture(t *testing.T) (mainPath, subPath string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "\u7f16\u8f91 folder")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	mainPath = filepath.Join(dir, "\u732b first.txt")
	subPath = filepath.Join(root, "caf\u00e9 second.txt")
	if err := os.WriteFile(mainPath, []byte(DocumentPathMainText), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subPath, []byte(DocumentPathSubText), 0600); err != nil {
		t.Fatal(err)
	}
	return mainPath, subPath
}

// DocumentPathClipboardValues is ordered as the five commands in the submenu.
func DocumentPathClipboardValues(mainPath, subPath string) [5]string {
	return [5]string{
		mainPath,
		filepath.Base(mainPath),
		filepath.Dir(mainPath),
		filepath.Base(mainPath) + "\r\n" + filepath.Base(subPath) + "\r\n",
		mainPath + "\r\n" + subPath + "\r\n",
	}
}

func CheckDocumentPathClipboard(sequence uintptr, want string) error {
	if ClipboardSequence() == sequence {
		return fmt.Errorf("command did not update the Windows clipboard")
	}
	return retryBusyClipboard(func() error {
		value, column, err := app.ClipboardTextWithFormat()
		if err != nil {
			return err
		}
		if value != want || column {
			return fmt.Errorf("clipboard = %q (column=%t), want plain text %q", value, column, want)
		}
		return nil
	})
}
