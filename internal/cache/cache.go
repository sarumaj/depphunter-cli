// Package cache keeps plugin extractions keyed by file content, so unchanged files are
// never parsed twice: across runs (persisted under the user cache directory) and
// across re-analyzes in watch mode. A nil *Cache is valid and caches nothing.
package cache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// format changes whenever the on-disk layout does; older files are ignored.
const format = 1

type file struct {
	Format  int
	Entries map[string]*lang.Extraction
}

type Cache struct {
	path    string
	mu      sync.Mutex
	entries map[string]*lang.Extraction
	seen    map[string]bool // keys used by the current run
}

// Open loads the cache for project root from dir. A missing or unreadable cache file
// yields an empty cache: the cache is an optimisation, never a reason to fail.
func Open(dir, root string) *Cache {
	sum := sha256.Sum256([]byte(root))
	c := &Cache{
		path:    filepath.Join(dir, hex.EncodeToString(sum[:12])+".gob"),
		entries: map[string]*lang.Extraction{},
		seen:    map[string]bool{},
	}
	if f, err := os.Open(c.path); err == nil {
		defer f.Close()
		var data file
		if gob.NewDecoder(f).Decode(&data) == nil && data.Format == format && data.Entries != nil {
			c.entries = data.Entries
		}
	}
	return c
}

// Key identifies an extraction: plugin, its version, the file extension (plugins
// pick grammars by it) and the content hash.
func Key(plugin string, version int, filePath string, src []byte) string {
	sum := sha256.Sum256(src)
	return fmt.Sprintf("%s/%d/%s/%x", plugin, version, path.Ext(filePath), sum)
}

// BeginRun starts a new analysis: entries the previous run did not use are dropped.
func (c *Cache) BeginRun() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.seen) > 0 {
		for k := range c.entries {
			if !c.seen[k] {
				delete(c.entries, k)
			}
		}
	}
	c.seen = map[string]bool{}
}

func (c *Cache) Get(key string) (*lang.Extraction, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ex, ok := c.entries[key]
	if ok {
		c.seen[key] = true
	}
	return ex, ok
}

func (c *Cache) Put(key string, ex *lang.Extraction) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = ex
	c.seen[key] = true
}

// Save writes the entries used by the current run, atomically.
func (c *Cache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	data := file{Format: format, Entries: map[string]*lang.Extraction{}}
	for k := range c.seen {
		data.Entries[k] = c.entries[k]
	}
	c.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".cache-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := gob.NewEncoder(tmp).Encode(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), c.path)
}
