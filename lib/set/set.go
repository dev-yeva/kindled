package set

import (
	"log/slog"
	"sync"
)

type numeric interface {
	int8 | int16 | int32 | int64 | int | uint8 | uint16 | uint32 | uint64 | uint
}

type Set[K comparable] struct {
	data map[K]struct{}
	mu   sync.RWMutex
}

func New[K comparable, N numeric](size N) *Set[K] {
	return &Set[K]{data: make(map[K]struct{}, size)}
}

func (s *Set[K]) LogValue() slog.Value {
	s.mu.RLock()
	keys := make([]K, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	s.mu.RUnlock()
	return slog.AnyValue(keys)
}

func (s *Set[K]) Range() func(func(K) bool) {
	s.mu.RLock()
	keys := make([]K, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	s.mu.RUnlock()
	return func(yield func(K) bool) {
		for _, k := range keys {
			if !yield(k) {
				return
			}
		}
	}
}

func (s *Set[K]) Add(key K) {
	s.mu.Lock()
	s.data[key] = struct{}{}
	s.mu.Unlock()
}

func (s *Set[K]) AddSlice(keys []K) {
	s.mu.Lock()
	for _, k := range keys {
		if _, exists := s.data[k]; !exists {
			s.data[k] = struct{}{}
		}
	}
	s.mu.Unlock()
}

func (s *Set[K]) Delete(key K) {
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}

func (s *Set[K]) Size() int {
	s.mu.RLock()
	size := len(s.data)
	s.mu.RUnlock()
	return size
}
