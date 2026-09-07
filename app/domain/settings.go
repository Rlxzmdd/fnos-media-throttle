package domain

import "errors"

const (
	ReleaseOriginal  = "original"
	ReleaseUnlimited = "unlimited"
	ReleaseMaximum   = "maximum"
)

type Settings struct {
	Debug       bool   `json:"debug"`
	ReleaseMode string `json:"releaseMode"`
	AppVersion  string `json:"appVersion,omitempty"`
}

func (s *Settings) Normalize() error {
	if s.ReleaseMode == "" {
		s.ReleaseMode = ReleaseOriginal
	}
	switch s.ReleaseMode {
	case ReleaseOriginal, ReleaseUnlimited, ReleaseMaximum:
		return nil
	default:
		return errors.New("无效的退出限速策略")
	}
}
