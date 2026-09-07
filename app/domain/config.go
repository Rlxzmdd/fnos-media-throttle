package domain

import "time"

const ConfigFormatVersion = 1

// ConfigBackup is the portable, user-controlled application state. Runtime
// measurements and takeover snapshots are deliberately excluded.
type ConfigBackup struct {
	FormatVersion   int                `json:"formatVersion"`
	AppVersion      string             `json:"appVersion"`
	ExportedAt      time.Time          `json:"exportedAt"`
	IncludesSecrets bool               `json:"includesSecrets"`
	Settings        Settings           `json:"settings"`
	Libraries       []LibraryConfig    `json:"libraries"`
	Downloaders     []DownloaderConfig `json:"downloaders"`
	Chains          []ChainConfig      `json:"chains"`
}

type LibraryConfig struct {
	ID                int64       `json:"id"`
	Name              string      `json:"name"`
	Kind              LibraryKind `json:"kind"`
	BaseURL           string      `json:"baseUrl"`
	APIKey            string      `json:"apiKey,omitempty"`
	PollSeconds       int         `json:"pollSeconds"`
	ActivitySeconds   int         `json:"activitySeconds"`
	CountLocalClients bool        `json:"countLocalClients"`
	Enabled           bool        `json:"enabled"`
}

type DownloaderConfig struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Kind        DownloaderKind `json:"kind"`
	BaseURL     string         `json:"baseUrl"`
	Username    string         `json:"username"`
	Password    string         `json:"password,omitempty"`
	UserID      int64          `json:"userId"`
	Enabled     bool           `json:"enabled"`
	PollSeconds int            `json:"pollSeconds"`
}

type ChainConfig struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Enabled       bool      `json:"enabled"`
	Rule          ChainRule `json:"rule"`
	LibraryIDs    []int64   `json:"libraryIds"`
	DownloaderIDs []int64   `json:"downloaderIds"`
}
