package store

import "sync"

type CapacityGuard struct {
	mu sync.Mutex
}

func (g *CapacityGuard) Within(fn func() error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return fn()
}
