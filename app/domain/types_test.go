package domain

import "testing"

func TestDirectionRuleLimits(t *testing.T) {
	rule := ChainRule{
		Upload:   DirectionRule{Enabled: true, BaseKB: 10000, StepKB: 2000, MinKB: 3000},
		Download: DirectionRule{Enabled: true, BaseKB: 20000, StepKB: 5000, MinKB: 4000},
	}
	for _, tc := range []struct {
		viewers  int
		up, down int64
	}{{0, 10000, 20000}, {3, 4000, 5000}, {99, 3000, 4000}} {
		if up, down := rule.Upload.Limit(tc.viewers), rule.Download.Limit(tc.viewers); up != tc.up || down != tc.down {
			t.Fatalf("viewers=%d limits=%d/%d", tc.viewers, up, down)
		}
	}
}
