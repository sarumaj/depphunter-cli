package index

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// store keeps what an index answered, so a second run - and a second package asking
// about the same dependency - costs nothing. Entries expire: a package's dependencies
// change with its versions.
type store struct {
	dir string
	ttl time.Duration
}

func newStore(dir string, ttl time.Duration) *store {
	if dir == "" {
		return nil
	}
	if os.MkdirAll(dir, 0o755) != nil {
		return nil
	}
	return &store{dir: dir, ttl: ttl}
}

type entry struct {
	At   time.Time `json:"at"`
	Deps []dep     `json:"deps"`
}

func (s *store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func (s *store) get(key string) ([]dep, bool) {
	if s == nil {
		return nil, false
	}
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return nil, false
	}
	var e entry
	if json.Unmarshal(data, &e) != nil || time.Since(e.At) > s.ttl {
		return nil, false
	}
	return e.Deps, true
}

func (s *store) put(key string, deps []dep) {
	if s == nil {
		return
	}
	data, err := json.Marshal(entry{At: time.Now().UTC(), Deps: deps})
	if err != nil {
		return
	}
	tmp := s.path(key) + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, s.path(key))
	}
}
