package findings

import (
	"context"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Options says where to look for findings.
type Options struct {
	// Root is the repository; report paths are made relative to it.
	Root string
	// Reports are files or globs holding what a scanner already wrote. A pattern that
	// matches nothing is reported and does not stop the rest.
	Reports []string
	// Packages are the pinned external dependencies to ask the vulnerability database
	// about; empty asks nothing.
	Packages []Package
	// OSV is the database client; nil keeps the run offline.
	OSV *OSV
	// Docs are the repository-relative Markdown files whose links to follow. The
	// check needs nothing but the repository, so it runs whether or not anything
	// else was asked for; empty checks nothing.
	Docs []string
	// Web additionally asks whether the http(s) links those documents carry still
	// answer. nil keeps the link check offline, where it is exact.
	Web  *Web
	Logf func(string, ...any)
}

// Collect reads every report and, when a database client is given, asks it about the
// pinned packages. A report that will not parse is not fatal: the rest of the map is
// still worth having, and the set says it is incomplete.
//
// Implements: REQ-FND-001, REQ-FND-016, REQ-FND-026
func Collect(ctx context.Context, o Options) *Set {
	set := &Set{}
	logFormat := o.Logf
	if logFormat == nil {
		logFormat = func(string, ...any) {}
	}

	for _, name := range expand(o.Root, o.Reports) {
		f, err := reportFiles(o.Root, name).Open(name)
		if err != nil {
			logFormat("findings: %v", err)
			set.Partial = true
			continue
		}
		source, found, err := Read(f)
		f.Close()
		if err != nil {
			logFormat("findings: %s: %v", relative(o.Root, name), err)
			set.Partial = true
			continue
		}
		logFormat("findings: %s: %d from %s", relative(o.Root, name), len(found), source)
		set.Add(source, found)
	}

	if len(o.Docs) > 0 {
		found, partial := checkLinks(ctx, o.Root, o.Docs, o.Web, logFormat)
		set.Partial = set.Partial || partial
		if len(found) > 0 {
			logFormat("findings: %d broken links in %d documents", len(found), len(o.Docs))
		}
		set.Add("links", found)
	}

	if o.OSV != nil && len(o.Packages) > 0 {
		if o.OSV.Logf == nil {
			o.OSV.Logf = logFormat
		}
		found, partial := o.OSV.Query(ctx, o.Packages)
		set.Partial = set.Partial || partial
		if len(found) > 0 {
			logFormat("findings: %d from the OSV database", len(found))
		}
		// What a package's commit matched is reported under its own source, so the
		// panel says which question found it.
		byName, byCommit := []*Finding{}, []*Finding{}
		for _, f := range found {
			if f.Source == SourceCommit {
				byCommit = append(byCommit, f)
			} else {
				byName = append(byName, f)
			}
		}
		set.Add("osv", byName)
		if slices.ContainsFunc(o.Packages, func(p Package) bool { return p.Commit != "" }) {
			set.Add(SourceCommit, byCommit)
		}
	}

	set.Localize(o.Root)
	set.Finish()
	return set
}

// Files is the reports a run would read, which is what a watcher has to watch: a
// report rewritten by a scanner is a finding fixed, or a new one, and the map should
// say so without being restarted.
func Files(root string, patterns []string) []string { return expand(root, patterns) }

// Watched says what to watch for the scanner reports, so that rewriting one is a
// change the watcher sees: named reports as single files, and the directory of every
// pattern that is a glob.
//
// A report usually sits in a directory holding a great deal besides - often the
// repository root - and a watch on the directory fires for every file written into
// it. A glob has no one file to watch, and a new file matching it is news, so there
// the whole directory stays watched.
//
// Implements: REQ-FND-023
func Watched(root string, patterns []string) (directories, files []string) {
	globbed := map[string]bool{}
	for _, p := range patterns {
		if !strings.ContainsAny(p, "*?[") {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		d := filepath.Dir(p)
		if !globbed[d] {
			globbed[d] = true
			directories = append(directories, d)
		}
	}
	for _, f := range Files(root, patterns) {
		if !globbed[filepath.Dir(f)] {
			files = append(files, f)
		}
	}
	return directories, files
}

// expand turns the configured patterns into file names, in a stable order. A pattern
// is relative to the repository unless it is absolute.
//
// Implements: REQ-FND-001
func expand(root string, patterns []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		matches, err := filepath.Glob(p)
		if err != nil || len(matches) == 0 {
			// Not a pattern, or one that matched nothing: let the open report why.
			matches = []string{p}
		}
		for _, m := range matches {
			if fileInfo, err := reportFiles(root, m).Stat(m); err == nil && fileInfo.IsDir() {
				continue
			}
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	sort.Strings(out)
	return out
}

// reportFiles reads a report: one in the repository, as every report a
// repository's own configuration names is (config confines them), through the
// repository's Root, which a link out of it does not pass; one elsewhere, which
// only this machine's configuration or the command line can name, as it is.
func reportFiles(root, name string) lang.Root {
	if repository := lang.OpenRoot(root); repository.Contains(name) {
		return repository
	}
	return lang.Machine
}

func relative(root, name string) string {
	if r, err := filepath.Rel(root, name); err == nil {
		return filepath.ToSlash(r)
	}
	return name
}
