package context

import (
	"strings"
	"sync"
)

// ChangeType indicates the type of change made to the store.
type ChangeType int

const (
	ChangeSet ChangeType = iota
	ChangeDelete
)

// ChangeCallback is called when a value is set or deleted.
// key is the full prefixed key, value is nil for deletes.
type ChangeCallback func(changeType ChangeType, key string, value any)

// MemoryStore is an in-memory implementation of Store using sync.Map.
// It is safe for concurrent use.
type MemoryStore struct {
	data     sync.Map
	onChange ChangeCallback
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// SetOnChange sets a callback that fires on Set/Delete operations.
func (m *MemoryStore) SetOnChange(cb ChangeCallback) {
	m.onChange = cb
}

// Get retrieves a value by key.
func (m *MemoryStore) Get(key string) (any, bool) {
	return m.data.Load(key)
}

// Set stores a value by key.
func (m *MemoryStore) Set(key string, value any) {
	m.data.Store(key, value)
	if m.onChange != nil {
		m.onChange(ChangeSet, key, value)
	}
}

// Delete removes a key.
func (m *MemoryStore) Delete(key string) {
	m.data.Delete(key)
	if m.onChange != nil {
		m.onChange(ChangeDelete, key, nil)
	}
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

// All returns all key-value pairs in the store.
func (m *MemoryStore) All() map[string]any {
	result := make(map[string]any)
	m.data.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			result[k] = value
		}
		return true
	})
	return result
}
