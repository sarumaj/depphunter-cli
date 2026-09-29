package index

// CPAN mirrors, opam repositories and Alire indexes: where this machine's cpanm,
// Carton, opam and alr install from, the copies of those indexes opam and alr
// keep on disk, and listing a package's versions - from such a copy, or from
// GitHub for the public repositories - so that a range is answered by the
// newest release it admits.

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/kballard/go-shellquote"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/ocaml"
	"github.com/sarumaj/depphunter-cli/internal/lang/opam"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// memo reads something once per key; a key that could not be read is not read
// again until failRetry has passed. The zero value is ready.
type memo[T any] struct {
	mu     sync.Mutex
	got    map[string]T
	failed map[string]qlFailure
}

// known is what was read for key, without reading it.
func (m *memo[T]) known(key string) (T, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.got[key]
	return v, ok
}

func (m *memo[T]) get(key string, read func() (T, error)) (T, error) {
	m.mu.Lock()
	v, ok := m.got[key]
	failed, gave := m.failed[key]
	m.mu.Unlock()
	if ok {
		return v, nil
	}
	if gave && time.Since(failed.at) < failRetry {
		return v, failed.err
	}
	v, err := read()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.got == nil {
		m.got, m.failed = map[string]T{}, map[string]qlFailure{}
	}
	if err != nil {
		m.failed[key] = qlFailure{err, time.Now()}
	} else {
		delete(m.failed, key)
		m.got[key] = v
	}
	return v, err
}

// localCopy is the copy on this machine's disk of an index this machine's
// configuration names, or "".
func (c *Config) localCopy(ecosystem, index string) string {
	for _, s := range c.sources[ecosystem] {
		if s.URL == index && s.Local != "" && s.Trusted {
			return s.Local
		}
	}
	return ""
}

// unlisted reports whether a package's versions cannot be listed on an index
// served over HTTP: it has no copy on disk, and it is not the public one,
// whose repository GitHub lists. An index served otherwise (git) with no copy
// is not read at all, and passes the question on (see opamRepository, alireIndex).
func (c *Client) unlisted(ecosystem, index string) bool {
	return (strings.HasPrefix(index, "https://") || strings.HasPrefix(index, "http://")) &&
		c.config.localCopy(ecosystem, index) == "" && index != c.config.publicURL(ecosystem)
}

// fileURLPath is the local path a file: URL or a plain path names, or "".
func fileURLPath(u string) string {
	switch {
	case strings.HasPrefix(u, "file:"):
		p, err := url.Parse(u)
		if err != nil {
			return ""
		}
		name := p.Path
		if p.Opaque != "" {
			name = p.Opaque // file:relative
		}
		if len(name) > 2 && name[0] == '/' && name[2] == ':' {
			name = name[1:] // file:///C:/x
		}
		return filepath.FromSlash(name)
	case filepath.IsAbs(u) || strings.HasPrefix(u, "/"):
		return u
	}
	return ""
}

// canonical is a repository URL with what does not change where it is -
// `git+`, a `#ref`, `.git`, a trailing slash, the scheme's case - taken off,
// for comparing with the public repositories' addresses.
func canonical(u string) string {
	u = strings.TrimPrefix(strings.TrimSpace(u), "git+")
	if i := strings.IndexByte(u, '#'); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimSuffix(strings.TrimRight(u, "/"), ".git")
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	return strings.ToLower(u)
}

// sshUserless is a repository URL without the user of an ssh URL
// (git+ssh://git@host/...), which names the account ssh logs in as, not a
// credential for the host's web server.
func sshUserless(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.User == nil || !strings.Contains(p.Scheme, "ssh") {
		return u
	}
	p.User = nil
	return p.String()
}

// ---------------------------------------------------------------- CPAN

