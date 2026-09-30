package index

// Julia registries: the ones installed in this machine's depots, read from the
// copy there - a git checkout, or the archive a Pkg server served - and a
// registry's files over HTTP when there is no copy (General's on GitHub).

import (
	"archive/tar"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/juliapkg"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// juliaGeneral is the General registry's UUID.
const juliaGeneral = "23338594-aafe-5451-b93e-139f81909106"

// juliaRegistry is what a Registry.toml says: the registry's name, UUID and
// repository, and each package's name and directory, keyed by UUID.
type juliaRegistry struct {
	Name       string `toml:"name"`
	UUID       string `toml:"uuid"`
	Repository string `toml:"repo"`
	Packages   map[string]struct {
		Name string `toml:"name"`
		Path string `toml:"path"`
	} `toml:"packages"`
}

// general reports whether a registry is General.
func (r juliaRegistry) general() bool {
	return strings.EqualFold(r.UUID, juliaGeneral) || r.UUID == "" && r.Name == "General"
}

// machineJulia reads the registries installed in this machine's Julia depots
// (userconf.JuliaDepots), each depot's in name order: registries/<Name>/ holding
// a Registry.toml (a git checkout), or registries/<Name>.toml naming the archive
// beside it (`path`) that a Pkg server served (Julia 1.7 and later). General is
// the public index, read from its copy; every other registry serves the packages
// its Registry.toml lists, each a scoped source with its UUID and the copy, at
// raw.githubusercontent.com when the registry's repository is on GitHub and at
// its repository's URL otherwise (never fetched: only the copy is read). With
// registries installed and none of them General, Pkg asks General about nothing,
// and neither does the client.
//
// Implements: REQ-SUP-055, REQ-SUP-064
func machineJulia(m userconf.Machine, k sink) {
	found, general := false, false
	for _, depot := range m.JuliaDepots() {
		directory := filepath.Join(depot, "registries")
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, e := range entries {
			local, registry, ok := installedJuliaRegistry(directory, e)
			if !ok {
				continue
			}
			found = true
			if registry.general() {
				general = true
				k.put(Julia, Source{URL: publicIndex(Julia), Local: local})
				continue
			}
			base := juliaRegistryURL(registry.Repository, local)
			type listed struct{ name, uuid string }
			var packages []listed
			for uuid, p := range registry.Packages {
				if p.Name != "" {
					packages = append(packages, listed{p.Name, strings.ToLower(uuid)})
				}
			}
			sort.Slice(packages, func(i, j int) bool {
				return packages[i].name < packages[j].name ||
					packages[i].name == packages[j].name && packages[i].uuid < packages[j].uuid
			})
			for _, p := range packages {
				k.put(Julia, Source{URL: base, Scope: p.name, Local: local, uuid: p.uuid})
			}
		}
	}
	if found && !general {
		k.off(Julia)
	}
}

// installedJuliaRegistry reads a depot's registries/ entry: the copy it is (a
// directory or an archive) and what its Registry.toml says. General's packages
// are not read: nothing needs them before a question.
func installedJuliaRegistry(directory string, e os.DirEntry) (local string, registry juliaRegistry, ok bool) {
	if e.IsDir() {
		local = filepath.Join(directory, e.Name())
		data, err := os.ReadFile(filepath.Join(local, "Registry.toml"))
		if err != nil {
			return "", registry, false
		}
		registry, ok = decodeJuliaRegistry(data)
		return local, registry, ok
	}
	if !strings.HasSuffix(e.Name(), ".toml") {
		return "", registry, false
	}
	var info struct {
		UUID string `toml:"uuid"`
		Path string `toml:"path"`
	}
	if _, err := toml.DecodeFile(filepath.Join(directory, e.Name()), &info); err != nil || info.Path == "" {
		return "", registry, false
	}
	local = filepath.Join(directory, filepath.FromSlash(info.Path))
	if fileInfo, err := os.Stat(local); err != nil || !fileInfo.Mode().IsRegular() {
		return "", registry, false
	}
	if strings.EqualFold(info.UUID, juliaGeneral) {
		return local, juliaRegistry{Name: "General", UUID: info.UUID}, true
	}
	files, err := juliaTar(local, func(name string) bool { return name == "Registry.toml" }, true)
	if err != nil || files["Registry.toml"] == nil {
		return "", registry, false
	}
	registry, ok = decodeJuliaRegistry(files["Registry.toml"])
	return local, registry, ok
}

// decodeJuliaRegistry decodes a Registry.toml; General's only as far as its
// [packages] table, which is most of it.
func decodeJuliaRegistry(data []byte) (juliaRegistry, bool) {
	var registry juliaRegistry
	head := data
	if i := bytes.Index(data, []byte("\n[packages]")); i >= 0 {
		head = data[:i]
	}
	if _, err := toml.Decode(string(head), &registry); err != nil {
		return registry, false
	}
	if registry.general() {
		return registry, true
	}
	_, err := toml.Decode(string(data), &registry)
	return registry, err == nil
}

// scpLike is git's scp-like repository address, user@host:path.
var scpLike = regexp.MustCompile(`^(?:[^@/:]+@)?([^/:]+):([^/].*)$`)

// juliaRegistryURL is the index a registry other than General stands for: its
// files at raw.githubusercontent.com when its repository is on GitHub; else the
// repository's URL, without the user an ssh address logs in as; else the copy's
// path.
func juliaRegistryURL(repository, local string) string {
	repository = strings.TrimSpace(repository)
	if m := scpLike.FindStringSubmatch(repository); m != nil && !strings.Contains(repository, "://") {
		repository = "ssh://" + m[1] + "/" + m[2]
	}
	u, err := url.Parse(repository)
	if err != nil || repository == "" {
		return local
	}
	if strings.EqualFold(u.Hostname(), "github.com") {
		if name := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git"); strings.Count(name, "/") == 1 {
			return "https://raw.githubusercontent.com/" + name + "/HEAD"
		}
	}
	return sshUserless(repository)
}

