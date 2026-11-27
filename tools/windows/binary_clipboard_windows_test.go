package windows_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"vivapad/tools/windows"
)

const (
	nppCopyBinary  = 42048
	nppCutBinary   = 42049
	nppPasteBinary = 42050
)

func TestNotepadPlusPlusBinaryClipboard(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	const left, payload, right = "left:", "A\x00B", ":end\r\n"
	const source = left + payload + right
	t.Run("stream", func(t *testing.T) {
		editor, send, path := openBinaryReference(t, reference, source)
		const from, to = uintptr(len(left)), uintptr(len(left) + len(payload))
		send(2556, from, 0) // SCI_SETEMPTYSELECTION
		sequence := windows.ClipboardSequence()
		invokeNotepadCommand(t, editor, nppCopyBinary)
		if windows.ClipboardSequence() != sequence || send(2159, 0, 0) != 0 {
			t.Fatal("empty binary Copy changed the clipboard or document")
		}

		for _, selection := range [][2]uintptr{{to, from}, {from, to}} {
			send(2572, selection[0], selection[1]) // SCI_SETSELECTION
			if got := invokeBinaryCopy(t, editor, send, nppCopyBinary); got != payload {
				t.Fatalf("binary Copy payload = % x, want % x", got, payload)
			}
			if send(2008, 0, 0) != selection[0] || send(2009, 0, 0) != selection[1] || send(2159, 0, 0) != 0 {
				t.Fatal("binary Copy changed the selection or document")
			}
		}

		if got := invokeBinaryCopy(t, editor, send, nppCutBinary); got != payload {
			t.Fatalf("binary Cut payload = % x, want % x", got, payload)
		}
		if send(2006, 0, 0) != uintptr(len(left+right)) || send(2159, 0, 0) == 0 {
			t.Fatal("binary Cut did not remove the selected bytes")
		}
		assertNotepadSaved(t, editor, path, "\ufeff"+left+right)
		send(2176, 0, 0) // SCI_UNDO
		assertNotepadSaved(t, editor, path, "\ufeff"+source)

		// Replace one byte with the three clipboard bytes to distinguish deletion
		// from insertion when observing how many Undo operations restore the file.
		const pasteAt = uintptr(len(left) + len(payload))
		send(2572, pasteAt+1, pasteAt)
		invokeNotepadCommand(t, editor, nppPasteBinary)
		const pasted = left + payload + payload + "end\r\n"
		if send(2006, 0, 0) != uintptr(len(pasted)) || send(2159, 0, 0) == 0 {
			t.Fatal("binary Paste did not replace the selection with all three bytes")
		}
		assertNotepadSaved(t, editor, path, "\ufeff"+pasted)
		for step := 1; step <= 2; step++ {
			send(2176, 0, 0)
			if send(2006, 0, 0) == uintptr(len(source)) {
				assertNotepadSaved(t, editor, path, "\ufeff"+source)
				t.Logf("stream: copied, cut and pasted % x; paste restored in %d Undo step(s)", payload, step)
				editor.close(t)
				return
			}
		}
		t.Fatal("Undo did not restore the pre-paste byte length")
	})

	const rows = "abc\r\nDEF\r\nghi\r\n"
	for _, shape := range []string{"rectangular", "disjoint"} {
		t.Run(shape, func(t *testing.T) {
			editor, send, _ := openBinaryReference(t, reference, rows)
			if shape == "rectangular" {
				send(2422, 1, 0)           // SCI_SETSELECTIONMODE: SC_SEL_RECTANGLE
				send(2590, 1, 0)           // SCI_SETRECTANGULARSELECTIONANCHOR
				send(2588, 12, 0)          // SCI_SETRECTANGULARSELECTIONCARET
				if send(2423, 0, 0) != 1 { // SCI_GETSELECTIONMODE
					t.Fatal("rectangular selection was not installed")
				}
			} else {
				send(2563, 1, 0) // SCI_SETMULTIPLESELECTION
				send(2572, 2, 0)
				send(2573, 12, 10) // SCI_ADDSELECTION
			}
			count := send(2570, 0, 0) // SCI_GETSELECTIONS
			if count < 2 {
				t.Fatalf("%s fixture has only %d range(s)", shape, count)
			}
			start, end := send(2143, 0, 0), send(2145, 0, 0) // SCI_GETSELECTIONSTART/END
			data := invokeBinaryCopy(t, editor, send, nppCopyBinary)
			if send(2570, 0, 0) != count || send(2006, 0, 0) != uintptr(len(rows)) || send(2159, 0, 0) != 0 {
				t.Fatal("binary Copy changed the range count or document")
			}
			t.Logf("%s: %d ranges, contiguous limits %d..%d, binary marker %d, payload % x", shape, count, start, end, len(data), data)
			editor.close(t)
		})
	}
}

