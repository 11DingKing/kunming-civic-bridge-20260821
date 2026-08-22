package fieldops

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Worker struct {
	service  *Service
	tenant   string
	owner    string
	interval time.Duration
	cancel   context.CancelFunc
	done     chan struct{}
	started  atomic.Bool
	mu       sync.Mutex
}

func NewWorker(service *Service, tenant, owner string, interval time.Duration) *Worker {
	return &Worker{service: service, tenant: tenant, owner: owner, interval: interval}
}

func (w *Worker) Start(parent context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started.Load() {
		return ErrConflict
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel, w.done = cancel, make(chan struct{})
	w.started.Store(true)
	go w.loop(ctx)
	return nil
}

func (w *Worker) loop(ctx context.Context) {
	defer close(w.done)
	defer w.started.Store(false)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = w.service.RequeueExpired(ctx, w.tenant)
		}
	}
}

func (w *Worker) Stop() {
	w.mu.Lock()
	if w.cancel == nil {
		w.mu.Unlock()
		return
	}
	cancel, done := w.cancel, w.done
	w.cancel = nil
	w.mu.Unlock()
	cancel()
	<-done
}

func (w *Worker) Started() bool { return w.started.Load() }