// juliaScoped lists the registries installed here that serve a Julia package, in
// the order found: those that list its name under its UUID (lang.Target.Registry),
// or under any UUID when the target has none. A package no such registry lists is
// General's (see candidates): another registry's package of the same name is
// another package.
//
// Implements: REQ-SUP-055
func (c *Config) juliaScoped(packageName, uuid string) []candidate {
	var out []candidate
	for _, s := range c.sources[Julia] {
		if s.Scope != packageName || s.Registry != "" || uuid != "" && s.uuid != "" && !strings.EqualFold(s.uuid, uuid) {
			continue
		}
		out = append(out, candidate{url: s.URL, primary: len(out) == 0, known: c.fetchable(Julia, s)})
	}
	return out
}

// juliaPackage reads a package's dependencies from a Julia registry's files - from
// its copy in a depot when there is one, else over HTTP:
// <registry>/<dir>/Versions.toml for the version (the one pinned, else the newest a
// [compat] range admits, else the newest not yanked), then Deps.toml and Compat.toml,
// whose sections are keyed by the version ranges they hold for ("0.21.2 - 0"). A
// dependency's version is its compat range for that release, in the registry's
// notation, where a whole "1.2.3" is that release, and its UUID is what Deps.toml
// says; julia itself is not a dependency, and Julia's standard libraries are
// julia-std.
//
// Implements: REQ-SUP-055
func (c *Client) juliaPackage(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	base := strings.TrimRight(index, "/")
	read, local, err := c.juliaFiles(ctx, base)
	if err != nil {
		return nil, err
	}
	directory, err := c.juliaDirectory(base, local, read, t)
	if err != nil {
		return nil, err
	}
	body, err := read(directory + "/Versions.toml")
	if err != nil {
		return nil, err
	}
	var versions map[string]struct {
		Yanked bool `toml:"yanked"`
	}
	if _, err := toml.Decode(string(body), &versions); err != nil {
		return nil, err
	}
	var listed []string
	for v, metadata := range versions {
		if !metadata.Yanked {
			listed = append(listed, v)
		}
	}
	version := strings.TrimSpace(t.Version)
	if _, ok := versions[version]; !ok {
		ranges, _ := juliapkg.CompatRanges(version)
		if version = juliapkg.Newest(listed, ranges); version == "" {
			version = juliapkg.Newest(listed, nil)
		}
	}
	v, ok := juliapkg.ParseVersion(version)
	if !ok {
		return nil, nil
	}
	body, err = read(directory + "/Deps.toml")
	if err != nil {
		return nil, nil // a package without dependencies has no Deps.toml
	}
	dependencies := juliaSections(body, v)
	compat := map[string]string{}
	if body, err := read(directory + "/Compat.toml"); err == nil {
		for name, value := range juliaSections(body, v) {
			compat[name] = juliaCompat(value)
		}
	}
	var out []dependency
	for name, uuid := range dependencies {
		if name == "julia" {
			continue
		}
		d := dependency{Name: name, Version: compat[name]}
		if s, ok := uuid.(string); ok {
			d.Registry = strings.ToLower(s)
		}
		if juliapkg.Stdlib(name) {
			d = dependency{Name: name, Ecosystem: "julia-std"}
		}
		out = append(out, d)
	}
	sortDependencies(out)
	return out, nil
}

