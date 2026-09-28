package jsonnet

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dep is one dependency of a jsonnetfile.json or jsonnetfile.lock.json.
type dep struct {
	remote  string // git remote, as written
	subdir  string
	dir     string // a local source's directory, relative to the jsonnetfile
	version string
	sum     string
	name    string // the "name" field: the legacy import name when set
	line    int
}

func (d *dep) local() bool { return d.remote == "" }

// pkg is the package name: the repository (lang.RepoName) and subdir, which is
// also where jsonnet-bundler installs it under vendor/ and the path its
// full-path imports start with.
func (d *dep) pkg() string {
	if d.local() {
		return d.dir
	}
	p := lang.RepoName(d.remote)
	if sub := strings.Trim(path.Clean("/"+d.subdir), "/"); sub != "" {
		p += "/" + sub
	}
	return p
}

// legacy is the short name jsonnet-bundler links vendor/<name> to (legacy
// imports): the name field, else the subdir's last element, else the
// repository's (a local source: its directory's).
func (d *dep) legacy() string {
	if d.name != "" {
		return d.name
	}
	if d.local() {
		return path.Base(path.Clean(d.dir))
	}
	if sub := strings.Trim(path.Clean("/"+d.subdir), "/"); sub != "" {
		return path.Base(sub)
	}
	return path.Base(lang.RepoName(d.remote))
}

type jbFile struct {
	Dependencies []struct {
		Source struct {
			Git *struct {
				Remote string `json:"remote"`
				Subdir string `json:"subdir"`
			} `json:"git"`
			Local *struct {
				Directory string `json:"directory"`
			} `json:"local"`
		} `json:"source"`
		Version string `json:"version"`
		Sum     string `json:"sum"`
		Name    string `json:"name"`
	} `json:"dependencies"`
	LegacyImports *bool `json:"legacyImports"`
}

var sourceKey = regexp.MustCompile(`"source"\s*:`)

// readJsonnetfile reads a jsonnetfile.json or jsonnetfile.lock.json: its
// dependencies (each with the line of its "source") and whether legacy
// import names are linked (the default).
//
// Implements: REQ-JSONNET-005
func readJsonnetfile(src []byte) (deps []*dep, legacy bool) {
	var f jbFile
	if json.Unmarshal(src, &f) != nil {
		return nil, true
	}
	var lines []int
	line, prev := 1, 0
	for _, loc := range sourceKey.FindAllIndex(src, -1) {
		line += strings.Count(string(src[prev:loc[0]]), "\n")
		prev = loc[0]
		lines = append(lines, line)
	}
	for i, d := range f.Dependencies {
		out := &dep{version: d.Version, sum: d.Sum, name: d.Name, line: 1}
		if len(lines) == len(f.Dependencies) {
			out.line = lines[i]
		}
		switch {
		case d.Source.Git != nil && d.Source.Git.Remote != "":
			out.remote, out.subdir = d.Source.Git.Remote, d.Source.Git.Subdir
		case d.Source.Local != nil && d.Source.Local.Directory != "":
			out.dir = d.Source.Local.Directory
		default:
			continue
		}
		deps = append(deps, out)
	}
	return deps, f.LegacyImports == nil || *f.LegacyImports
}

// tagLike tells a tag (v1.2.3, 2.0) from a branch name (main, release-0.9).
var tagLike = regexp.MustCompile(`^v?\d+(\.\d+)*([-+.][0-9A-Za-z.-]+)?$`)
