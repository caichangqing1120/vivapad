//go:build windows

package document

import (
	"os"

	"golang.org/x/sys/windows"
)

// openTrashSource opens the file whose identity a TrashPlan pins until the move
// or cancel decision is complete. os.Open asks for FILE_SHARE_READ|FILE_SHARE_WRITE
// only, so holding that handle would make the Recycle Bin move fail with a
// sharing violation; this descriptor also shares delete, which is what the
// IFileOperation delete/rename needs while the plan keeps the source inode alive.
func openTrashSource(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
