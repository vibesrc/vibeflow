package context

import (
	"strings"
	"sync"
)

// MemoryStore is an in-memory implementation of Store using sync.Map.
// It is safe for concurrent use.
type MemoryStore struct {
	data sync.Map
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Get retrieves a value by key.
func (m *MemoryStore) Get(key string) (any, bool) {
	return m.data.Load(key)
}

// Set stores a value by key.
func (m *MemoryStore) Set(key string, value any) {
	m.data.Store(key, value)
}

// Delete removes a key.
func (m *MemoryStore) Delete(key string) {
	m.data.Delete(key)
}

// Keys returns all keys with the given prefix.
func (m *MemoryStore) Keys(prefix string) []string {
	var keys []string
	m.data.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

// Clear removes all keys with the given prefix.
func (m *MemoryStore) Clear(prefix string) {
	var keysToDelete []string
	m.data.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			keysToDelete = append(keysToDelete, k)
		}
		return true
	})
	for _, k := range keysToDelete {
		m.data.Delete(k)
	}
}
