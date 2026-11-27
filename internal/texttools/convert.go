package texttools

import (
	"encoding/base64"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func Base64Encode(src string) string { return base64.StdEncoding.EncodeToString([]byte(src)) }

func Base64Decode(src string) (string, error) {
	src = strings.TrimSpace(src)
	for _, codec := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := codec.Strict().DecodeString(src); err == nil {
			return string(decoded), nil
		}
	}
	return "", fmt.Errorf("invalid Base64: expected standard or URL-safe Base64, with or without padding")
}

func URLEncode(src string) string          { return url.QueryEscape(src) }
func URLDecode(src string) (string, error) { return url.QueryUnescape(src) }
func HTMLEscape(src string) string         { return html.EscapeString(src) }
func HTMLUnescape(src string) string       { return html.UnescapeString(src) }

const timestampLayout = "2006-01-02 15:04:05"

func ConvertTimestamp(token string, loc *time.Location) (string, error) {
	token = strings.TrimSpace(token)
	if loc == nil {
		loc = time.Local
	}
	digits := strings.TrimPrefix(strings.TrimPrefix(token, "-"), "+")
	if number, err := strconv.ParseInt(token, 10, 64); err == nil {
		switch len(digits) {
		case 10:
			return time.Unix(number, 0).In(loc).Format(timestampLayout), nil
		case 13:
			return time.UnixMilli(number).In(loc).Format(timestampLayout), nil
		}
	}
	for _, layout := range []string{timestampLayout, time.RFC3339Nano, "2006-01-02T15:04:05"} {
		parsed, err := time.ParseInLocation(layout, token, loc)
		if err == nil {
			clock := token
			if fraction := strings.IndexAny(clock, ".,"); fraction >= 0 {
				clock = clock[:fraction]
			}
			if layout != time.RFC3339Nano && parsed.Format(layout) != clock {
				return "", fmt.Errorf("local date/time does not exist in %s", loc)
			}
			return fmt.Sprintf("%d\n%d", parsed.Unix(), parsed.UnixMilli()), nil
		}
	}
	return "", fmt.Errorf("expected a 10-digit Unix timestamp, a 13-digit Unix timestamp, or YYYY-MM-DD HH:mm:ss")
}
