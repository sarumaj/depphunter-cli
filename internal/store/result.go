package store

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// WriteAtomic writes file with write, so that a reader finds the previous content or
// the new, never a part of it: write fills a scratch file of its own beside file (put-*),
// which is then renamed over it. The scratch file is named per write rather than
// after file: two writers of the same file - two depphunters over one project, say -
// would otherwise write one scratch file over each other, and the rename would
// publish whichever half won. Nothing is left behind when write or the rename fails.
func WriteAtomic(file string, write func(io.Writer) error) error {
	temporary, err := os.CreateTemp(filepath.Dir(file), "put-*")
	if err != nil {
		return err
	}
	err = write(temporary)
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporary.Name(), file)
	}
	if err != nil {
		os.Remove(temporary.Name())
	}
	return err
}

// Result keeps one kind of result computed for one project - its git history, its
// symbol references - as gzipped JSON, one file per key: what the result was computed
// from, such as the HEAD commit. Every file name starts with a hash of the project's
// root and the kind, so a Save replaces the project's results of that kind for other
// keys, and nobody else's.
//
// A Result of an empty directory keeps nothing: that is what --no-cache comes to.
type Result[T any] struct {
	directory string
	prefix    string // the file names' common start, directory included
}

// NewResult is the result of the given kind for the project at root, kept in directory.
func NewResult[T any](directory, root, kind string) Result[T] {
	sum := sha256.Sum256([]byte(root))
	return Result[T]{directory: directory, prefix: filepath.Join(directory, hex.EncodeToString(sum[:12])+"-"+kind+"-")}
}

// File is where the result for key is kept.
func (r Result[T]) File(key string) string { return r.prefix + key + ".json.gz" }

// Load decodes the result kept for key. Anything missing or unreadable is a miss.
func (r Result[T]) Load(key string) (*T, bool) {
	if r.directory == "" {
		return nil, false
	}
	f, err := os.Open(r.File(key))
	if err != nil {
		return nil, false
	}
	defer f.Close()
	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return nil, false
	}
	var value T
	if json.NewDecoder(gzipReader).Decode(&value) != nil {
		return nil, false
	}
	return &value, true
}

// Save keeps value as the result for key, in place of the project's results for any
// other key, or silently does not.
func (r Result[T]) Save(key string, value *T) {
	if r.directory == "" || os.MkdirAll(r.directory, 0o755) != nil {
		return
	}
	old, _ := filepath.Glob(r.prefix + "*.json.gz")
	for _, o := range old {
		os.Remove(o)
	}
	WriteAtomic(r.File(key), func(w io.Writer) error {
		gzipWriter := gzip.NewWriter(w)
		if err := json.NewEncoder(gzipWriter).Encode(value); err != nil {
			return err
		}
		return gzipWriter.Close()
	})
}
