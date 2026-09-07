// Package media contains media-library contracts and provider implementations.
package media

import (
	"context"

	"github.com/fnos-media-throttle/fnos-media-throttle/domain"
)

// Adapter is the stable extension point implemented by every media provider.
type Adapter interface {
	Test(context.Context, domain.Library) error
	ActiveViewerCount(context.Context, domain.Library) (int, error)
}
