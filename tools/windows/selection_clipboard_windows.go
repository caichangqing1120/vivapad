package windows

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/vivaui/viva/app"
)

// SelectionClipboardSource is a mixed-EOL fixture shared by both editors.
const SelectionClipboardSource = "a\r\nb\nc\rd"

// SelectionHashSource includes a file BOM that is not part of the selection.
const SelectionHashSource = "\ufeffcaf\u00e9 caf\u00e9\r\nxy\rZ\n"
const SelectionHashSHA256 = "f7345909f561b145ff55ab82fe11cb3808b5b55a35eae29ae172dd2eed182e19"

func ClipboardSequence() uintptr {
	value, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("GetClipboardSequenceNumber").Call()
	return value
}

func CheckSelectionHashClipboard() error {
	return retryBusyClipboard(func() error {
		value, err := app.ClipboardText()
		if err != nil {
			return err
		}
		if value != SelectionHashSHA256 {
			return fmt.Errorf("clipboard digest = %q, want %q", value, SelectionHashSHA256)
		}
		return nil
	})
}

// CheckSelectionClipboard verifies the UTF-16 payload and both column formats.
func CheckSelectionClipboard() error {
	const want = "c\rd\r\n\r\na\r\n\r\n"
	var value string
	var column bool
	err := retryBusyClipboard(func() error {
		var err error
		value, column, err = app.ClipboardTextWithFormat()
		return err
	})
	if err != nil || value != want || !column || !app.ClipboardRectangular() {
		return fmt.Errorf("clipboard = %q, column=%v, error=%v; want %q and MSDEVColumnSelect", value, column, err, want)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	user := syscall.NewLazyDLL("user32.dll")
	kernel := syscall.NewLazyDLL("kernel32.dll")
	name, err := syscall.UTF16PtrFromString("Borland IDE Block Type")
	if err != nil {
		return err
	}
	format, _, err := user.NewProc("RegisterClipboardFormatW").Call(uintptr(unsafe.Pointer(name)))
	if format == 0 {
		return fmt.Errorf("register Borland clipboard format: %w", err)
	}
	openClipboard := user.NewProc("OpenClipboard")
	if err := retryBusyClipboard(func() error {
		if ok, _, err := openClipboard.Call(0); ok == 0 {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("open clipboard to check Borland marker: %w", err)
	}
	defer user.NewProc("CloseClipboard").Call()
	handle, _, err := user.NewProc("GetClipboardData").Call(format)
	if handle == 0 {
		return fmt.Errorf("read Borland marker: %w", err)
	}
	if size, _, _ := kernel.NewProc("GlobalSize").Call(handle); size != 1 {
		return fmt.Errorf("Borland marker size = %d, want 1", size)
	}
	address, _, err := kernel.NewProc("GlobalLock").Call(handle)
	if address == 0 {
		return fmt.Errorf("lock Borland clipboard marker: %w", err)
	}
	defer kernel.NewProc("GlobalUnlock").Call(handle)
	// Copy the marker without round-tripping the address through unsafe.Pointer.
	var marker byte
	kernel.NewProc("RtlMoveMemory").Call(uintptr(unsafe.Pointer(&marker)), address, 1)
	if marker != 0x02 {
		return fmt.Errorf("Borland marker = %#x, want 0x02", marker)
	}
	return nil
}

func retryBusyClipboard(read func() error) error {
	for attempt := 0; ; attempt++ {
		err := read()
		if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) || attempt == 9 {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
}
