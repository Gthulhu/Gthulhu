package util

import "sync"

// GenericMap is a concurrent safe map with generic key and value types.
type GenericMap[K comparable, V any] struct {
	m sync.Map
}

// NewGenericMap creates a new instance of GenericMap.
func NewGenericMap[K comparable, V any]() *GenericMap[K, V] {
	return &GenericMap[K, V]{}
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (m *GenericMap[K, V]) Load(key K) (value V, ok bool) {
	v, loaded := m.m.Load(key)
	if !loaded {
		var zero V
		return zero, false
	}
	return v.(V), true
}

// Store sets the value for a key.
func (m *GenericMap[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}

// Clear deletes all the entries, resulting in an empty Map.
func (m *GenericMap[K, V]) Clear() {
	m.m.Clear()
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *GenericMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	v, loaded := m.m.LoadOrStore(key, value)
	return v.(V), loaded
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *GenericMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		var zero V
		return zero, false
	}
	return v.(V), true
}

// Delete deletes the value for a key.
func (m *GenericMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

// Swap swaps the value for a key and returns the previous value if any.
// The loaded result reports whether the key was present.
func (m *GenericMap[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	v, loaded := m.m.Swap(key, value)
	if !loaded {
		var zero V
		return zero, false
	}
	return v.(V), true
}

// CompareAndSwap swaps the old and new values for key
// if the value stored in the map is equal to old.
// The old value must be of a comparable type.
func (m *GenericMap[K, V]) CompareAndSwap(key K, old, new V) (swapped bool) {
	return m.m.CompareAndSwap(key, old, new)
}

// CompareAndDelete deletes the entry for key if its value is equal to old.
// The old value must be of a comparable type.
//
// If there is no current value for key in the map, CompareAndDelete
// returns false (even if the old value is the nil interface value).
func (m *GenericMap[K, V]) CompareAndDelete(key K, old V) (deleted bool) {
	return m.m.CompareAndDelete(key, old)
}

func (m *GenericMap[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}
