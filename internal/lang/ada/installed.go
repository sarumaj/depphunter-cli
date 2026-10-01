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

// crateDirectoryRe splits Alire's <crate>_<version>_<hash> directory names.
var crateDirectoryRe = regexp.MustCompile(`^([a-z0-9_]+?)_(\d[^_]*)_([0-9a-f]{6,})$`)

// readInstalled reads what Alire fetched beside the manifest in absoluteDirectory: each
// crate's alire.toml and the units and project files its sources hold.
//
// Implements: REQ-ADA-008
func (c *crateDirectory) readInstalled(repository lang.Root, absoluteDirectory string) {
	c.installed, c.units, c.projects = map[string]*installedCrate{}, map[string]string{}, map[string]string{}
	for _, section := range []string{"dependencies", "pins"} {
		base := filepath.Join(absoluteDirectory, "alire", "cache", section)
		entries, err := repository.ReadDir(base)
		if err != nil {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name, version := "", ""
			if m := crateDirectoryRe.FindStringSubmatch(e.Name()); m != nil {
				name, version = m[1], m[2]
			}
			directory := filepath.Join(base, e.Name())
			if source, ok := repository.ReadBounded(filepath.Join(directory, "alire.toml")); ok {
				if n := readManifest(source).name; n != "" {
					name = n
				}
			}
			if name == "" {
				name = strings.ToLower(e.Name())
			}
			if _, ok := c.installed[name]; ok {
				continue
			}
			c.add(repository, directory, name, version)
		}
	}
}

// sharedReleases is where Alire 2 keeps the sources of the releases it fetched
// once for every workspace ("shared" dependencies), and the Root that reads it:
// releases/ in its cache, which is cache/ below ALIRE_SETTINGS_DIR when that is
// set, else $XDG_DATA_HOME/alire, else ~/.local/share/alire (%LOCALAPPDATA%\alire
// on Windows). Relative values are taken against the repository root.
func sharedReleases(root string, getenv func(string) string, goos string) (string, lang.Root) {
	if settings, files := lang.FromEnvironment(root, getenv("ALIRE_SETTINGS_DIR")); settings != "" {
		return filepath.Join(settings, "cache", "releases"), files
	}
	variable, home := "XDG_DATA_HOME", filepath.Join(getenv("HOME"), ".local", "share")
	if goos == "windows" {
		variable, home = "LOCALAPPDATA", filepath.Join(getenv("USERPROFILE"), "AppData", "Local")
	}
	data, files := lang.FromEnvironment(root, getenv(variable))
	if data == "" {
		if !filepath.IsAbs(home) { // no home directory
			return "", lang.Root{}
		}
		data, files = home, lang.Machine
	}
	return filepath.Join(data, "alire", "releases"), files
}

// readShared adds the crates c's manifest or lock file names that Alire keeps
// in a shared releases directory and c's alire/cache/ lacks: the release of
// the locked version, else of an exact constraint, else the newest there.
// Crates nothing names are not read, so one workspace's crates do not leak
// into another's.
//
// Implements: REQ-ADA-008
func (c *crateDirectory) readShared(base string, files lang.Root) {
	type release struct{ directory, version string }
	found := map[string][]release{}
	entries, _ := files.ReadDir(base)
	for _, e := range entries {
		m := crateDirectoryRe.FindStringSubmatch(e.Name())
		if m == nil || !e.IsDir() {
			continue
		}
		found[m[1]] = append(found[m[1]], release{filepath.Join(base, e.Name()), m[2]})
	}
	for _, name := range c.known() {
		releases := found[name]
		if _, ok := c.installed[name]; ok || len(releases) == 0 {
			continue
		}
		want := ""
		if state := c.lock[name]; state != nil {
			want = state.version
		} else if d := c.m.dependencies[name]; d != nil {
			want, _ = exactVersion(d.constraint)
		}
		best := -1
		for i, relative := range releases {
			switch {
			case want != "" && relative.version == want:
				best = i
			case want == "" && (best < 0 || versionLess(releases[best].version, relative.version)):
				best = i
			}
		}
		if best < 0 {
			continue
		}
		c.add(files, releases[best].directory, name, releases[best].version)
	}
}

// add reads an installed crate's directory through files (the repository's Root
// for its alire/cache, sharedReleases' for the shared releases): its manifest, project
// files and units.
func (c *crateDirectory) add(files lang.Root, directory, name, version string) {
	installed := &installedCrate{name: name, version: version, m: &manifest{dependencies: map[string]*dependency{}, pins: map[string]*pin{}}}
	if source, ok := files.ReadBounded(filepath.Join(directory, "alire.toml")); ok {
		installed.m = readManifest(source)
		if installed.m.version != "" {
			installed.version = installed.m.version
		}
	}
	c.installed[name] = installed
	for _, projectFile := range installed.m.projectFiles {
		c.addProject(projectFile.s, name)
	}
	c.walk(files, directory, name)
}

func (c *crateDirectory) addProject(file, crate string) {
	name := lower(strings.TrimSuffix(filepath.Base(filepath.FromSlash(file)), filepath.Ext(file)))
	if _, ok := c.projects[name]; !ok {
		c.projects[name] = crate
	}
}

// walk indexes the units and project files of an installed crate's directory
// (at most 5000 files; nested alire/ and obj/ directories are skipped).
func (c *crateDirectory) walk(files lang.Root, directory, crate string) {
	n := 0
	files.WalkDir(directory, func(q string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if q != directory && (d.Name() == "alire" || d.Name() == "obj" || strings.HasPrefix(d.Name(), ".")) {
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
		// Measured once open: a symbolic link's own size (d.Info) says
		// nothing of its target's.
		if source, ok := files.ReadBounded(q); ok {
			for _, u := range units(source, strings.EqualFold(filepath.Ext(q), ".ada")) {
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
	aNumbers, bNumbers := strings.FieldsFunc(a, notDigit), strings.FieldsFunc(b, notDigit)
	for i := 0; i < len(aNumbers) && i < len(bNumbers); i++ {
		if len(aNumbers[i]) != len(bNumbers[i]) {
			return len(aNumbers[i]) < len(bNumbers[i])
		}
		if aNumbers[i] != bNumbers[i] {
			return aNumbers[i] < bNumbers[i]
		}
	}
	return len(aNumbers) < len(bNumbers)
}

func notDigit(r rune) bool { return r < '0' || r > '9' }
