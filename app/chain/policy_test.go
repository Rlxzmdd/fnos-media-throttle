package chain

import (
	"testing"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

func TestRuleControlsDirectionsIndependently(t *testing.T) {
	rule := domain.ChainRule{Upload: domain.DirectionRule{Enabled: true, BaseKB: 10000, StepKB: 2000, MinKB: 3000}, Download: domain.DirectionRule{Enabled: false}}
	patch, uploadKB, downloadKB := PatchForRule(rule, 3)
	if patch.UploadBytes == nil || *patch.UploadBytes != 4000000 || patch.DownloadBytes != nil || uploadKB != 4000 || downloadKB != 0 {
		t.Fatalf("unexpected patch: %#v upload=%d download=%d", patch, uploadKB, downloadKB)
	}
}
