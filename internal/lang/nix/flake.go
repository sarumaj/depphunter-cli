package nix

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// ref is a flake reference or a fetcher's source, whichever way it was written:
// "github:NixOS/nixpkgs/nixos-24.05", "git+https://host/r?ref=main&rev=...",
// { type = "github"; owner = ...; }, builtins.fetchTarball { url; sha256; }.
type ref struct {
	typ     string // github, gitlab, sourcehut, git, hg, tarball, file, path, indirect
	url     string // what git, hg, tarball, file and path fetch
	owner   string
	repo    string
	host    string
	ref     string // a branch or tag
	rev     string // a commit
	narHash string // a content hash: narHash, or a fetcher's sha256/hash
	id      string // an indirect reference's registry name
	channel string // an npins channel's name (nixos-24.05), not encoded
}

var archiveExts = []string{".tar.gz", ".tgz", ".tar.xz", ".txz", ".tar.bz2", ".tbz2", ".tar.zst", ".tzst", ".tar", ".zip"}

// parseRef reads a flake reference URL.
//
// Implements: REQ-NIX-004
func parseRef(s string) ref {
	s = strings.TrimSpace(s)
	var r ref
	body, query, _ := strings.Cut(s, "?")
	q, _ := url.ParseQuery(query)
	r.ref, r.rev, r.narHash, r.host = q.Get("ref"), q.Get("rev"), q.Get("narHash"), q.Get("host")
	scheme, rest, hasScheme := strings.Cut(body, ":")
	if !hasScheme || strings.HasPrefix(body, ".") || strings.HasPrefix(body, "/") {
		if strings.HasPrefix(body, ".") || strings.HasPrefix(body, "/") {
			return ref{typ: "path", url: body}
		}
		scheme, rest = "flake", body // "nixpkgs", "nixpkgs/nixos-24.05": the registry
	}
	switch scheme {
	case "flake":
		r.typ = "indirect"
		parts := strings.Split(rest, "/")
		if !registryName.MatchString(parts[0]) {
			return ref{} // "@nixpkgs@": a placeholder, not a registry name
		}
		r.id = parts[0]
		for _, p := range parts[1:] {
			if lang.Commit(p) {
				r.rev = p
			} else if p != "" {
				r.ref = p
			}
		}
	case "github", "gitlab", "sourcehut":
		r.typ = scheme
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) >= 2 {
			r.owner, r.repo = parts[0], parts[1]
		}
		if len(parts) == 3 && parts[2] != "" {
			if lang.Commit(parts[2]) {
				r.rev = parts[2]
			} else {
				r.ref = parts[2]
			}
		}
	case "path":
		r.typ, r.url = "path", rest
	default:
		transport := scheme
		if a, b, ok := strings.Cut(scheme, "+"); ok {
			r.typ, transport = a, b
		}
		r.url = transport + ":" + rest
		switch r.typ {
		case "git", "hg", "tarball", "file":
		case "":
			switch {
			case transport == "git" || transport == "ssh":
				r.typ = "git"
			case hasArchiveExt(rest):
				r.typ = "tarball"
			default:
				r.typ = "file"
			}
		default:
			r.typ = "file"
		}
		if transport == "file" && (r.typ == "git" || r.typ == "hg") {
			r.typ, r.url = "path", strings.TrimPrefix(strings.TrimPrefix(rest, "//"), "localhost")
		}
	}
	return r
}

func hasArchiveExt(s string) bool {
	for _, e := range archiveExts {
		if strings.HasSuffix(strings.ToLower(s), e) {
			return true
		}
	}
	return false
}

// attrsRef reads a flake input or fetchTree written as an attribute set.
func attrsRef(f map[string]string) ref {
	if u := f["url"]; u != "" && f["type"] == "" {
		r := parseRef(u)
		for k, v := range map[string]*string{"ref": &r.ref, "rev": &r.rev, "narHash": &r.narHash} {
			if f[k] != "" {
				*v = f[k]
			}
		}
		return r
	}
	r := ref{typ: f["type"], owner: f["owner"], repo: f["repo"], host: f["host"], ref: f["ref"], rev: f["rev"],
		narHash: f["narHash"], id: f["id"], url: f["url"]}
	if r.typ == "path" && r.url == "" {
		r.url = f["path"]
	}
	return r
}

