package domain

import "time"

type LibraryKind string

const (
	LibraryEmby     LibraryKind = "emby"
	LibraryJellyfin LibraryKind = "jellyfin"
	LibraryHTTP     LibraryKind = "http"
	LibraryFNOS     LibraryKind = "fnos"
)

type Library struct {
	ID                int64       `json:"id"`
	Name              string      `json:"name"`
	Kind              LibraryKind `json:"kind"`
	BaseURL           string      `json:"baseUrl"`
	APIKey            string      `json:"apiKey,omitempty"`
	PollSeconds       int         `json:"pollSeconds"`
	ActivitySeconds   int         `json:"activitySeconds"`
	CountLocalClients bool        `json:"countLocalClients"`
	Enabled           bool        `json:"enabled"`
	LastCount         int         `json:"lastCount"`
	LastCheckedAt     *time.Time  `json:"lastCheckedAt,omitempty"`
	LastError         string      `json:"lastError,omitempty"`
}
