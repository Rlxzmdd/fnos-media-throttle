package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSQLiteConcurrentSoakWithoutBusy(t *testing.T) {
	s, l, _, d, _ := makeChainFixture(t)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := BindingChain{Name: "soak", Enabled: true, LibraryIDs: []int64{l.ID}, DownloaderIDs: []int64{d.ID}, Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 1000}}}
	if err := s.SaveChain(ctx, &c); err != nil {
		t.Fatal(err)
	}
	var journal string
	if err := s.DB.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journal); err != nil || journal != "wal" {
		t.Fatalf("journal=%s error=%v", journal, err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				var err error
				switch worker % 4 {
				case 0:
					update := c
					update.Name = fmt.Sprintf("soak-%d-%d", worker, i)
					err = s.SaveChain(ctx, &update)
				case 1:
					_, err = s.Chains(ctx)
					if err == nil {
						_, err = s.ActiveChainForDownloader(ctx, d.ID)
					}
				case 2:
					err = s.SaveSettings(ctx, Settings{Debug: i%2 == 0})
					if err == nil {
						err = s.SetLibraryStatus(ctx, l.ID, i%3, "")
					}
				case 3:
					err = s.Event(ctx, &d.ID, "info", "same event")
					if err == nil {
						_, err = s.Events(ctx)
					}
				}
				if err != nil {
					errors <- err
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("concurrent storage error (including SQLITE_BUSY): %v", err)
	}
	var count int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE message='same event'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate event rows=%d err=%v", count, err)
	}
	t.Log("400 mixed concurrent iterations completed; zero SQLITE_BUSY / storage errors")
}

func TestConcurrentChainConflictCheckedInTransaction(t *testing.T) {
	s, l, _, d, _ := makeChainFixture(t)
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			c := BindingChain{Name: "competing", Enabled: true, LibraryIDs: []int64{l.ID}, DownloaderIDs: []int64{d.ID}, Rule: ChainRule{Upload: DirectionRule{Enabled: true, BaseKB: 100}}}
			results <- s.SaveChain(ctx, &c)
		}()
	}
	close(start)
	success := 0
	for i := 0; i < 2; i++ {
		if err := <-results; err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("competing active chains accepted: %d", success)
	}
}
