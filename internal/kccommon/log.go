/*
FILE: internal/kccommon/log.go

DESCRIPTION:
Helpers for putting raw wire payloads into log lines. A dropped frame is only
diagnosable when the operator can see what the venue actually sent, but a
full level2 snapshot or a batch response in a warn line would flood the log —
hence a hard byte cap with an explicit "…" marker.
*/

package kccommon

// TruncateForLog returns data as a string cut to at most limit bytes; a cut
// payload ends with "…(<total> bytes)" so the reader knows it is partial.
// limit <= 0 means "no cap".
func TruncateForLog(data []byte, limit int) string {
	if limit <= 0 || len(data) <= limit {
		return string(data)
	}
	return string(data[:limit]) + "…(" + itoa(len(data)) + " bytes)"
}

// itoa — minimal non-negative integer formatter (avoids importing strconv
// into a helper that is called from hot-path-adjacent code only on failure).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	var i int = len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
