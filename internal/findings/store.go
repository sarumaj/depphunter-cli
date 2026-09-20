package findings

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// store remembers what the vulnerability database answered, so a second run - and a
// second package asking about the same advisory - costs nothing. Entries expire: what
// is known about a version changes as advisories are published.
type store struct {
	dir string
	ttl time.Duration
}

func newStore(dir string, ttl time.Duration) *store {
	if dir == "" || ttl <= 0 || os.MkdirAll(dir, 0o755) != nil {
		return nil
	}
	return &store{dir: dir, ttl: ttl}
}

type cached struct {
	At    time.Time       `json:"at"`
	Value json.RawMessage `json:"value"`
}

func (s *store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func (s *store) get(key string) (json.RawMessage, bool) {
	if s == nil {
		return nil, false
	}
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return nil, false
	}
	var c cached
	if json.Unmarshal(data, &c) != nil || time.Since(c.At) > s.ttl {
		return nil, false
	}
	return c.Value, true
}

func (s *store) put(key string, value any) {
	if s == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	data, err := json.Marshal(cached{At: time.Now().UTC(), Value: raw})
	if err != nil {
		return
	}
	tmp := s.path(key) + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, s.path(key))
	}
}