func openBinaryReference(t *testing.T, reference, source string) (*desktopEditor, func(uintptr, uintptr, uintptr) uintptr, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "binary-clipboard.txt")
	if err := os.WriteFile(path, []byte("\ufeff"+source), 0600); err != nil {
		t.Fatal(err)
	}
	editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", path)
	editor.focusText(t)
	send := focusedScintilla(t, editor)
	if length, codePage := send(2006, 0, 0), send(2137, 0, 0); length != uintptr(len(source)) || codePage != 65001 {
		t.Fatalf("reference buffer length=%d code page=%d, want %d bytes in UTF-8", length, codePage, len(source))
	}
	return editor, send, path
}

func invokeBinaryCopy(t *testing.T, editor *desktopEditor, send func(uintptr, uintptr, uintptr) uintptr, command uintptr) string {
	t.Helper()
	wantLength := send(2161, 0, 0) // SCI_GETSELTEXT without an output buffer
	if wantLength == 0 {
		t.Fatal("binary fixture did not select any bytes")
	}
	sequence := windows.ClipboardSequence()
	invokeNotepadCommand(t, editor, command)
	if windows.ClipboardSequence() == sequence {
		t.Fatal("Notepad++ did not publish binary clipboard data")
	}
	result := readNotepadBinaryClipboard(t)
	if uintptr(len(result)) != wantLength {
		t.Fatalf("binary length marker = %d, want SCI_GETSELTEXT length %d", len(result), wantLength)
	}
	return result
}

func readNotepadBinaryClipboard(t *testing.T) string {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	user := syscall.NewLazyDLL("user32.dll")
	kernel := syscall.NewLazyDLL("kernel32.dll")
	name, err := syscall.UTF16PtrFromString("Notepad++ Binary Length")
	if err != nil {
		t.Fatal(err)
	}
	marker, _, err := user.NewProc("RegisterClipboardFormatW").Call(uintptr(unsafe.Pointer(name)))
	if marker == 0 {
		t.Fatalf("register binary clipboard marker: %v", err)
	}
	open := user.NewProc("OpenClipboard")
	for attempt := 0; ; attempt++ {
		ok, _, err := open.Call(0)
		if ok != 0 {
			break
		}
		if err != syscall.ERROR_ACCESS_DENIED || attempt == 9 {
			t.Fatalf("open clipboard: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer user.NewProc("CloseClipboard").Call()
	read := func(format uintptr, data []byte) {
		t.Helper()
		handle, _, err := user.NewProc("GetClipboardData").Call(format)
		if handle == 0 {
			t.Fatalf("clipboard format %d: %v", format, err)
		}
		if size, _, _ := kernel.NewProc("GlobalSize").Call(handle); size < uintptr(len(data)) {
			t.Fatalf("clipboard format %d has %d bytes, need %d", format, size, len(data))
		}
		address, _, err := kernel.NewProc("GlobalLock").Call(handle)
		if address == 0 {
			t.Fatalf("lock clipboard format %d: %v", format, err)
		}
		defer kernel.NewProc("GlobalUnlock").Call(handle)
		kernel.NewProc("RtlMoveMemory").Call(uintptr(unsafe.Pointer(&data[0])), address, uintptr(len(data)))
	}
	var length [4]byte
	read(marker, length[:])
	n := binary.LittleEndian.Uint32(length[:])
	if n == 0 || n > 4096 {
		t.Fatalf("reference wrote an invalid fixture length: %d", n)
	}
	data := make([]byte, n+1)
	read(1, data) // CF_TEXT; the registered length excludes the terminator.
	if data[n] != 0 {
		t.Fatal("binary clipboard data lacks the CF_TEXT terminator")
	}
	return string(data[:n])
}
