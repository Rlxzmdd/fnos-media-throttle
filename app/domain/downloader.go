package domain

import "time"

type DownloaderKind string

const (
	DownloaderQB           DownloaderKind = "qbittorrent"
	DownloaderTransmission DownloaderKind = "transmission"
	DownloaderFNOS         DownloaderKind = "fnos"
)

type Downloader struct {
	ID                     int64          `json:"id"`
	Name                   string         `json:"name"`
	Kind                   DownloaderKind `json:"kind"`
	BaseURL                string         `json:"baseUrl"`
	Username               string         `json:"username"`
	Password               string         `json:"password,omitempty"`
	UserID                 int64          `json:"userId"`
	Enabled                bool           `json:"enabled"`
	PollSeconds            int            `json:"pollSeconds"`
	OriginalUploadBytes    *int64         `json:"originalUploadBytes,omitempty"`
	OriginalDownloadBytes  *int64         `json:"originalDownloadBytes,omitempty"`
	CurrentUploadKB        int64          `json:"currentUploadKB"`
	CurrentDownloadKB      int64          `json:"currentDownloadKB"`
	CurrentUploadSpeedKB   int64          `json:"currentUploadSpeedKB"`
	CurrentDownloadSpeedKB int64          `json:"currentDownloadSpeedKB"`
	ViewerCount            int            `json:"viewerCount"`
	LastAppliedAt          *time.Time     `json:"lastAppliedAt,omitempty"`
	LastError              string         `json:"lastError,omitempty"`
}

type TransferStats struct {
	UploadSpeedBytes   int64
	DownloadSpeedBytes int64
	UploadLimitBytes   int64
	DownloadLimitBytes int64
}

type LimitPatch struct {
	UploadBytes   *int64
	DownloadBytes *int64
}
