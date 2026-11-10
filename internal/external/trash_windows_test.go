//go:build windows

package external

import "testing"

func TestWindowsTrashRequiresRecycleOnDelete(t *testing.T) {
	want := trashGUID{0x947aab5f, 0x0a5c, 0x4c13, [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8}}
	if fileOperationInterface != want {
		t.Fatalf("IFileOperation IID = %+v, want %+v", fileOperationInterface, want)
	}
	if recycleOperationFlags&0x00080000 == 0 {
		t.Fatal("IFileOperation must request Recycle Bin rather than permanent deletion")
	}
	if recycleOperationFlags&0x00000400 == 0 {
		t.Fatal("IFileOperation must report errors to the application")
	}
}
