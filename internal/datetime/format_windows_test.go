package datetime

import (
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/language"
)

func TestWindowsStandardDateTimeFollowsRegionalFormats(t *testing.T) {
	now := time.Date(2026, time.November, 6, 15, 4, 5, 0, time.UTC)
	wall := windows.Systemtime{Year: 2026, Month: 11, Day: 6, Hour: 15, Minute: 4, Second: 5}
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	dateProc := kernel.NewProc("GetDateFormatEx").Addr()
	timeProc := kernel.NewProc("GetTimeFormatEx").Addr()
	var clock [128]uint16
	if n, _, err := syscall.SyscallN(timeProc, 0, 2, uintptr(unsafe.Pointer(&wall)), 0, uintptr(unsafe.Pointer(&clock[0])), uintptr(len(clock))); n == 0 {
		t.Fatalf("TIME_NOSECONDS: %v", err)
	}
	clockText := windows.UTF16ToString(clock[:])
	for _, tc := range []struct {
		name string
		kind Kind
		flag uintptr
	}{{"short", Short, 1}, {"long", Long, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			var date [128]uint16
			if n, _, err := syscall.SyscallN(dateProc, 0, tc.flag, uintptr(unsafe.Pointer(&wall)), 0, uintptr(unsafe.Pointer(&date[0])), uintptr(len(date)), 0); n == 0 {
				t.Fatalf("Windows date formatter: %v", err)
			}
			dateText := windows.UTF16ToString(date[:])
			for _, order := range []struct {
				reverse bool
				want    string
			}{{false, clockText + " " + dateText}, {true, dateText + " " + clockText}} {
				for _, preferred := range []language.Tag{language.AmericanEnglish, language.SimplifiedChinese} {
					got, err := Format(now, tc.kind, "", order.reverse, preferred)
					if err != nil || got != order.want {
						t.Errorf("preferred=%v reverse=%v: got %q, %v; want %q", preferred, order.reverse, got, err, order.want)
					}
				}
			}
		})
	}
}
