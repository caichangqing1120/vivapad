//go:build !windows

package datetime

import (
	"fmt"
	"time"

	"golang.org/x/text/language"
)

func standardParts(now time.Time, kind Kind, locale language.Tag) (date, clock string, err error) {
	base, _ := locale.Base()
	region, _ := locale.Region()
	switch {
	case base.String() == "zh":
		date = fmt.Sprintf("%d/%d/%d", now.Year(), now.Month(), now.Day())
		if kind == Long {
			weekday := [...]string{"日", "一", "二", "三", "四", "五", "六"}
			date = fmt.Sprintf("%d年%d月%d日 星期%s", now.Year(), now.Month(), now.Day(), weekday[now.Weekday()])
		}
		clock = now.Format("15:04")
	case base.String() == "en" && region.String() == "US":
		date = now.Format("1/2/2006")
		if kind == Long {
			date = now.Format("Monday, January 2, 2006")
		}
		clock = now.Format("3:04 PM")
	default:
		date = now.Format("02/01/2006")
		if kind == Long {
			date = now.Format("Monday, 2 January 2006")
		}
		clock = now.Format("15:04")
	}
	return date, clock, nil
}
