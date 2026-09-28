package dhall

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// prelude names the Dhall Prelude, which prelude.dhall-lang.org serves from
// the Prelude/ directory of github.com/dhall-lang/dhall-lang.
const prelude = "github.com/dhall-lang/dhall-lang/Prelude"

type resolver struct {
	files map[string]bool
}

func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: make(map[string]bool, len(all))}
	for _, f := range all {
		r.files[f.Path] = true
	}
	return r
}

// Resolve maps a relative path to the repository's file, a URL to its
// package, and drops what names nothing in the repository: absolute and
// home-relative paths, environment variables and paths to missing files.
//
// Implements: REQ-DHALL-004, REQ-DHALL-005, REQ-DHALL-008
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	loc := imp.Module
	switch {
	case strings.HasPrefix(loc, "https://") || strings.HasPrefix(loc, "http://"):
		return remote(loc, imp.Name)
	case strings.HasPrefix(loc, "./") || strings.HasPrefix(loc, "../"):
		p := path.Join(path.Dir(file), strings.ReplaceAll(loc, `"`, "")) // ./"a b"/c.dhall
		if r.files[p] {
			return lang.Target{Local: p}
		}
	}
	return lang.Target{}
}

// remote names the package a URL downloads: the Prelude (from
// prelude.dhall-lang.org or dhall-lang's repository), a repository whose raw
// files GitHub, GitLab or jsDelivr serve, else the host and the path up to
// the first segment that is a version (or, without one, the URL's
// directory). The version is that segment, or the repository reference. A
// sha256 hash pins: Dhall refuses content that does not match it. Without
// one, a URL naming no version or a branch floats.
//
// Implements: REQ-DHALL-005
func remote(url, hash string) lang.Target {
	u := url[strings.Index(url, "://")+3:]
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	host, p, _ := strings.Cut(u, "/")
	if _, h, ok := strings.Cut(host, "@"); ok {
		host = h
	}
	host, _, _ = strings.Cut(strings.ToLower(host), ":")
	var segments []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}
	var pkg, ver string
	ref := false // ver is a repository reference, which may be a branch
	switch {
	case host == "prelude.dhall-lang.org":
		pkg = prelude
		if len(segments) > 1 && version(segments[0]) {
			ver = segments[0]
		}
	case host == "raw.githubusercontent.com" && len(segments) >= 3:
		pkg, ver, ref = github(segments[0], segments[1], segments[3:]), segments[2], true
	case host == "github.com" && len(segments) >= 4 && (segments[2] == "raw" || segments[2] == "blob"):
		pkg, ver, ref = github(segments[0], segments[1], segments[4:]), segments[3], true
	case host == "cdn.jsdelivr.net" && len(segments) >= 3 && segments[0] == "gh":
		repo, r, _ := strings.Cut(segments[2], "@")
		pkg, ver, ref = github(segments[1], repo, segments[3:]), r, r != ""
	case host == "gitlab.com" && gitlab(segments) > 0:
		k := gitlab(segments)
		pkg, ver, ref = host+"/"+strings.Join(segments[:k], "/"), segments[k+2], true
	default:
		pkg = host
		k := len(segments) - 1 // the file
		for i, s := range segments[:max(k, 0)] {
			if version(s) || lang.Commit(s) {
				k, ver = i, s
				break
			}
		}
		if k > 0 {
			pkg += "/" + strings.Join(segments[:k], "/")
		}
	}
	t := lang.Target{Ecosystem: ecoDhall, Package: pkg, Version: ver}
	switch {
	case hash != "":
		t.Pinned = true
		if t.Version == "" {
			t.Version = hash[:min(len(hash), len("sha256:")+12)]
		}
	case ver == "", ref && !version(ver) && !lang.Commit(ver):
		t.Floating = true // no version, or a branch
	}
	return t
}

// github names a GitHub repository, or the Prelude when the path is in
// dhall-lang's Prelude/.
func github(org, repo string, rest []string) string {
	if strings.EqualFold(org, "dhall-lang") && strings.EqualFold(repo, "dhall-lang") && len(rest) > 1 && rest[0] == "Prelude" {
		return prelude
	}
	return lang.RepoName("https://github.com/" + org + "/" + repo)
}

// gitlab is the length of the repository path in a GitLab raw URL
// (group/project/-/raw/<ref>/...), or 0.
func gitlab(segments []string) int {
	for k := 2; k+2 < len(segments); k++ {
		if segments[k] == "-" && (segments[k+1] == "raw" || segments[k+1] == "blob") {
			return k
		}
	}
	return 0
}

// version reports whether a path segment is a version: v1, 1.2, v23.0.0,
// 1.2.3-rc1.
func version(s string) bool {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false
	}
	core, _, _ := strings.Cut(s, "-")
	for _, part := range strings.Split(core, ".") {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return false
		}
	}
	return true
}
