package domain

import "time"

type Event struct {
	ID           int64     `json:"id"`
	DownloaderID *int64    `json:"downloaderId,omitempty"`
	Level        string    `json:"level"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"createdAt"`
}