// machineCPAN reads the CPAN mirrors this machine installs from. cpanm's
// `--mirror` URLs in PERL_CPANM_OPT are indexes only under `--mirror-only`
// (or `--from`, which is both): cpanm then resolves modules from each
// mirror's 02packages in order, and only from them - the public CPAN too only
// when it is one of them. Otherwise a mirror is where archives are downloaded
// from, not an index. PERL_CARTON_MIRROR is the DarkPAN Carton installs from,
// before CPAN itself. A public CPAN mirror stands for MetaCPAN.
//
// Implements: REQ-SUP-053, REQ-SUP-064
func machineCPAN(m userconf.Machine, k sink) {
	if u := cpanMirrorURL(m.Environment("PERL_CARTON_MIRROR")); u != "" && !cpanPublicMirror(u) {
		k.extra(CPAN, u)
	}
	mirrors, only := cpanmMirrors(m.Environment("PERL_CPANM_OPT"))
	if !only || len(mirrors) == 0 {
		return
	}
	for _, u := range mirrors {
		if cpanPublicMirror(u) {
			u = public[CPAN]
		}
		k.put(CPAN, Source{URL: u, Kind: Listed})
	}
	if !slices.ContainsFunc(mirrors, cpanPublicMirror) {
		k.off(CPAN)
	}
}

// cpanmMirrors reads cpanm's options: the mirrors in order, and whether they
// are the only indexes.
func cpanmMirrors(option string) (mirrors []string, only bool) {
	arguments, err := shellquote.Split(option)
	if err != nil {
		arguments = strings.Fields(option)
	}
	for i := 0; i < len(arguments); i++ {
		a := arguments[i]
		name, value, inline := strings.Cut(a, "=")
		switch name {
		case "--mirror-only":
			only = true
			continue
		case "--mirror", "--from", "-M":
		default:
			continue
		}
		if !inline && i+1 < len(arguments) {
			i++
			value = arguments[i]
		}
		if name != "--mirror" {
			only = true
		}
		if u := cpanMirrorURL(value); u != "" {
			mirrors = append(mirrors, u)
		}
	}
	return mirrors, only
}

// cpanMirrorURL is a mirror as cpanm takes one - an http(s) or file: URL, or a
// directory - as a URL, or "".
func cpanMirrorURL(u string) string {
	u = strings.TrimSpace(u)
	switch {
	case strings.HasPrefix(u, "https://"), strings.HasPrefix(u, "http://"), strings.HasPrefix(u, "file:"):
		return strings.TrimRight(u, "/")
	case filepath.IsAbs(u) || strings.HasPrefix(u, "/"):
		p := strings.TrimRight(filepath.ToSlash(u), "/")
		if !strings.HasPrefix(p, "/") {
			p = "/" + p // C:/minicpan
		}
		return "file://" + p
	}
	return ""
}

// cpanPublicMirror reports whether a mirror is the public CPAN.
func cpanPublicMirror(u string) bool {
	switch canonical(u) {
	case "cpan.metacpan.org", "www.cpan.org", "cpan.org", "www.metacpan.org":
		return true
	}
	return false
}

// cpanDistVersion splits "libwww-perl-6.72" into the distribution and its version.
var cpanDistVersion = regexp.MustCompile(`^(.+)-(v?[0-9][0-9._]*(?:-TRIAL)?)$`)

// readCPANSnapshot records which archive a repository's cpanfile.snapshot says
// each distribution was installed from: its `pathname` names the author, whose
// release MetaCPAN is then asked for by name.
//
// Implements: REQ-SUP-053
func (c *Config) readCPANSnapshot(data []byte) {
	dist := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		indent := len(line) - len(strings.TrimLeft(line, " "))
		text := strings.TrimSpace(line)
		switch {
		case indent == 2:
			dist = ""
			if match := cpanDistVersion.FindStringSubmatch(text); match != nil {
				dist = match[1] + " " + match[2]
			}
		case indent == 4 && dist != "" && strings.HasPrefix(text, "pathname:"):
			p := strings.TrimSpace(strings.TrimPrefix(text, "pathname:"))
			if author, _ := cpanArchive(p); author == "" {
				continue
			}
			if c.cpanArchives == nil {
				c.cpanArchives = map[string]string{}
			}
			if _, ok := c.cpanArchives[dist]; !ok {
				c.cpanArchives[dist] = p
			}
		}
	}
}

