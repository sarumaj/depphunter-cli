package jsonnet

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one dependency of a jsonnetfile.json or jsonnetfile.lock.json.
type dependency struct {
	remote       string // git remote, as written
	subdirectory string
	directory    string // a local source's directory, relative to the jsonnetfile
	version      string
	sum          string
	name         string // the "name" field: the legacy import name when set
	line         int
}

func (d *dependency) local() bool { return d.remote == "" }

// packageName is the package name: the repository (lang.RepositoryName) and subdirectory, which is
// also where jsonnet-bundler installs it under vendor/ and the path its
// full-path imports start with.
func (d *dependency) packageName() string {
	if d.local() {
		return d.directory
	}
	p := lang.RepositoryName(d.remote)
	if subdirectory := strings.Trim(path.Clean("/"+d.subdirectory), "/"); subdirectory != "" {
		p += "/" + subdirectory
	}
	return p
}

// legacy is the short name jsonnet-bundler links vendor/<name> to (legacy
// imports): the name field, else the subdir's last element, else the
// repository's (a local source: its directory's).
func (d *dependency) legacy() string {
	if d.name != "" {
		return d.name
	}
	if d.local() {
		return path.Base(path.Clean(d.directory))
	}
	if subdirectory := strings.Trim(path.Clean("/"+d.subdirectory), "/"); subdirectory != "" {
		return path.Base(subdirectory)
	}
	return path.Base(lang.RepositoryName(d.remote))
}

type jbFile struct {
	Dependencies []struct {
		Source struct {
			Git *struct {
				Remote       string `json:"remote"`
				Subdirectory string `json:"subdir"`
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
func readJsonnetfile(source []byte) (dependencies []*dependency, legacy bool) {
	var f jbFile
	if json.Unmarshal(source, &f) != nil {
		return nil, true
	}
	var lines []int
	line, previous := 1, 0
	for _, span := range sourceKey.FindAllIndex(source, -1) {
		line += strings.Count(string(source[previous:span[0]]), "\n")
		previous = span[0]
		lines = append(lines, line)
	}
	for i, d := range f.Dependencies {
		out := &dependency{version: d.Version, sum: d.Sum, name: d.Name, line: 1}
		if len(lines) == len(f.Dependencies) {
			out.line = lines[i]
		}
		switch {
		case d.Source.Git != nil && d.Source.Git.Remote != "":
			out.remote, out.subdirectory = d.Source.Git.Remote, d.Source.Git.Subdirectory
		case d.Source.Local != nil && d.Source.Local.Directory != "":
			out.directory = d.Source.Local.Directory
		default:
			continue
		}
		dependencies = append(dependencies, out)
	}
	return dependencies, f.LegacyImports == nil || *f.LegacyImports
}

// tagLike tells a tag (v1.2.3, 2.0) from a branch name (main, release-0.9).
var tagLike = regexp.MustCompile(`^v?\d+(\.\d+)*([-+.][0-9A-Za-z.-]+)?$`)
