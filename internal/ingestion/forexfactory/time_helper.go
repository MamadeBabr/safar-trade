package forexfactory

import (
	"strings"
	"time"
)

// tehranLocation defines the Asia/Tehran timezone (+03:30 standard time).
// Using FixedZone ensures consistent execution across Docker, Alpine, and Windows
// even if timezone database packages (tzdata) are absent.
var tehranLocation = time.FixedZone("Asia/Tehran", 3*3600+30*60)

// FormatTehranTime converts an epoch dateline (seconds) to Tehran local time format (HH:mm).
// If the original time label indicates an untimed event (e.g. "All Day", "Tentative"),
// that label is preserved.
func FormatTehranTime(datelineSec int64, originalLabel string) string {
	trimmed := strings.TrimSpace(originalLabel)
	lower := strings.ToLower(trimmed)

	if lower == "all day" || lower == "tentative" || strings.HasPrefix(lower, "day") {
		return trimmed
	}

	if datelineSec <= 0 {
		return trimmed
	}

	t := time.Unix(datelineSec, 0).In(tehranLocation)
	return t.Format("15:04")
}