// juliaFiles is how a registry's files are read, by their path in the registry:
// from the copy in a depot of this machine when there is one (local names it),
// else over HTTP. A registry that is not served over HTTP is read only from a
// copy.
func (c *Client) juliaFiles(ctx context.Context, base string) (read func(string) ([]byte, error), local string, err error) {
	if local = c.config.localCopy(Julia, base); local != "" {
		localCopy, err := c.juliaCopies.get(local, func() (*juliaCopy, error) { return readJuliaCopy(local) })
		if err != nil {
			return nil, "", err
		}
		return localCopy.read, local, nil
	}
	if !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://") {
		// Implements: REQ-TRC-017
		c.note(trace.NoteNoCopy, "Julia registry "+base+" is read only from its copy in a depot on this machine "+
			"(registries/<Name>), and there is none: its packages are asked of the next registry")
		return nil, "", fmt.Errorf("%w: %s has no copy on this machine", errAbsent, base)
	}
	return func(p string) ([]byte, error) { return c.accept(ctx, base+"/"+p, "text/plain") }, "", nil
}

// juliaDirectory is where a registry keeps a package's files. The General registry's
// layout is known (J/JSON) when it is read over HTTP; a copy's Registry.toml, and
// another registry's, list each package's path, read once per registry, by UUID
// when the target has one. A registry that does not list the package does not
// have it.
func (c *Client) juliaDirectory(base, local string, read func(string) ([]byte, error), t lang.Target) (string, error) {
	if base == c.config.publicURL(Julia) && local == "" {
		return juliapkg.RegistryDirectory(t.Package), nil
	}
	paths, err := c.juliaRegistries.get(cmp.Or(local, base), func() (*juliaPaths, error) {
		body, err := read("Registry.toml")
		if err != nil {
			return nil, err
		}
		return parseJuliaPaths(body), nil
	})
	if err != nil {
		return "", err
	}
	if directory := paths.directory(t.Package, t.Registry); directory != "" {
		return directory, nil
	}
	return "", fmt.Errorf("%w: %s does not list %s", errAbsent, base, t.Package)
}

// juliaPaths is where a registry keeps each package's files, by UUID and by
// name.
type juliaPaths struct{ byUUID, byName map[string]string }