// encode and decode carry a ref through RawImport.Module, which is a string.
func (r ref) encode() string {
	v := url.Values{}
	for k, s := range map[string]string{"t": r.typ, "u": r.url, "o": r.owner, "r": r.repo, "h": r.host,
		"ref": r.ref, "rev": r.rev, "n": r.narHash, "id": r.id} {
		if s != "" {
			v.Set(k, s)
		}
	}
	return v.Encode()
}

func decodeRef(s string) ref {
	v, _ := url.ParseQuery(s)
	return ref{typ: v.Get("t"), url: v.Get("u"), owner: v.Get("o"), repo: v.Get("r"), host: v.Get("h"),
		ref: v.Get("ref"), rev: v.Get("rev"), narHash: v.Get("n"), id: v.Get("id")}
}

var (
	// github.com/o/r/archive/<ref>.tar.gz, also refs/heads/<b> and refs/tags/<t>.
	githubArchive = regexp.MustCompile(`^((?:github\.com|codeberg\.org|git\.sr\.ht)/[^/]+/[^/]+)/archive/(refs/heads/|refs/tags/)?(.+)$`)
	gitlabArchive = regexp.MustCompile(`^(.+?)/-/archive/([^/]+)/[^/]+$`)
	// api.github.com/repos/o/r/tarball/<ref>
	githubTarball = regexp.MustCompile(`^api\.github\.com/repos/([^/]+/[^/]+)/tarball/(.+)$`)
	// nixos.org/channels/<channel>/nixexprs.tar.xz and channels.nixos.org/<channel>/...
	channelTarball = regexp.MustCompile(`^(?:nixos\.org/channels|channels\.nixos\.org|releases\.nixos\.org/[^/]+/[^/]+)/([^/]+)/nixexprs$`)
	// flakehub.com/f/o/r/<version> and api.flakehub.com/f/pinned/o/r/<version>/<id>/source
	registryName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	namedArchive = regexp.MustCompile(`^(.+)/([^/]+?)-(v?[0-9][0-9A-Za-z.+_-]*)$`)
	flakehub     = regexp.MustCompile(`^(?:api\.)?flakehub\.com/f/(pinned/)?([^/]+/[^/]+)/([^/]+)`)
	tagLike      = regexp.MustCompile(`^v?\d+(\.\d+)*([-+._][0-9A-Za-z.+-]*)?$`)
)

