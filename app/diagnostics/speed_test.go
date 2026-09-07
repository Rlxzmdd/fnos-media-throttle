package diagnostics

import "testing"

func TestSpeedUsesKBForEveryValue(t *testing.T) {
	for value, want := range map[int64]string{0: "0 kb/s", 100000: "100 kb/s", 1536: "1.536 kb/s", 1: "0.001 kb/s", 1200: "1.2 kb/s", -1: "-0.001 kb/s"} {
		if got := Speed(value); got != want {
			t.Errorf("Speed(%d)=%s, want %s", value, got, want)
		}
	}
}