// cpanArchive reads the path of a release's archive below a CPAN mirror's
// authors/id (A/AU/AUTHOR/Dist-1.23.tar.gz): the author, and the release's name
// (Dist-1.23). The author is "" for a path of another shape.
func cpanArchive(p string) (author, name string) {
	p = strings.TrimPrefix(strings.TrimPrefix(p, "authors/"), "id/")
	parts := strings.Split(p, "/")
	if len(parts) < 4 || len(parts[0]) != 1 || len(parts[1]) != 2 || !strings.HasPrefix(parts[2], parts[1]) {
		return "", ""
	}
	name = parts[len(parts)-1]
	for _, extension := range []string{".tar.gz", ".tgz", ".tar.bz2", ".tbz", ".tar.xz", ".zip", ".tar"} {
		if n, ok := strings.CutSuffix(name, extension); ok {
			name = n
			break
		}
	}
	return parts[2], name
}

// cpanArchive is the archive the repository's snapshot says a distribution's
// version came from, or "".
func (c *Config) cpanArchive(dist, version string) string {
	return c.cpanArchives[dist+" "+version]
}

// cpanPackages is what a mirror's 02packages lists: the release of each
// distribution (the one holding the module named like it, else the first
// listed), and the distribution of each module.
type cpanPackages struct {
	dists   map[string]cpanMirrored
	modules map[string]string
}

type cpanMirrored struct {
	path, author, name, version string
}