// target names what r fetches as a package of the nix island, versioned and pinned
// by what the reference itself says (a lock says more: see lockTarget). A path
// reference returns ok false: it is a directory of the repository.
//
// Implements: REQ-NIX-004, REQ-NIX-006
func (r ref) target() (lang.Target, bool) {
	t := lang.Target{Ecosystem: ecoNix}
	version, head := r.ref, strings.HasPrefix(r.ref, "refs/heads/")
	switch r.typ {
	case "path", "":
		return lang.Target{}, false
	case "indirect":
		// The flake registry decides what "nixpkgs" means on each machine.
		t.Package = r.id
		t.Version = first(short(r.rev), r.ref)
		t.Pinned = r.rev != "" || r.narHash != ""
		t.Floating = !t.Pinned
		return t, true
	case "github", "gitlab", "sourcehut":
		host := r.host
		if host == "" {
			host = map[string]string{"github": "github.com", "gitlab": "gitlab.com", "sourcehut": "git.sr.ht"}[r.typ]
		}
		t.Package = lang.RepoName("https://" + host + "/" + r.owner + "/" + r.repo)
		if strings.Contains(r.owner, "%2F") || strings.Contains(r.owner, "%2f") { // gitlab:group%2Fsub/repo
			t.Package = strings.ReplaceAll(strings.ReplaceAll(t.Package, "%2F", "/"), "%2f", "/")
		}
	case "git", "hg":
		t.Package = lang.RepoName(r.url)
	default: // tarball, file
		u, _, _ := strings.Cut(r.url, "?")
		if un, err := url.PathUnescape(u); err == nil {
			u = un // flakehub.com/f/NixOS/nixpkgs/0.1.%2A.tar.gz
		}
		name := lang.RepoName(u)
		stripped := name
		for _, e := range archiveExts {
			if strings.HasSuffix(strings.ToLower(stripped), e) {
				stripped = stripped[:len(stripped)-len(e)]
				break
			}
		}
		switch m := (struct{ gh, gl, api, ch, fh []string }{githubArchive.FindStringSubmatch(stripped),
			gitlabArchive.FindStringSubmatch(stripped), githubTarball.FindStringSubmatch(stripped),
			channelTarball.FindStringSubmatch(stripped), flakehub.FindStringSubmatch(stripped)}); {
		case m.gh != nil:
			t.Package, version, head = m.gh[1], m.gh[3], m.gh[2] == "refs/heads/"
			if m.gh[2] == "refs/tags/" {
				version = "refs/tags/" + version
			}
		case m.gl != nil:
			t.Package, version = m.gl[1], m.gl[2]
		case m.api != nil:
			t.Package, version = "github.com/"+m.api[1], m.api[2]
		case m.ch != nil:
			// A channel's tarball: the nixpkgs of that channel, which moves.
			t.Package, version, head = "nixpkgs", m.ch[1], true
			if strings.Count(m.ch[1], ".") >= 2 { // a release: nixos-24.05.1234.abcdef
				head = false
			}
		case m.fh != nil:
			t.Package, version = "flakehub.com/f/"+m.fh[2], m.fh[3]
			if m.fh[1] != "" { // a pinned FlakeHub URL names one release
				t.Pinned = true
			}
		default:
			t.Package = stripped
			if m := namedArchive.FindStringSubmatch(stripped); m != nil && stripped != name {
				t.Package, version = m[1]+"/"+m[2], m[3] // downloads/baz-1.2.tar.gz
			}
		}
		if lang.Commit(version) {
			r.rev = version
		}
	}
	if strings.HasPrefix(t.Package, "github.com/") {
		// GitHub's names ignore case: github:NixOS/nixpkgs and github:nixos/nixpkgs
		// are one repository.
		t.Package = strings.ToLower(t.Package)
	}
	switch {
	case r.rev != "":
		t.Version, t.Pinned, t.Git = short(r.rev), true, r.git()
		if r.ref != "" && r.ref != r.rev {
			t.Requested = r.ref
		}
	case r.narHash != "":
		t.Version, t.Pinned = first(version, short(r.narHash)), true
	default:
		t.Version = strings.TrimPrefix(strings.TrimPrefix(version, "refs/tags/"), "refs/heads/")
		if !t.Pinned {
			// A tag is shown and neither pins nor floats; a branch, a channel or
			// no ref at all moves with every update.
			tag := strings.HasPrefix(version, "refs/tags/") || tagLike.MatchString(version)
			t.Floating = version == "" || head || !tag
		}
	}
	return t, true
}

// git is the checkout a reference locks, "<repository URL>#<commit>", for
// Target.Git: the version shows the commit shortened, as Nix prints it, and the
// vulnerability database is asked about the whole one. "" for anything that is
// not a git repository at a full commit.
//
// Implements: REQ-FND-026
func (r ref) git() string {
	if !lang.Commit(r.rev) {
		return ""
	}
	switch r.typ {
	case "github", "gitlab", "sourcehut":
		host := r.host
		if host == "" {
			host = map[string]string{"github": "github.com", "gitlab": "gitlab.com", "sourcehut": "git.sr.ht"}[r.typ]
		}
		owner := strings.ReplaceAll(strings.ReplaceAll(r.owner, "%2F", "/"), "%2f", "/")
		return "https://" + host + "/" + owner + "/" + r.repo + "#" + r.rev
	case "git":
		u, _, _ := strings.Cut(strings.TrimPrefix(r.url, "git+"), "?")
		return u + "#" + r.rev
	}
	return ""
}

// short is a commit shortened as Nix prints it, or a content hash's first
// characters.
func short(s string) string {
	s = strings.TrimSpace(s)
	if lang.Commit(s) {
		return s[:7]
	}
	if h, ok := strings.CutPrefix(s, "sha256-"); ok && len(h) > 12 {
		return "sha256-" + h[:12]
	}
	if len(s) > 19 {
		return s[:19]
	}
	return s
}

func first(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// localDir is where a path reference points, relative to the repository root, or
// "" when it leaves the repository or is absolute.
func localDir(fromDir, p string) string {
	if p == "" || strings.HasPrefix(p, "/") {
		return ""
	}
	d := path.Clean(path.Join(fromDir, p))
	if d == ".." || strings.HasPrefix(d, "../") {
		return ""
	}
	return d
}
