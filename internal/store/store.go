package store

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/fileutil"
)

// kv holds the in-memory key-value store. Reads are lock-free.
var kv = xsync.NewMap[string, json.RawMessage]()

// flushMu serializes file writes so concurrent Set/Delete/SetMany calls
// don't interleave their file I/O.
var flushMu sync.Mutex

func storePath() string {
	return filepath.Join(fileutil.ConfigDir(), "store.json")
}

// loadFromFile reads the store file and populates the in-memory map.
func loadFromFile() {
	data, err := os.ReadFile(storePath())
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		return
	}
	m := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &m); err != nil {
		log.Printf("[store] corrupt store file, starting fresh: %v", err)
		return
	}
	for k, v := range m {
		kv.Store(k, v)
	}
}

// init loads persisted data on first use.
var loadOnce sync.Once

func ensureLoaded() {
	loadOnce.Do(loadFromFile)
}

// Reset clears the in-memory store and reloads from disk.
// Used to isolate tests that change HOME.
func Reset() {
	loadOnce = sync.Once{}
	kv = xsync.NewMap[string, json.RawMessage]()
}

func flushToDisk() error {
	m := xsync.ToPlainMap(kv)
	path := storePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// Get unmarshals the stored value for key into dst.
// Returns false if the key is absent or the value cannot be decoded.
func Get(key string, dst any) bool {
	ensureLoaded()
	raw, ok := kv.Load(key)
	if !ok {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

// Set encodes value as JSON and persists it under key.
func Set(key string, value any) error {
	ensureLoaded()
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	kv.Store(key, raw)
	flushMu.Lock()
	defer flushMu.Unlock()
	return flushToDisk()
}

// Delete removes key from the store. A no-op if the key does not exist.
func Delete(key string) error {
	ensureLoaded()
	kv.Delete(key)
	flushMu.Lock()
	defer flushMu.Unlock()
	return flushToDisk()
}

// Keys returns all stored keys in sorted order.
func Keys() []string {
	ensureLoaded()
	keys := make([]string, 0)
	kv.Range(func(k string, _ json.RawMessage) bool {
		keys = append(keys, k)
		return true
	})
	sort.Strings(keys)
	return keys
}

// SetMany persists multiple key-value pairs in a single file write.
func SetMany(pairs map[string]any) error {
	ensureLoaded()
	for key, value := range pairs {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		kv.Store(key, raw)
	}
	flushMu.Lock()
	defer flushMu.Unlock()
	return flushToDisk()
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
