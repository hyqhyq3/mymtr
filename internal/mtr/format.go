package mtr

import "time"

func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return d.Round(time.Millisecond).String()
}
