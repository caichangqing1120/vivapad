package datetime

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/language"
)

const (
	dateShortDate = 0x00000001
	dateLongDate  = 0x00000002
	timeNoSeconds = 0x00000002
)

var (
	regionalKernel = windows.NewLazySystemDLL("kernel32.dll")
	getDateFormat  = regionalKernel.NewProc("GetDateFormatEx")
	getTimeFormat  = regionalKernel.NewProc("GetTimeFormatEx")
)

func standardParts(now time.Time, kind Kind, _ language.Tag) (date, clock string, err error) {
	year, month, day := now.Date()
	if year < 1 || year > 0xffff {
		return "", "", fmt.Errorf("year %d cannot be formatted by Windows", year)
	}
	hour, minute, second := now.Clock()
	wall := windows.Systemtime{
		Year: uint16(year), Month: uint16(month), Day: uint16(day), DayOfWeek: uint16(now.Weekday()),
		Hour: uint16(hour), Minute: uint16(minute), Second: uint16(second), Milliseconds: uint16(now.Nanosecond() / 1_000_000),
	}
	dateFlag := uintptr(dateShortDate)
	if kind == Long {
		dateFlag = dateLongDate
	}
	// A nil locale name uses the user's Windows regional settings, including overrides.
	var dateText, clockText [128]uint16
	if n, _, callErr := getDateFormat.Call(0, dateFlag, uintptr(unsafe.Pointer(&wall)), 0, uintptr(unsafe.Pointer(&dateText[0])), uintptr(len(dateText)), 0); n == 0 {
		return "", "", fmt.Errorf("Windows date format: %w", callErr)
	}
	if n, _, callErr := getTimeFormat.Call(0, timeNoSeconds, uintptr(unsafe.Pointer(&wall)), 0, uintptr(unsafe.Pointer(&clockText[0])), uintptr(len(clockText))); n == 0 {
		return "", "", fmt.Errorf("Windows time format: %w", callErr)
	}
	return windows.UTF16ToString(dateText[:]), windows.UTF16ToString(clockText[:]), nil
}
