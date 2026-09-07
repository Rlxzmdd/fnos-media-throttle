// Package chain owns binding-chain bandwidth policy calculations.
package chain

import (
	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

// PatchForRule converts user-facing KB/s rules into adapter-facing bytes/s.
func PatchForRule(rule domain.ChainRule, viewers int) (domain.LimitPatch, int64, int64) {
	patch := domain.LimitPatch{}
	uploadKB, downloadKB := int64(0), int64(0)
	if rule.Upload.Enabled {
		uploadKB = rule.Upload.Limit(viewers)
		uploadBytes := uploadKB * 1000
		patch.UploadBytes = &uploadBytes
	}
	if rule.Download.Enabled {
		downloadKB = rule.Download.Limit(viewers)
		downloadBytes := downloadKB * 1000
		patch.DownloadBytes = &downloadBytes
	}
	return patch, uploadKB, downloadKB
}
