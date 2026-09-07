package storage

import "github.com/fnos-media-throttle/fnos-media-throttle/domain"

type Library = domain.Library
type Downloader = domain.Downloader
type BindingChain = domain.BindingChain
type DirectionRule = domain.DirectionRule
type ChainRule = domain.ChainRule
type Settings = domain.Settings
type Event = domain.Event

const (
	LibraryFNOS            = domain.LibraryFNOS
	LibraryEmby            = domain.LibraryEmby
	LibraryJellyfin        = domain.LibraryJellyfin
	LibraryHTTP            = domain.LibraryHTTP
	DownloaderQB           = domain.DownloaderQB
	DownloaderTransmission = domain.DownloaderTransmission
	DownloaderFNOS         = domain.DownloaderFNOS
)
