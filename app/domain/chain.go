package domain

import "time"

type DirectionRule struct {
	Enabled bool  `json:"enabled"`
	BaseKB  int64 `json:"baseKB"`
	StepKB  int64 `json:"stepKB"`
	MinKB   int64 `json:"minKB"`
}

func (rule DirectionRule) Limit(viewers int) int64 {
	if !rule.Enabled {
		return 0
	}
	return Limit(rule.BaseKB, rule.StepKB, rule.MinKB, viewers)
}

type ChainRule struct {
	Upload   DirectionRule `json:"upload"`
	Download DirectionRule `json:"download"`
}

type BindingChain struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Enabled       bool       `json:"enabled"`
	Rule          ChainRule  `json:"rule"`
	LibraryIDs    []int64    `json:"libraryIds"`
	DownloaderIDs []int64    `json:"downloaderIds"`
	ViewerCount   int        `json:"viewerCount"`
	LastError     string     `json:"lastError,omitempty"`
	CreatedAt     *time.Time `json:"createdAt,omitempty"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

func Limit(base, step, minimum int64, viewers int) int64 {
	value := base - step*int64(viewers)
	if value < minimum {
		return minimum
	}
	return value
}
