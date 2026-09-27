package ada

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// installedCrate is a crate Alire fetched into alire/cache/dependencies/ (or a
// git pin it cloned into alire/cache/pins/): its own manifest.
type installedCrate struct {
	name, version string
	m             *manifest
}

// crateDirRe splits Alire's <crate>_<version>_<hash> directory names.
var crateDirRe = regexp.MustCompile(`^([a-z0-9_]+?)_(\d[^_]*)_([0-9a-f]{6,})$`)

// readInstalled reads what Alire fetched beside the manifest in absDir: each
// crate's alire.toml and the units and project files its sources hold.
//
// Implements: REQ-ADA-008
func (c *crateDir) readInstalled(absDir string) {
	c.installed, c.units, c.projects = map[string]*installedCrate{}, map[string]string{}, map[string]string{}
	for _, sub := range []string{"dependencies", "pins"} {
		base := filepath.Join(absDir, "alire", "cache", sub)
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name, version := "", ""
			if m := crateDirRe.FindStringSubmatch(e.Name()); m != nil {
				name, version = m[1], m[2]
			}
			dir := filepath.Join(base, e.Name())
			if src, err := os.ReadFile(filepath.Join(dir, "alire.toml")); err == nil {
				if n := readManifest(src).name; n != "" {
					name = n
				}
			}
			if name == "" {
				name = strings.ToLower(e.Name())
			}
			if _, ok := c.installed[name]; ok {
				continue
			}
			c.add(dir, name, version)
		}
	}
}

// sharedReleases are the directories where Alire 2 keeps the sources of the
// releases it fetched once for every workspace ("shared" dependencies): the
// cache below ALIRE_SETTINGS_DIR, else $XDG_CACHE_HOME/alire, else
// ~/.cache/alire.
func sharedReleases(getenv func(string) string) []string {
	var out []string
	if d := getenv("ALIRE_SETTINGS_DIR"); d != "" {
		out = append(out, filepath.Join(d, "cache", "releases"))
	}
	if d := getenv("XDG_CACHE_HOME"); d != "" {
		out = append(out, filepath.Join(d, "alire", "releases"))
	} else if h := getenv("HOME"); h != "" {
		out = append(out, filepath.Join(h, ".cache", "alire", "releases"))
	}
	return out
}

// readShared adds the crates c's manifest or lock file names that Alire keeps
// in a shared releases directory and c's alire/cache/ lacks: the release of
// the locked version, else of an exact constraint, else the newest there.
// Crates nothing names are not read, so one workspace's crates do not leak
// into another's.
//
// Implements: REQ-ADA-008
func (c *crateDir) readShared(dirs []string) {
	type release struct{ dir, version string }
	found := map[string][]release{}
	for _, base := range dirs {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			m := crateDirRe.FindStringSubmatch(e.Name())
			if m == nil || !e.IsDir() {
				continue
			}
			found[m[1]] = append(found[m[1]], release{filepath.Join(base, e.Name()), m[2]})
		}
	}
	for _, name := range c.known() {
		releases := found[name]
		if _, ok := c.installed[name]; ok || len(releases) == 0 {
			continue
		}
		want := ""
		if st := c.lock[name]; st != nil {
			want = st.version
		} else if d := c.m.deps[name]; d != nil {
			want, _ = exactVersion(d.constraint)
		}
		best := -1
		for i, rel := range releases {
			switch {
			case want != "" && rel.version == want:
				best = i
			case want == "" && (best < 0 || versionLess(releases[best].version, rel.version)):
				best = i
			}
		}
		if best < 0 {
			continue
		}
		c.add(releases[best].dir, name, releases[best].version)
	}
}

// add reads an installed crate's directory: its manifest, project files and
// units.
func (c *crateDir) add(dir, name, version string) {
	ic := &installedCrate{name: name, version: version, m: &manifest{deps: map[string]*dependency{}, pins: map[string]*pin{}}}
	if src, err := os.ReadFile(filepath.Join(dir, "alire.toml")); err == nil {
		ic.m = readManifest(src)
		if ic.m.version != "" {
			ic.version = ic.m.version
		}
	}
	c.installed[name] = ic
	for _, pf := range ic.m.projectFiles {
		c.addProject(pf.s, name)
	}
	c.walk(dir, name)
}

func (c *crateDir) addProject(file, crate string) {
	name := lower(strings.TrimSuffix(filepath.Base(filepath.FromSlash(file)), filepath.Ext(file)))
	if _, ok := c.projects[name]; !ok {
		c.projects[name] = crate
	}
}

// walk indexes the units and project files of an installed crate's directory
// (at most 5000 files; nested alire/ and obj/ directories are skipped).
func (c *crateDir) walk(dir, crate string) {
	n := 0
	filepath.WalkDir(dir, func(q string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if q != dir && (d.Name() == "alire" || d.Name() == "obj" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if gprFile(q) {
			c.addProject(q, crate)
			return nil
		}
		if !source(q) || n >= 5000 {
			return nil
		}
		n++
		if st, err := d.Info(); err != nil || st.Size() > lang.MaxParseSize {
			return nil
		}
		if src, err := os.ReadFile(q); err == nil {
			for _, u := range units(src, strings.EqualFold(filepath.Ext(q), ".ada")) {
				if _, ok := c.units[u.name]; !ok {
					c.units[u.name] = crate
				}
			}
		}
		return nil
	})
}

// versionLess orders versions by their numeric segments.
func versionLess(a, b string) bool {
	pa, pb := strings.FieldsFunc(a, notDigit), strings.FieldsFunc(b, notDigit)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if len(pa[i]) != len(pb[i]) {
			return len(pa[i]) < len(pb[i])
		}
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}

func notDigit(r rune) bool { return r < '0' || r > '9' }
