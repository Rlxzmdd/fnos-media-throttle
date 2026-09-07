package downloader

import (
	"context"

	"github.com/fnos-media-throttle/fnos-media-throttle/diagnostics"
)

func (a FNOSDownloaderAdapter) BeginCycle(ctx context.Context, d Downloader) (context.Context, func()) {
	return withFNOSGatewayCycle(ctx, d)
}

func (a FNOSDownloaderAdapter) Test(ctx context.Context, d Downloader) error {
	gateway, release, err := acquireFNOSGateway(ctx, d)
	if err != nil {
		return err
	}
	defer release()
	_, err = gateway.transferConfig(ctx)
	return err
}

func (a FNOSDownloaderAdapter) Limits(ctx context.Context, d Downloader) (int64, int64, error) {
	gateway, release, err := acquireFNOSGateway(ctx, d)
	if err != nil {
		return 0, 0, err
	}
	defer release()
	config, err := gateway.transferConfig(ctx)
	if err != nil {
		return 0, 0, err
	}
	return fnosToAppBytes(int64Value(config["torrent_up_limit"])), fnosToAppBytes(int64Value(config["torrent_dl_limit"])), nil
}

func (a FNOSDownloaderAdapter) Stats(ctx context.Context, d Downloader) (TransferStats, error) {
	gateway, release, err := acquireFNOSGateway(ctx, d)
	if err != nil {
		return TransferStats{}, err
	}
	defer release()
	config, err := gateway.transferConfig(ctx)
	if err != nil {
		return TransferStats{}, err
	}
	response, err := gateway.request(ctx, "appcgi.downloadcenter.stat.all", map[string]any{"uid": gateway.uid})
	if err != nil {
		return TransferStats{}, err
	}
	stats := unwrapFNOSMap(response)
	result := TransferStats{
		UploadSpeedBytes: int64Value(stats["up_speed"]), DownloadSpeedBytes: int64Value(stats["dl_speed"]),
		UploadLimitBytes: fnosToAppBytes(int64Value(config["torrent_up_limit"])), DownloadLimitBytes: fnosToAppBytes(int64Value(config["torrent_dl_limit"])),
	}
	debugf(ctx, "飞牛下载网关统计：uid=%d，当前上传=%s，当前下载=%s", gateway.uid, diagnostics.Speed(result.UploadSpeedBytes), diagnostics.Speed(result.DownloadSpeedBytes))
	return result, nil
}

func (a FNOSDownloaderAdapter) ApplyLimits(ctx context.Context, d Downloader, patch LimitPatch) error {
	if patch.UploadBytes == nil && patch.DownloadBytes == nil {
		return nil
	}
	gateway, release, err := acquireFNOSGateway(ctx, d)
	if err != nil {
		return err
	}
	defer release()
	config, err := gateway.transferConfig(ctx)
	if err != nil {
		return err
	}
	if patch.UploadBytes != nil {
		config["torrent_up_limit"] = appToFNOSBytes(*patch.UploadBytes)
	}
	if patch.DownloadBytes != nil {
		config["stream_dl_limit"] = appToFNOSBytes(*patch.DownloadBytes)
		config["torrent_dl_limit"] = appToFNOSBytes(*patch.DownloadBytes)
	}
	up := fnosToAppBytes(int64Value(config["torrent_up_limit"]))
	down := fnosToAppBytes(int64Value(config["torrent_dl_limit"]))
	config["speed_limit_enabled"] = up > 0 || down > 0
	config["uid"] = gateway.uid
	_, err = gateway.request(ctx, "appcgi.downloadcenter.config.setTransferCfg", map[string]any{"transfer_cfg": config})
	if err == nil {
		debugf(ctx, "飞牛下载网关限速已写入：uid=%d，上传=%s，下载=%s", gateway.uid, diagnostics.Speed(up), diagnostics.Speed(down))
	}
	return err
}

func appToFNOSBytes(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return value * 1024 / 1000
}

func fnosToAppBytes(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return value * 1000 / 1024
}
