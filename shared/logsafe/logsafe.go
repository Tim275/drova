// Package logsafe sanitizes user-controlled strings before they are written to
// logs, preventing log injection / forged log lines (CWE-117).
package logsafe

import "strings"

// Clean removes CR and LF from s so user input cannot inject or forge extra log
// lines. Apply to any user-controlled value before logging it.
func Clean(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
