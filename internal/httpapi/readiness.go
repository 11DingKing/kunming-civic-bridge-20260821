package httpapi

import "sync"

type readinessResult struct {
	status int
	body   map[string]any
}

type readinessCache struct {
	mu     sync.RWMutex
	result *readinessResult
}

func (c *readinessCache) load() (*readinessResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result, c.result != nil
}

func (c *readinessCache) store(result *readinessResult) {
	c.mu.Lock()
	c.result = result
	c.mu.Unlock()
}
