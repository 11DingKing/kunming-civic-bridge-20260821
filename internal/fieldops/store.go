package fieldops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu     sync.RWMutex
	path   string
	state  state
	closed bool
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, state: newState()}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read field operation state: %w", err)
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return nil, fmt.Errorf("decode field operation state: %w", err)
	}
	normalizeState(&s.state)
	return s, nil
}

func (s *Store) Update(ctx context.Context, fn func(*state) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("update closed store: %w", ErrConflict)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	next := s.state.clone()
	if err := fn(&next); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.persist(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *Store) View(ctx context.Context, fn func(state) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("view closed store: %w", ErrConflict)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(s.state.clone())
}

func (s *Store) persist(next state) error {
	data, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("encode field operation state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create field operation directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".fieldops-*")
	if err != nil {
		return fmt.Errorf("create field operation snapshot: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write field operation snapshot: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync field operation snapshot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close field operation snapshot: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("publish field operation snapshot: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if err := s.persist(s.state); err != nil {
		return err
	}
	s.closed = true
	return nil
}

func scoped(tenant, value string) string {
	return tenant + "\x00" + value
}
