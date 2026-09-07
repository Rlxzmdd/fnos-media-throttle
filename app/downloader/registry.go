package downloader

import (
	"context"
	"fmt"
)

// Registry routes a saved downloader to its concrete implementation.
type Registry struct {
	providers map[DownloaderKind]Adapter
}

type Registration struct {
	Kind DownloaderKind
	Impl Adapter
}

func (registry Registry) BeginCycle(ctx context.Context, value Downloader) (context.Context, func()) {
	provider, err := registry.provider(value.Kind)
	if err == nil {
		if cycle, ok := provider.(CycleAdapter); ok {
			return cycle.BeginCycle(ctx, value)
		}
	}
	return ctx, func() {}
}

func NewRegistry(providers ...Registration) Registry {
	items := make(map[DownloaderKind]Adapter, len(providers))
	for _, provider := range providers {
		items[provider.Kind] = provider.Impl
	}
	return Registry{providers: items}
}

func (registry Registry) provider(kind DownloaderKind) (Adapter, error) {
	provider := registry.providers[kind]
	if provider == nil {
		return nil, fmt.Errorf("未注册下载器适配器: %s", kind)
	}
	return provider, nil
}

func (registry Registry) Test(ctx context.Context, value Downloader) error {
	provider, err := registry.provider(value.Kind)
	if err != nil {
		return err
	}
	return provider.Test(ctx, value)
}

func (registry Registry) Limits(ctx context.Context, value Downloader) (int64, int64, error) {
	provider, err := registry.provider(value.Kind)
	if err != nil {
		return 0, 0, err
	}
	return provider.Limits(ctx, value)
}

func (registry Registry) Stats(ctx context.Context, value Downloader) (TransferStats, error) {
	provider, err := registry.provider(value.Kind)
	if err != nil {
		return TransferStats{}, err
	}
	return provider.Stats(ctx, value)
}

func (registry Registry) ApplyLimits(ctx context.Context, value Downloader, patch LimitPatch) error {
	provider, err := registry.provider(value.Kind)
	if err != nil {
		return err
	}
	return provider.ApplyLimits(ctx, value, patch)
}
