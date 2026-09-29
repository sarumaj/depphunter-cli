package index

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A flat index is what pip calls find-links and uv a format = "flat" index: one
// HTML page, or one local directory, listing distribution files of every project
// side by side. Its file names say which project and version each file is; a file
// with PEP 658 metadata has it beside it at <file>.metadata.

// errNoLocalMetadata is a release a local flat index holds only as source
// archives, which depphunter does not unpack. Like errNoMetadata it is not "not
// found": the directory has the package.
var errNoLocalMetadata = errors.New("the directory holds no wheel or metadata file for this release, and depphunter does not unpack source archives")

// maxMetadata bounds what is read of a wheel's METADATA.
const maxMetadata = 8 << 20

// pypiFlat reads a distribution's dependencies from a flat index: the files its
// page (or directory) lists, read once per index, the release asked for among
// them, and that release's metadata - the PEP 658 file the page advertises, or for
// a local directory the .metadata file beside the archive or the METADATA inside a
// wheel. A directory is read only when this machine's own configuration names it
// (local).
//
// Implements: REQ-SUP-067, REQ-SUP-066
func (c *Client) pypiFlat(ctx context.Context, page string, local bool, t lang.Target) ([]dependency, error) {
	directory := fileURLPath(page)
	if directory != "" && !local {
		return nil, fmt.Errorf("%s: %w", page, errAbsent) // a directory nothing here vouches for
	}
	files, err := c.flatPages.get(page, func() ([]simpleFile, error) {
		if directory != "" {
			return flatDirectory(directory)
		}
		return c.simpleFiles(ctx, page)
	})
	if err != nil {
		return nil, err
	}
	return c.releaseRequires(ctx, page, files, t)
}

// flatDirectory lists the files at the top of a local flat index. A file has
// metadata when it is a wheel (METADATA inside) or a .metadata file lies beside
// it.
func flatDirectory(directory string) ([]simpleFile, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", directory, errAbsent)
	}
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for _, e := range entries {
		present[e.Name()] = !e.IsDir()
	}
	var out []simpleFile
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".metadata") {
			continue
		}
		name := e.Name()
		file := filepath.Join(directory, name)
		out = append(out, simpleFile{name: name, url: file, path: file,
			metadata: strings.HasSuffix(strings.ToLower(name), ".whl") || present[name+".metadata"]})
	}
	return out, nil
}

// localMetadata is the core metadata of a file of a local flat index: the
// .metadata file beside it, else the METADATA of a wheel's .dist-info directory.
func localMetadata(file string) ([]byte, error) {
	if body, err := readBounded(file + ".metadata"); !errors.Is(err, fs.ErrNotExist) {
		return body, err
	}
	if !strings.HasSuffix(strings.ToLower(file), ".whl") {
		return nil, fmt.Errorf("%s: %w", file, errAbsent)
	}
	archive, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	for _, f := range archive.File {
		directory, base := path.Split(f.Name)
		if base != "METADATA" || strings.Count(directory, "/") != 1 || !strings.HasSuffix(directory, ".dist-info/") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(io.LimitReader(r, maxMetadata))
	}
	return nil, fmt.Errorf("%s: %w", file, errAbsent)
}

// readBounded reads a file of at most maxMetadata bytes.
func readBounded(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxMetadata))
}
