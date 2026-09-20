package findings

import (
	"context"
	"os"
	"path/filepath"
	"sort"
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
	OSV  *OSV
	Logf func(string, ...any)
}

// Collect reads every report and, when a database client is given, asks it about the
// pinned packages. A report that will not parse is not fatal: the rest of the map is
// still worth having, and the set says it is incomplete.
func Collect(ctx context.Context, o Options) *Set {
	set := &Set{}
	logFormat := o.Logf
	if logFormat == nil {
		logFormat = func(string, ...any) {}
	}

	for _, name := range expand(o.Root, o.Reports) {
		f, err := os.Open(name)
		if err != nil {
			logFormat("findings: %v", err)
			set.Partial = true
			continue
		}
		source, found, err := Read(f)
		f.Close()
		if err != nil {
			logFormat("findings: %s: %v", rel(o.Root, name), err)
			set.Partial = true
			continue
		}
		logFormat("findings: %s: %d from %s", rel(o.Root, name), len(found), source)
		set.Add(source, found)
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
		set.Add("osv", found)
	}

	set.Localize(o.Root)
	set.Finish()
	return set
}

// Files is the reports a run would read, which is what a watcher has to watch: a
// report rewritten by a scanner is a finding fixed, or a new one, and the map should
// say so without being restarted.
func Files(root string, patterns []string) []string { return expand(root, patterns) }

// expand turns the configured patterns into file names, in a stable order. A pattern
// is relative to the repository unless it is absolute.
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
			if st, err := os.Stat(m); err == nil && st.IsDir() {
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

func rel(root, name string) string {
	if r, err := filepath.Rel(root, name); err == nil {
		return filepath.ToSlash(r)
	}
	return name
}
