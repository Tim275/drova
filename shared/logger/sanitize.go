package logger

import "strings"

// Clean removes CR and LF from a user-controlled string before it is logged,
// preventing log injection / forged log lines (CWE-117). Apply to any
// user-controlled value before logging it. (Formerly shared/logsafe.Clean.)
func Clean(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
