package lang

import "strings"

// publicForges are the git hosts whose repositories anybody can clone. The
// plugins that read git dependencies call a repository on any other host the
// organization's own (they put its URL in Target.Origin, which makes the
// package private), and a commit on such a host is not sent anywhere either.
var publicForges = map[string]bool{
	"github.com": true, "gitlab.com": true, "bitbucket.org": true, "codeberg.org": true,
	"git.sr.ht": true, "sr.ht": true,
}

// PublicForge reports whether a repository - a URL, or a name as RepoName
// spells it - is on a public git forge.
//
// Implements: REQ-FND-026
func PublicForge(repository string) bool {
	host, _, _ := strings.Cut(RepositoryName(repository), "/")
	return publicForges[host]
}

// PublicOrUnnamed is PublicForge for a dependency that may name no repository.
// One that names none is not taken for the organization's own, because nothing
// says where it came from, so it gets no Target.Origin either.
func PublicOrUnnamed(repository string) bool {
	return repository == "" || PublicForge(repository)
}

// forgeNamed are the ecosystems whose plugins record a git dependency on any host
// but a public forge in Target.Origin: a package of theirs pinned to a commit that
// has neither an Origin nor a host in its name was cloned from a public forge
// (the plugin's own `public` check said so), although the node does not say
// which repository.
//
// Not a submodule with a relative URL (on the superproject's host, whichever that
// is), nor a Racket package, whose commit is a catalog's checksum and whose
// catalog may be the organization's own.
var forgeNamed = map[string]bool{
	"shards": true, "alire": true, "haxelib": true, "soldeer": true, "fpm": true,
	"nimble": true, "quicklisp": true,
}

// GitPin is the git commit a package is fixed to, and the repository it was
// cloned from ("" when the package does not say), read from what the map
// records: the Git field (Target.Git), a full commit as the version (Carthage,
// Zig, Paket, CMake's FetchContent, Terraform modules, SwiftPM revisions,
// submodules, jsonnet-bundler, dub, fpm, nimble, mix and rebar3 git deps, …), a
// shard's "1.2.3+git.commit.<sha>", an npm "github:owner/repo#<sha>" or
// "git+https://…#<sha>" and Composer's "dev-main#<sha>". The repository comes
// from the Git field, then the origin, then a version naming one, then a
// package named after its repository (github.com/owner/repo).
//
// Only a full commit counts: 40 hex digits (SHA-1) or 64 (SHA-256). A
// shortened one (Bun's seven digits, Nix's display form) could match another
// repository's commit, so it is not a pin anybody can be asked about.
//
// public says whether the commit may be sent to a vulnerability database: the
// repository is on a public forge, or the package names none but belongs to an
// ecosystem whose plugins would have recorded a private one in the origin.
//
// Implements: REQ-FND-026
func GitPin(ecosystem, name, version, origin, git string) (commit, repository string, public bool) {
	if url, c, ok := cutLast(git, "#"); ok && Commit(c) {
		commit, repository = c, url
	}
	v := strings.TrimSpace(version)
	if commit == "" {
		switch {
		case Commit(v):
			commit = v
		case strings.Contains(v, "+git.commit."):
			_, c, _ := cutLast(v, "+git.commit.")
			if Commit(c) {
				commit = c
			}
		default:
			if spec, c, ok := cutLast(v, "#"); ok && Commit(c) {
				commit = c
				if repository == "" && ecosystem == "npm" {
					repository = npmRepository(spec)
				}
			}
		}
	}
	if commit == "" {
		return "", "", false
	}
	commit = strings.ToLower(commit)
	if repository == "" && remote(origin) {
		repository = origin
	}
	if repository == "" && remote(name) && ecosystem != "npm" && ecosystem != "composer" {
		repository = name // named after its repository: github.com/owner/repo
	}
	if repository != "" {
		repository = RepositoryName(strings.TrimPrefix(repository, "git+"))
		return commit, repository, PublicForge(repository)
	}
	return commit, "", origin == "" && forgeNamed[ecosystem]
}

// remote reports whether s names a repository on a host rather than a directory,
// an archive or a package: a URL, an scp-like git@host:path, or host/owner/repo.
func remote(s string) bool {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "://") {
		return !strings.HasPrefix(s, "file://")
	}
	if at, rest, ok := strings.Cut(s, "@"); ok && !strings.Contains(at, "/") && strings.Contains(rest, ":") {
		return true
	}
	host, rest, ok := strings.Cut(s, "/")
	return ok && rest != "" && strings.Contains(host, ".") && !strings.HasPrefix(host, ".") &&
		!strings.Contains(host, ":")
}

// npmRepository is the repository an npm git specifier names: "github:owner/repo",
// "gitlab:…", "bitbucket:…", a bare "owner/repo" (GitHub), or a git URL.
func npmRepository(spec string) string {
	if prefix, rest, ok := strings.Cut(spec, ":"); ok && !strings.Contains(prefix, "/") {
		if host := map[string]string{"github": "github.com", "gitlab": "gitlab.com", "bitbucket": "bitbucket.org"}[prefix]; host != "" {
			return host + "/" + rest
		}
	}
	if remote(spec) {
		return spec
	}
	if owner, repository, ok := strings.Cut(spec, "/"); ok && owner != "" && repository != "" && !strings.ContainsAny(spec, ":@") {
		return "github.com/" + spec
	}
	return ""
}

// cutLast is strings.Cut at the last separator.
func cutLast(s, separator string) (before, after string, found bool) {
	if i := strings.LastIndex(s, separator); i >= 0 {
		return s[:i], s[i+len(separator):], true
	}
	return s, "", false
}
