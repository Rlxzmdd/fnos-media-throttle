package downloader

// FNOSDownloaderAdapter controls Download Center through fnOS' authenticated
// WebSocket gateway. Credentials are never sent to the embedded qBittorrent.
type FNOSDownloaderAdapter struct{}