// cpanMirror answers from a CPAN mirror - a DarkPAN, Pinto or OrePAN2
// repository, a minicpan - whose modules/02packages.details.txt(.gz) says which
// distribution's archive provides each module: a distribution it does not list
// is not found there. The dependencies are in the archive's META.json, which is
// not downloaded; they are those of the same release on MetaCPAN (by the
// author the mirror's path names; the version asked for, else the mirror's), the
// modules named by the distribution the mirror lists for each. A distribution a
// private pattern covers is not named to MetaCPAN, and one MetaCPAN does not
// describe is answered without dependencies, with a note.
//
// Implements: REQ-SUP-053
func (c *Client) cpanMirror(ctx context.Context, base string, t lang.Target) ([]dependency, error) {
	packages, err := c.cpanMirrors.get(base, func() (*cpanPackages, error) { return c.readCPANPackages(ctx, base) })
	if err != nil {
		return nil, err
	}
	relative, ok := packages.dists[t.Package]
	if !ok {
		return nil, fmt.Errorf("%w: %s lists no %s", errAbsent, base, t.Package)
	}
	name := relative.name
	if v := strings.TrimSpace(t.Version); cpanVersion.MatchString(v) && v != relative.version {
		name = t.Package + "-" + v
	}
	metacpan := c.config.publicURL(CPAN)
	if c.private.Match(CPAN, t.Package) || metacpan == "" {
		return []dependency{}, nil
	}
	found, err := c.cpanRelease(ctx, metacpan+"/v1/release/"+url.PathEscape(relative.author)+"/"+url.PathEscape(name))
	if notFound(err) {
		// Implements: REQ-TRC-017
		c.note(trace.NoteNoRelease, "CPAN mirror "+base+" has "+t.Package+" ("+relative.path+"), a release MetaCPAN does "+
			"not describe: its dependencies are in the archive's META.json, which is not downloaded")
		return []dependency{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []dependency
	seen := map[string]bool{t.Package: true, found.Distribution: true}
	for _, d := range found.Dependency {
		if d.Phase != "runtime" || d.Relationship != "requires" || d.Module == "perl" {
			continue
		}
		dist := packages.modules[d.Module]
		if dist == "" || dist == "perl" || seen[dist] {
			continue
		}
		seen[dist] = true
		out = append(out, dependency{Name: dist, Version: cpanMinimum(d.Version)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// readCPANPackages reads a mirror's 02packages: the gzipped file, else the
// plain one; from the disk for a file: mirror.
func (c *Client) readCPANPackages(ctx context.Context, base string) (*cpanPackages, error) {
	var body []byte
	var err error
	if directory := fileURLPath(base); strings.HasPrefix(base, "file:") && directory != "" {
		name := filepath.Join(directory, "modules", "02packages.details.txt")
		if body, err = os.ReadFile(name + ".gz"); err != nil {
			body, err = os.ReadFile(name)
		} else {
			body, err = gunzip(body)
		}
	} else {
		name := base + "/modules/02packages.details.txt"
		if body, err = c.accept(ctx, name+".gz", "application/gzip"); notFound(err) {
			body, err = c.accept(ctx, name, "text/plain")
		} else if err == nil {
			body, err = gunzip(body)
		}
	}
	if err != nil {
		return nil, err
	}
	return parseCPANPackages(body), nil
}

// gunzip is data uncompressed, or data itself when it is not gzipped (a server
// that decompressed it on the way).
func gunzip(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(r, 256<<20))
}

// parseCPANPackages reads 02packages.details.txt: a header, a blank line, then
// `Module version A/AU/AUTHOR/Dist-1.23.tar.gz` per line.
func parseCPANPackages(data []byte) *cpanPackages {
	p := &cpanPackages{dists: map[string]cpanMirrored{}, modules: map[string]string{}}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	header := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if header {
			header = line != ""
			continue
		}
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		author, name := cpanArchive(f[2])
		match := cpanDistVersion.FindStringSubmatch(name)
		if author == "" || match == nil {
			continue
		}
		dist := match[1]
		p.modules[f[0]] = dist
		if _, ok := p.dists[dist]; !ok || strings.ReplaceAll(dist, "-", "::") == f[0] {
			p.dists[dist] = cpanMirrored{path: f[2], author: author, name: name, version: match[2]}
		}
	}
	return p
}

// cpanMinimum shows a MetaCPAN requirement's version as a minimum: ">= 1.2", or
// "" for none (0).
func cpanMinimum(version any) string {
	v := strings.TrimSpace(fmt.Sprint(version))
	if version == nil || strings.Trim(v, "0.") == "" {
		return ""
	}
	if v[0] >= '0' && v[0] <= '9' || v[0] == 'v' {
		return ">= " + v
	}
	return v
}

// ---------------------------------------------------------------- GitHub

// githubAPI is GitHub's REST API; tests point it at their own server.
var githubAPI = "https://api.github.com"

// githubRepository is a public index's git repository on GitHub, for listing a
// directory of it when there is no copy on disk.
type githubRepository struct{ repository, reference string }

var (
	opamGitHub  = githubRepository{"ocaml/opam-repository", "master"}
	alireGitHub = githubRepository{"alire-project/alire-index", "stable-1.4.0"}
)

// githubList lists a directory of a repository through GitHub's contents API
// (unauthenticated, 60 requests an hour, unless this machine holds a credential
// for api.github.com), once per directory.
func (c *Client) githubList(ctx context.Context, r githubRepository, directory string) ([]string, error) {
	address := githubAPI + "/repos/" + r.repository + "/contents/" + directory + "?ref=" + url.QueryEscape(r.reference)
	return c.listings.get(address, func() ([]string, error) {
		body, err := c.accept(ctx, address, "application/vnd.github+json")
		if err != nil {
			return nil, err
		}
		var entries []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, err
		}
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Name)
		}
		return out, nil
	})
}

// ---------------------------------------------------------------- opam

// opamUpstream and opamOverlays are the repositories dune's package management
// names upstream and overlay unless a dune-workspace defines them.
const (
	opamUpstream = "git+https://github.com/ocaml/opam-repository.git"
	opamOverlays = "git+https://github.com/ocaml-dune/opam-overlays.git"
)

// opamPublic reports whether a repository URL is opam-repository itself.
func opamPublic(u string) bool {
	switch canonical(u) {
	case "opam.ocaml.org", "github.com/ocaml/opam-repository":
		return true
	}
	return false
}

// machineOpam reads the repositories this machine's opam installs from, from
// its root (OPAMROOT, ~/.opam): the URL of each in repo/repos-config, in the
// order of priority the current switch's switch-config gives (OPAMSWITCH, else
// the root config's `switch`), else the root config's. Each is asked in that
// order, opam-repository (opam.ocaml.org) as the public index, which is off when
// it is not among them; each comes with the copy opam keeps of it in
// repo/<name>, a directory or (opam 2.1 and later) repo/<name>.tar.gz.
//
// Implements: REQ-SUP-054, REQ-SUP-064
func machineOpam(m userconf.Machine, k sink) {
	root := m.OpamRoot()
	if root == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(root, "repo", "repos-config"))
	if err != nil {
		return
	}
	urls := map[string]string{}
	var names []string
	for _, r := range opam.Repositories(data) {
		if r.URL != "" {
			urls[r.Name] = r.URL
			names = append(names, r.Name)
		}
	}
	if order := opamOrder(m, root); len(order) > 0 {
		names = order
	}
	added, public := false, false
	for _, name := range names {
		u := urls[name]
		if u == "" {
			continue
		}
		if opamPublic(u) {
			u, public = publicIndex(Opam), true
		}
		k.put(Opam, Source{URL: sshUserless(u), Kind: Listed, Local: opamCopyPath(root, name)})
		added = true
	}
	if added && !public {
		k.off(Opam)
	}
}

