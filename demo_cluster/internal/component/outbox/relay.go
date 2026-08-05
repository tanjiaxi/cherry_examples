package outbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/model"
)

type Relay struct {
	repo      *Repository
	publisher *Publisher
	batchSize int
	workers   int
	interval  time.Duration
	wake      chan struct{}

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewRelay(repo *Repository, publisher *Publisher, batchSize, workers int, interval time.Duration) *Relay {
	if repo == nil || publisher == nil {
		panic("outbox relay: nil dependency")
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if workers <= 0 {
		workers = 16
	}
	if workers > batchSize {
		workers = batchSize
	}
	if interval <= 0 {
		interval = 1 * time.Second
	}
	return &Relay{
		repo:      repo,
		publisher: publisher,
		batchSize: batchSize,
		workers:   workers,
		interval:  interval,
		wake:      make(chan struct{}, 1),
	}
}
func (r *Relay) Start(parent context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel, r.done = cancel, make(chan struct{})
	go r.run(ctx)
}
func (r *Relay) Wake() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}
func (r *Relay) Stop(timeout time.Duration) {
	r.mu.Lock()
	cance, done := r.cancel, r.done
	r.cancel, r.done = nil, nil
	r.mu.Unlock()
	if cance == nil {
		return
	}
	cance()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case <-done:
	case <-time.After(timeout):
		clog.Warnf("outbox relay stop timeout after %v", timeout)
	}
}
func (r *Relay) run(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-r.wake:
		}
		if err := r.drain(ctx); err != nil && ctx.Err() == nil {
			clog.Errorf("outbox relay drain: %v", err)
		}
	}
}
func (r *Relay) drain(ctx context.Context) error {
	for {
		batch, err := r.repo.Claim(ctx, r.batchSize)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		events := make(chan model.DomainOutbox)
		errs := make(chan error, len(batch))
		var wg sync.WaitGroup
		for i := 0; i < r.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for event := range events {
					if err := r.processEvent(ctx, event); err != nil {
						errs <- err
					}
				}
			}()
		}
		for _, event := range batch {
			if ctx.Err() != nil {
				close(events)
				wg.Wait()
				return ctx.Err()
			}
			events <- event
		}
		close(events)
		wg.Wait()
		select {
		case err := <-errs:
			return err
		default:
		}
		if len(batch) < r.batchSize {
			return nil
		}
	}
}

func (r *Relay) processEvent(ctx context.Context, event model.DomainOutbox) error {
	publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	err := r.publisher.Publish(publishCtx, event)
	cancel()
	if err != nil {
		if releaseErr := r.repo.ReleaseForRetry(ctx, event.EventID, err); releaseErr != nil {
			return fmt.Errorf("publish %s: %v; release: %w", event.EventID, err, releaseErr)
		}
		return nil
	}
	if err := r.repo.MarkPublished(ctx, event.EventID); err != nil {
		return fmt.Errorf("publish %s succeeded; mark published: %w", event.EventID, err)
	}
	return nil
}
