// Package store keeps what somebody else's server said, so that asking again costs
// nothing: what a package index answered about a package's dependencies, and what the
// vulnerability database answered about a version. Both are facts about a published
// artifact rather than about this repository, which is why they survive the run that
// fetched them - and why they expire, since a package gains dependencies with its
// versions and a version gains advisories with time.
//
// A nil *Store is valid and keeps nothing. That is what --no-cache comes to, and what
// an unavailable cache directory comes to, so no caller has to ask which.
//
// The package also holds what every on-disk cache of depphunter writes with:
// WriteAtomic, and Result for what was computed for one project - its git history,
// its symbol references - rather than asked of somebody else.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	directory string
	ttl       time.Duration
}

// New opens the store under directory, keeping answers for ttl. It returns nil - a store
// that keeps nothing - when there is nowhere to write or nothing to be gained.
//
// Implements: REQ-FND-012
func New(directory string, ttl time.Duration) *Store {
	if directory == "" || ttl <= 0 || os.MkdirAll(directory, 0o755) != nil {
		return nil
	}
	sweep(directory, ttl)
	return &Store{directory: directory, ttl: ttl}
}

// sweep removes answers past their time to live. A stale answer is replaced only when
// the same question is asked again, and most never are, so without a sweep the
// directory grows with every version ever asked about. It goes by file age, which is
// never less than the age of the answer inside, and gives a Put's scratch file an hour
// in case that Put is still running.
func sweep(directory string, ttl time.Duration) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		limit := ttl
		switch {
		case strings.HasPrefix(name, "put-"):
			limit = min(ttl, time.Hour)
		case !strings.HasSuffix(name, ".json"):
			continue
		}
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() && time.Since(info.ModTime()) > limit {
			os.Remove(filepath.Join(directory, name))
		}
	}
}

// entry is one answer with the moment it was given.
type entry struct {
	At    time.Time       `json:"at"`
	Value json.RawMessage `json:"value"`
}

// Get decodes the answer kept under key, if one is and it is still fresh. Anything
// unreadable, stale or of the wrong shape is a miss: the store is an optimization,
// never a reason to fail.
func Get[T any](s *Store, key string) (T, bool) {
	var value T
	if s == nil {
		return value, false
	}
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return value, false
	}
	var e entry
	if json.Unmarshal(data, &e) != nil || time.Since(e.At) > s.ttl {
		return value, false
	}
	if json.Unmarshal(e.Value, &value) != nil {
		var zero T
		return zero, false
	}
	return value, true
}

// Put records an answer under key, or silently does not.
func (s *Store) Put(key string, value any) {
	if s == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	data, err := json.Marshal(entry{At: time.Now().UTC(), Value: raw})
	if err != nil {
		return
	}
	// Two writers asking about the same package at once - two depphunters over two
	// folders, say - each publish a whole answer (WriteAtomic).
	WriteAtomic(s.path(key), func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// path is where an answer to key is kept. The key is whatever the caller finds
// distinguishing - a URL, an ecosystem and a version - so it is hashed rather than
// trusted to be a file name.
func (s *Store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.directory, hex.EncodeToString(sum[:])+".json")
}