// publicIndex is an ecosystem's public default.
func publicIndex(ecosystem string) string { return public[ecosystem] }

// opamOrder is the repositories of the current switch in order of priority: its
// switch-config's, else the root config's.
func opamOrder(m userconf.Machine, root string) []string {
	config, _ := os.ReadFile(filepath.Join(root, "config"))
	sw := m.Environment("OPAMSWITCH")
	if sw == "" {
		sw = opam.String(config, "switch")
	}
	var names []string
	if sw != "" {
		directory := filepath.Join(root, sw)
		if strings.ContainsAny(sw, `/\`) {
			directory = filepath.Join(sw, "_opam") // a local switch
		}
		if data, err := os.ReadFile(filepath.Join(directory, ".opam-switch", "switch-config")); err == nil {
			for _, r := range opam.Repositories(data) {
				names = append(names, r.Name)
			}
		}
	}
	if len(names) == 0 {
		for _, r := range opam.Repositories(config) {
			names = append(names, r.Name)
		}
	}
	return names
}

// opamCopyPath is the copy opam keeps of a repository, or "".
func opamCopyPath(root, name string) string {
	directory := filepath.Join(root, "repo", name)
	if info, err := os.Stat(filepath.Join(directory, "packages")); err == nil && info.IsDir() {
		return directory
	}
	if info, err := os.Stat(directory + ".tar.gz"); err == nil && !info.IsDir() {
		return directory + ".tar.gz"
	}
	return ""
}

// parseDuneWorkspace reads the opam repositories a dune-workspace names for
// dune's package management. With a lock_dir's `(repositories ...)` those are
// asked in its order (`:standard` being overlay and upstream, each defined by a
// repository stanza or dune's default), and opam-repository only when it is
// one of them; otherwise every repository stanza is asked beside it.
//
// Implements: REQ-SUP-054
func parseDuneWorkspace(data []byte, k sink) {
	defined, order, listed := ocaml.WorkspaceRepositories(data)
	urls := map[string]string{"upstream": opamUpstream, "overlay": opamOverlays}
	for _, r := range defined {
		urls[r.Name] = r.URL
	}
	if !listed {
		for _, r := range defined {
			if !opamPublic(r.URL) {
				k.extra(Opam, r.URL)
			}
		}
		return
	}
	upstream := false
	for _, name := range order {
		names := []string{name}
		if name == ":standard" {
			names = []string{"overlay", "upstream"}
		}
		for _, n := range names {
			switch u := urls[n]; {
			case u == "":
			case opamPublic(u):
				upstream = true
			default:
				k.extra(Opam, u)
			}
		}
	}
	if !upstream {
		k.off(Opam)
	}
}

// opamName is an opam package name.
var opamName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_+.-]{0,127}$`)

// opamRepository is a repository's descriptions as they can be read: its versions of a
// package (nil when they cannot be listed) and one version's description.
type opamRepository struct {
	versions func(name string) ([]string, error)
	read     func(name, version string) ([]byte, error)
}

// opamRepository is how an opam repository is read: from the copy opam keeps of it,
// else as files over HTTP - listed through GitHub for opam-repository itself.
// A repository only git serves, with no copy here, cannot be read at all.
func (c *Client) opamRepository(ctx context.Context, base string) (opamRepository, error) {
	if local := c.config.localCopy(Opam, base); local != "" {
		localCopy, err := c.opamCopies.get(local, func() (*opamCopy, error) { return readOpamCopy(local) })
		if err != nil {
			return opamRepository{}, err
		}
		return opamRepository{versions: localCopy.list, read: localCopy.description}, nil
	}
	if !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://") {
		// Implements: REQ-TRC-017
		c.note(trace.NoteNoCopy, "opam repository "+base+" is read only from a copy on this machine's disk "+
			"(opam's repo/<name>), and there is none: its packages are asked of the next repository")
		return opamRepository{}, fmt.Errorf("%w: %s has no copy on this machine", errAbsent, base)
	}
	r := opamRepository{read: func(name, version string) ([]byte, error) {
		n := url.PathEscape(name)
		return c.accept(ctx, base+"/packages/"+n+"/"+n+"."+url.PathEscape(version)+"/opam", "text/plain")
	}}
	if base == c.config.publicURL(Opam) {
		r.versions = func(name string) ([]string, error) {
			entries, err := c.githubList(ctx, opamGitHub, "packages/"+url.PathEscape(name))
			var out []string
			for _, e := range entries {
				if v, ok := strings.CutPrefix(e, name+"."); ok {
					out = append(out, v)
				}
			}
			return out, err
		}
	}
	return r, nil
}

// opamCopy is a repository copy opam keeps: a directory, or an archive read
// once into memory.
type opamCopy struct {
	directory string
	files     map[string]map[string][]byte // name -> version -> opam file, for an archive
}

func readOpamCopy(local string) (*opamCopy, error) {
	if !strings.HasSuffix(local, ".tar.gz") {
		return &opamCopy{directory: local}, nil
	}
	f, err := os.Open(local)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	localCopy := &opamCopy{files: map[string]map[string][]byte{}}
	tarReader := tar.NewReader(z)
	for {
		h, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		// .../packages/<name>/<name>.<version>/opam
		parts := strings.Split(strings.TrimPrefix(h.Name, "./"), "/")
		n := len(parts)
		if h.Typeflag != tar.TypeReg || n < 4 || parts[n-1] != "opam" || !slices.Contains(parts[:n-3], "packages") {
			continue
		}
		name := parts[n-3]
		version, ok := strings.CutPrefix(parts[n-2], name+".")
		if !ok {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(tarReader, maxBody))
		if err != nil {
			return nil, err
		}
		if localCopy.files[name] == nil {
			localCopy.files[name] = map[string][]byte{}
		}
		localCopy.files[name][version] = body
	}
	return localCopy, nil
}

func (localCopy *opamCopy) list(name string) ([]string, error) {
	if localCopy.files != nil {
		var out []string
		for v := range localCopy.files[name] {
			out = append(out, v)
		}
		if out == nil {
			return nil, errAbsent
		}
		return out, nil
	}
	entries, err := os.ReadDir(filepath.Join(localCopy.directory, "packages", name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errAbsent
	}
	var out []string
	for _, e := range entries {
		if v, ok := strings.CutPrefix(e.Name(), name+"."); ok && e.IsDir() {
			out = append(out, v)
		}
	}
	return out, err
}

func (localCopy *opamCopy) description(name, version string) ([]byte, error) {
	if localCopy.files != nil {
		body, ok := localCopy.files[name][version]
		if !ok {
			return nil, errAbsent
		}
		return body, nil
	}
	body, err := os.ReadFile(filepath.Join(localCopy.directory, "packages", name, name+"."+version, "opam"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errAbsent
	}
	return body, err
}

// ---------------------------------------------------------------- Alire

// alirePublic reports whether an index URL is the Alire community index.
func alirePublic(u string) bool {
	return canonical(u) == "github.com/alire-project/alire-index"
}

// machineAlire reads the indexes this machine's alr uses, from its settings
// directory (ALIRE_SETTINGS_DIR, ~/.config/alire): indexes/<name>/index.toml
// holds each one's url and priority (lower first). Each is asked in that order,
// the community index as the public index, which is off when it is not among
// them; a git index comes with the checkout alr keeps beside it (repo/), a
// directory index is itself. With no index configured, alr adds the community
// index, and so the public default stays.
//
// Implements: REQ-SUP-061, REQ-SUP-064
func machineAlire(m userconf.Machine, k sink) {
	directory := m.AlireSettingsDirectory()
	if directory == "" {
		return
	}
	files, _ := filepath.Glob(filepath.Join(directory, "indexes", "*", "index.toml"))
	type index struct {
		URL      string `toml:"url"`
		Name     string `toml:"name"`
		Priority int    `toml:"priority"`
		local    string
	}
	var indexes []index
	for _, f := range files {
		var parsed index
		if _, err := toml.DecodeFile(f, &parsed); err != nil || parsed.URL == "" {
			continue
		}
		if p := fileURLPath(parsed.URL); p != "" {
			parsed.local = p
		} else if strings.HasPrefix(parsed.URL, "git+") {
			parsed.local = filepath.Join(filepath.Dir(f), "repo")
		}
		if info, err := os.Stat(parsed.local); err != nil || !info.IsDir() {
			parsed.local = ""
		}
		indexes = append(indexes, parsed)
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		if indexes[i].Priority != indexes[j].Priority {
			return indexes[i].Priority < indexes[j].Priority
		}
		return indexes[i].Name < indexes[j].Name
	})
	public := false
	for _, parsed := range indexes {
		u := parsed.URL
		if alirePublic(u) {
			u, public = publicIndex(Alire), true
		}
		k.put(Alire, Source{URL: sshUserless(u), Kind: Listed, Local: parsed.local})
	}
	if len(indexes) > 0 && !public {
		k.off(Alire)
	}
}

// alireIndex is how an Alire index is read: a release's manifest, and a crate's
// versions (nil when they cannot be listed).
type alireIndex struct {
	versions func(crate string) ([]string, error)
	read     func(crate, version string) ([]byte, error)
}

// alireIndex reads an index from the checkout alr keeps of it, else as files
// over HTTP - listed through GitHub for the community index. An index only git
// serves, with no checkout here, cannot be read at all.
func (c *Client) alireIndex(ctx context.Context, base string) (alireIndex, error) {
	if local := c.config.localCopy(Alire, base); local != "" {
		root := alireRoot(local)
		directory := func(crate string) string { return filepath.Join(root, crate[:2], crate) }
		return alireIndex{
			versions: func(crate string) ([]string, error) {
				entries, err := os.ReadDir(directory(crate))
				if errors.Is(err, os.ErrNotExist) {
					return nil, errAbsent
				}
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				return alireVersions(crate, names), err
			},
			read: func(crate, version string) ([]byte, error) {
				body, err := os.ReadFile(filepath.Join(directory(crate), crate+"-"+version+".toml"))
				if errors.Is(err, os.ErrNotExist) {
					return nil, errAbsent
				}
				return body, err
			},
		}, nil
	}
	if !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://") {
		// Implements: REQ-TRC-017
		c.note(trace.NoteNoCopy, "Alire index "+base+" is read only from a checkout on this machine's disk "+
			"(alr's indexes/<name>/repo), and there is none: its crates are asked of the next index")
		return alireIndex{}, fmt.Errorf("%w: %s has no copy on this machine", errAbsent, base)
	}
	alire := alireIndex{read: func(crate, version string) ([]byte, error) {
		return c.accept(ctx, base+"/index/"+crate[:2]+"/"+crate+"/"+crate+"-"+url.PathEscape(version)+".toml", "text/plain")
	}}
	if base == c.config.publicURL(Alire) {
		alire.versions = func(crate string) ([]string, error) {
			names, err := c.githubList(ctx, alireGitHub, "index/"+crate[:2]+"/"+crate)
			return alireVersions(crate, names), err
		}
	}
	return alire, nil
}

// alireVersions are the versions of a crate's release manifests
// (<crate>-<version>.toml) among file names.
func alireVersions(crate string, names []string) []string {
	var out []string
	for _, n := range names {
		v, ok := strings.CutPrefix(n, crate+"-")
		if v, ok2 := strings.CutSuffix(v, ".toml"); ok && ok2 && v != "" && v[0] >= '0' && v[0] <= '9' {
			out = append(out, v)
		}
	}
	return out
}

// alireRoot is the directory of an index checkout that holds its index.toml,
// below which the crates are: the checkout itself, else its index/ directory.
func alireRoot(local string) string {
	if _, err := os.Stat(filepath.Join(local, "index.toml")); err == nil {
		return local
	}
	if found, _ := filepath.Glob(filepath.Join(local, "*", "index.toml")); len(found) > 0 {
		sort.Strings(found)
		return filepath.Dir(found[0])
	}
	return filepath.Join(local, "index")
}

// release is the version a listing gives for a constraint: the newest it admits
// (see opam.Newest, ada.Newest), or errAbsent when none does.
func release(versions []string, err error, newest func([]string) string) (string, error) {
	if err != nil {
		return "", err
	}
	if v := newest(versions); v != "" {
		return v, nil
	}
	return "", errAbsent
}
