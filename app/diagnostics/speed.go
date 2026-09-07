package diagnostics

import (
	"fmt"
	"strings"
)

// Speed formats decimal kilobytes per second using the UI's kb/s spelling.
// Keep byte precision for sub-kilobyte values; never change protocol units.
func Speed(bytes int64) string {
	whole, fraction := bytes/1000, bytes%1000
	if fraction == 0 {
		return fmt.Sprintf("%d kb/s", whole)
	}
	if fraction < 0 {
		fraction = -fraction
	}
	sign := ""
	if bytes < 0 && whole == 0 {
		sign = "-"
	}
	decimal := strings.TrimRight(fmt.Sprintf("%03d", fraction), "0")
	return fmt.Sprintf("%s%d.%s kb/s", sign, whole, decimal)
}
