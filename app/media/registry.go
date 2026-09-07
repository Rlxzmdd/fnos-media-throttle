package media

import (
	"context"
	"fmt"
)

// Registry maps media-library kinds to focused provider implementations.
type Registry struct {
	providers map[LibraryKind]Adapter
}

type Registration struct {
	Kind LibraryKind
	Impl Adapter
}

func NewRegistry(providers ...Registration) Registry {
	items := make(map[LibraryKind]Adapter, len(providers))
	for _, provider := range providers {
		items[provider.Kind] = provider.Impl
	}
	return Registry{providers: items}
}

func (registry Registry) provider(kind LibraryKind) (Adapter, error) {
	provider, ok := registry.providers[kind]
	if !ok || provider == nil {
		return nil, fmt.Errorf("未注册影视库适配器: %s", kind)
	}
	return provider, nil
}

func (registry Registry) Test(ctx context.Context, library Library) error {
	provider, err := registry.provider(library.Kind)
	if err != nil {
		return err
	}
	return provider.Test(ctx, library)
}

func (registry Registry) ActiveViewerCount(ctx context.Context, library Library) (int, error) {
	provider, err := registry.provider(library.Kind)
	if err != nil {
		return 0, err
	}
	return provider.ActiveViewerCount(ctx, library)
}
