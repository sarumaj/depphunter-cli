package dhall

import (
	"cmp"
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
	return &resolver{files: lang.PathSet(all)}
}

// Resolve maps a relative path to the repository's file, a URL to its
// package, and drops what names nothing in the repository: absolute and
// home-relative paths, environment variables and paths to missing files.
//
// Implements: REQ-DHALL-004, REQ-DHALL-005, REQ-DHALL-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	loc := rawImport.Module
	switch {
	case strings.HasPrefix(loc, "https://") || strings.HasPrefix(loc, "http://"):
		return remote(loc, rawImport.Name)
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
	var packageName, versionText string
	reference := false // versionText is a repository reference, which may be a branch
	switch {
	case host == "prelude.dhall-lang.org":
		packageName = prelude
		if len(segments) > 1 && version(segments[0]) {
			versionText = segments[0]
		}
	case host == "raw.githubusercontent.com" && len(segments) >= 3:
		packageName, versionText, reference = github(segments[0], segments[1], segments[3:]), segments[2], true
	case host == "github.com" && len(segments) >= 4 && (segments[2] == "raw" || segments[2] == "blob"):
		packageName, versionText, reference = github(segments[0], segments[1], segments[4:]), segments[3], true
	case host == "cdn.jsdelivr.net" && len(segments) >= 3 && segments[0] == "gh":
		repository, r, _ := strings.Cut(segments[2], "@")
		packageName, versionText, reference = github(segments[1], repository, segments[3:]), r, r != ""
	case host == "gitlab.com" && gitlab(segments) > 0:
		k := gitlab(segments)
		packageName, versionText, reference = host+"/"+strings.Join(segments[:k], "/"), segments[k+2], true
	default:
		packageName = host
		k := len(segments) - 1 // the file
		for i, s := range segments[:max(k, 0)] {
			if version(s) || lang.Commit(s) {
				k, versionText = i, s
				break
			}
		}
		if k > 0 {
			packageName += "/" + strings.Join(segments[:k], "/")
		}
	}
	t := lang.Target{Ecosystem: ecosystemDhall, Package: packageName, Version: versionText}
	switch {
	case hash != "":
		t.Pinned = true
		t.Version = cmp.Or(t.Version, hash[:min(len(hash), len("sha256:")+12)])
	case versionText == "", reference && !version(versionText) && !lang.Commit(versionText):
		t.Floating = true // no version, or a branch
	}
	return t
}

// github names a GitHub repository, or the Prelude when the path is in
// dhall-lang's Prelude/.
func github(org, repository string, rest []string) string {
	if strings.EqualFold(org, "dhall-lang") && strings.EqualFold(repository, "dhall-lang") && len(rest) > 1 && rest[0] == "Prelude" {
		return prelude
	}
	return lang.RepositoryName("https://github.com/" + org + "/" + repository)
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
