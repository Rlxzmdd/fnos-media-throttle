package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEventsMergeIdenticalContent(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "throttle.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	store.Event(ctx, nil, "info", "same")
	store.Event(ctx, nil, "info", "same")
	store.Event(ctx, nil, "info", "same")
	events, err := store.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Message != "same" {
		t.Fatalf("events = %#v, want one merged event", events)
	}
}
