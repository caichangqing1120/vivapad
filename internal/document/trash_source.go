//go:build !windows

package document

import "os"

// openTrashSource opens the file whose identity a TrashPlan pins until the move
// or cancel decision is complete. POSIX unlink and rename do not need a share
// mode, so a plain read-only descriptor is enough.
func openTrashSource(path string) (*os.File, error) {
	return os.Open(path)
}
