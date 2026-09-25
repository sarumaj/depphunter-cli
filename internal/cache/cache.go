// Package cache keeps plugin extractions keyed by file content, so unchanged files are
// never parsed twice: across runs (persisted under the user cache directory) and
// across re-analysis runs in watch mode. A nil *Cache is valid and caches nothing.
package cache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
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
	// done says the run that filled seen finished, so seen is the whole of what the
	// project still uses. A run that failed or was cancelled part way saw only some
	// of it, and pruning by that would throw away every file it did not reach.
	done bool
	// dirty says something was added since the last Save, which otherwise rewrites
	// the whole file for nothing on a --watch re-analysis that parsed nothing.
	dirty bool
}

// Open loads the cache for project root from dir. A missing or unreadable cache file
// yields an empty cache: the cache is an optimization, never a reason to fail.
//
// Implements: REQ-LANG-028
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

// Key identifies an extraction: plugin, its version, the class of the file (its
// extension, which plugins pick grammars by, and whatever else lang.ClassOf says
// the plugin reads) and the content hash.
//
// Implements: REQ-LANG-026
func Key(plugin string, version int, class string, src []byte) string {
	sum := sha256.Sum256(src)
	return fmt.Sprintf("%s/%d/%s/%x", plugin, version, class, sum)
}

// BeginRun starts a new analysis: entries the previous run did not use are dropped,
// if that run finished (EndRun).
func (c *Cache) BeginRun() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done && len(c.seen) > 0 {
		for k := range c.entries {
			if !c.seen[k] {
				delete(c.entries, k)
			}
		}
	}
	c.seen = map[string]bool{}
	c.done = false
}

// EndRun marks the analysis started by BeginRun as finished.
func (c *Cache) EndRun() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.done = true
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
	c.dirty = true
}

// Save writes the entries used by the current run, atomically. It writes nothing when
// nothing was added since the last time.
//
// Implements: REQ-LANG-028
func (c *Cache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if !c.dirty {
		c.mu.Unlock()
		return nil
	}
	c.dirty = false
	data := file{Format: format, Entries: map[string]*lang.Extraction{}}
	for k := range c.seen {
		data.Entries[k] = c.entries[k]
	}
	c.mu.Unlock()
	return c.fail(c.write(data))
}

func (c *Cache) write(data file) error {
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

// fail puts back the mark Save took, so a save that did not happen is tried again.
func (c *Cache) fail(err error) error {
	if err != nil {
		c.mu.Lock()
		c.dirty = true
		c.mu.Unlock()
	}
	return err
}