// parseJuliaPaths reads a Registry.toml's packages. A path that climbs out of
// the registry is left out.
func parseJuliaPaths(body []byte) *juliaPaths {
	p := &juliaPaths{byUUID: map[string]string{}, byName: map[string]string{}}
	var registry juliaRegistry
	if _, err := toml.Decode(string(body), &registry); err != nil {
		return p
	}
	for uuid, e := range registry.Packages {
		directory := path.Clean(filepath.ToSlash(e.Path))
		if e.Path == "" || path.IsAbs(directory) || directory == ".." || strings.HasPrefix(directory, "../") || strings.Contains(directory, ":") {
			continue
		}
		p.byUUID[strings.ToLower(uuid)] = directory
		p.byName[e.Name] = directory
	}
	return p
}

// directory is a package's directory: the one of its UUID when it has one.
func (p *juliaPaths) directory(name, uuid string) string {
	if uuid != "" {
		return p.byUUID[strings.ToLower(uuid)]
	}
	return p.byName[name]
}

// juliaCopy is a registry installed in a depot: a directory, or an archive
// whose registry files are read once into memory.
type juliaCopy struct {
	directory string
	files     map[string][]byte
}

// juliaRegistryFile reports whether a registry's file is one the client reads.
func juliaRegistryFile(name string) bool {
	switch path.Base(name) {
	case "Registry.toml", "Versions.toml", "Deps.toml", "Compat.toml":
		return true
	}
	return false
}

func readJuliaCopy(local string) (*juliaCopy, error) {
	fileInfo, err := os.Stat(local)
	if err != nil {
		return nil, err
	}
	if fileInfo.IsDir() {
		return &juliaCopy{directory: local}, nil
	}
	files, err := juliaTar(local, juliaRegistryFile, false)
	if err != nil {
		return nil, err
	}
	return &juliaCopy{files: files}, nil
}

// read is a file of the registry, by its path there.
func (localCopy *juliaCopy) read(name string) ([]byte, error) {
	if localCopy.files != nil {
		body, ok := localCopy.files[name]
		if !ok {
			return nil, fmt.Errorf("%w: no %s in the registry", errAbsent, name)
		}
		return body, nil
	}
	body, err := os.ReadFile(filepath.Join(localCopy.directory, filepath.FromSlash(name)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: no %s in the registry", errAbsent, name)
	}
	return body, err
}

// juliaTar reads the regular files of a gzipped tar archive whose names keep
// takes, by their slash path without a leading "./"; with first set, it stops
// after the first one.
func juliaTar(archive string, keep func(string) bool, first bool) (map[string][]byte, error) {
	f, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	tarReader := tar.NewReader(z)
	for {
		h, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(path.Clean("/"+h.Name), "/")
		if h.Typeflag != tar.TypeReg || !keep(name) {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(tarReader, maxBody))
		if err != nil {
			return nil, err
		}
		files[name] = body
		if first {
			return files, nil
		}
	}
}

// juliaSections merges the sections of a Deps.toml or Compat.toml whose range key
// holds for a version.
func juliaSections(body []byte, v juliapkg.Version) map[string]any {
	var doc map[string]map[string]any
	if _, err := toml.Decode(string(body), &doc); err != nil {
		return nil
	}
	out := map[string]any{}
	for key, section := range doc {
		if r, ok := juliapkg.RegistryRange(key); ok && r.Contains(v) {
			for name, value := range section {
				out[name] = value
			}
		}
	}
	return out
}

// juliaCompat writes a Compat.toml value - a range or a list of them, "0.1-0.3"
// compressed - as one string, hyphen ranges spaced so that no range reads as a
// pre-release: ["0.1-0.3", "1"] is "0.1 - 0.3, 1".
func juliaCompat(value any) string {
	var parts []string
	switch v := value.(type) {
	case string:
		parts = []string{v}
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				parts = append(parts, s)
			}
		}
	}
	for i, p := range parts {
		if low, high, ok := strings.Cut(p, "-"); ok && !strings.Contains(p, " - ") {
			parts[i] = strings.TrimSpace(low) + " - " + strings.TrimSpace(high)
		}
	}
	return strings.Join(parts, ", ")
}
