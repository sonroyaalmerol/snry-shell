// Package store provides a persistent key-value store backed by a JSON file
// at ~/.config/snry-shell/store.json. It is the generic complement to the
// typed settings.Config: use it for arbitrary values that don't need a fixed
// schema. All operations are safe for concurrent use.
package store

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/sonroyaalmerol/snry-shell/internal/fileutil"
)

var mu sync.RWMutex

func storePath() string {
	return filepath.Join(fileutil.ConfigDir(), "store.json")
}

// load reads the current store file. Returns an empty map if the file does
// not exist. Caller must hold at least a read lock.
func load() map[string]json.RawMessage {
	data, err := os.ReadFile(storePath())
	if os.IsNotExist(err) {
		return make(map[string]json.RawMessage)
	}
	if err != nil {
		return make(map[string]json.RawMessage)
	}
	m := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &m); err != nil {
		log.Printf("[store] corrupt store file, starting fresh: %v", err)
		return make(map[string]json.RawMessage)
	}
	return m
}

func flush(m map[string]json.RawMessage) error {
	path := storePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Get unmarshals the stored value for key into dst.
// Returns false if the key is absent or the value cannot be decoded.
func Get(key string, dst any) bool {
	mu.RLock()
	m := load()
	raw, ok := m[key]
	mu.RUnlock()
	if !ok {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

// Set encodes value as JSON and persists it under key.
func Set(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	m := load()
	m[key] = raw
	return flush(m)
}

// Delete removes key from the store. A no-op if the key does not exist.
func Delete(key string) error {
	mu.Lock()
	defer mu.Unlock()
	m := load()
	delete(m, key)
	return flush(m)
}

// Keys returns all stored keys in sorted order.
func Keys() []string {
	mu.RLock()
	m := load()
	mu.RUnlock()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SetMany persists multiple key-value pairs in a single file write.
func SetMany(pairs map[string]any) error {
	mu.Lock()
	defer mu.Unlock()
	m := load()
	for key, value := range pairs {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		m[key] = raw
	}
	return flush(m)
}

// Lookup retrieves a typed value from the store.
// Returns the zero value of T and false if the key is absent or the stored
// value cannot be decoded as T.
func Lookup[T any](key string) (T, bool) {
	var v T
	ok := Get(key, &v)
	return v, ok
}

// LookupOr returns the stored value for key, or def if the key is absent.
func LookupOr[T any](key string, def T) T {
	v, ok := Lookup[T](key)
	if !ok {
		return def
	}
	return v
}
