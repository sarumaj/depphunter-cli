// Package links says where a package can be looked at in a web browser: its page on
// the index that publishes it, and the repository its source lives in.
//
// Both are worked out from what the graph already holds - the ecosystem, the name,
// the index, the origin - and nothing is fetched to confirm them, so a link can
// lead to a page that does not exist. That is the price of a map that is the same
// offline as online.
package links

import (
	"net/url"
	"strings"
)

// Page is a package's page on its ecosystem's public index, or "" when the ecosystem
// has no such site or the name is not one the site publishes. index is the index the
// package resolves from: "" or the ecosystem's public one, as the caller has checked;
// it only tells Maven Central from Clojars.
//
// Implements: REQ-MOD-014
func Page(ecosystem, name, index string) string {
	if name == "" || strings.ContainsAny(name, " \t\n\\?#") {
		return ""
	}
	escaped := escape(name)
	switch ecosystem {
	case "npm":
		return "https://www.npmjs.com/package/" + escaped
	case "pypi":
		return "https://pypi.org/project/" + escaped + "/"
	case "go", "go-std":
		return "https://pkg.go.dev/" + escaped
	case "crates":
		return "https://crates.io/crates/" + escaped
	case "maven":
		group, artifact, ok := strings.Cut(name, ":")
		if !ok || group == "" || artifact == "" {
			return ""
		}
		if index == "https://repo.clojars.org" {
			if group == artifact {
				return "https://clojars.org/" + escape(artifact)
			}
			return "https://clojars.org/" + escape(group) + "/" + escape(artifact)
		}
		return "https://central.sonatype.com/artifact/" + escape(group) + "/" + escape(artifact)
	case "nuget":
		return "https://www.nuget.org/packages/" + escaped
	case "psgallery":
		return "https://www.powershellgallery.com/packages/" + escaped
	case "oci":
		return dockerHub(name)
	case "composer":
		return "https://packagist.org/packages/" + escaped
	case "rubygems":
		return "https://rubygems.org/gems/" + escaped
	case "pub":
		return "https://pub.dev/packages/" + escaped
	case "hex":
		return "https://hex.pm/packages/" + escaped
	case "cran":
		return "https://cran.r-project.org/package=" + escaped
	case "bioconductor":
		return "https://bioconductor.org/packages/" + escaped + "/"
	case "hackage":
		return "https://hackage.haskell.org/package/" + escaped
	case "cocoapods":
		pod, _, _ := strings.Cut(name, "/")
		return "https://cocoapods.org/pods/" + escape(pod)
	case "luarocks":
		return "https://luarocks.org/search?q=" + url.QueryEscape(name)
	case "cpan":
		return "https://metacpan.org/dist/" + escaped
	case "opam":
		return "https://opam.ocaml.org/packages/" + escaped + "/"
	case "julia":
		return "https://juliahub.com/ui/Packages/General/" + escaped
	case "bazel":
		return "https://registry.bazel.build/modules/" + escaped
	case "elm":
		if strings.Count(name, "/") != 1 {
			return ""
		}
		return "https://package.elm-lang.org/packages/" + escaped + "/latest/"
	case "purescript":
		if strings.Contains(name, "/") {
			return ""
		}
		return "https://pursuit.purescript.org/packages/purescript-" + escaped
	case "dub":
		root, _, _ := strings.Cut(name, ":")
		return "https://code.dlang.org/packages/" + escape(root)
	case "alire":
		return "https://alire.ada.dev/crates/" + escaped
	case "puppet-forge":
		author, module, ok := strings.Cut(name, "-")
		if !ok || strings.Contains(name, "/") {
			return ""
		}
		return "https://forge.puppet.com/modules/" + escape(author) + "/" + escape(module)
	case "raco":
		return "https://pkgs.racket-lang.org/package/" + escaped
	case "wally":
		if strings.Count(name, "/") != 1 {
			return ""
		}
		return "https://wally.run/package/" + escaped
	case "buf":
		if !strings.HasPrefix(name, "buf.build/") {
			return ""
		}
		return "https://" + escaped
	case "conan":
		return "https://conan.io/center/recipes/" + escaped
	case "terraform-module":
		module, _, _ := strings.Cut(name, "//")
		if strings.Count(module, "/") != 2 || hosted(module) {
			return ""
		}
		return "https://registry.terraform.io/modules/" + escape(module)
	case "terraform-provider":
		if strings.Count(name, "/") != 1 || hosted(name) {
			return ""
		}
		return "https://registry.terraform.io/providers/" + escaped
	case "haxelib":
		return "https://lib.haxe.org/p/" + escaped + "/"
	case "nimble":
		if strings.Contains(name, "/") {
			return ""
		}
		return "https://nimble.directory/pkg/" + escaped
	case "shards":
		return "https://shardbox.org/shards/" + escaped
	case "nixpkgs":
		return "https://search.nixos.org/packages?query=" + url.QueryEscape(name)
	case "vcpkg":
		return "https://vcpkg.io/en/package/" + escaped
	case "python-std":
		module, _, _ := strings.Cut(name, ".")
		return "https://docs.python.org/3/library/" + escape(module) + ".html"
	case "rust-std":
		return "https://doc.rust-lang.org/" + escaped + "/"
	case "node":
		module, _, _ := strings.Cut(strings.TrimPrefix(name, "node:"), "/")
		return "https://nodejs.org/api/" + escape(module) + ".html"
	}
	return ""
}

// dockerHub is an image's page on Docker Hub, the official images under _/, or ""
// for an image another registry serves.
func dockerHub(name string) string {
	name = strings.TrimPrefix(name, "docker.io/")
	name = strings.TrimPrefix(name, "library/")
	if hosted(name) {
		return ""
	}
	switch strings.Count(name, "/") {
	case 0:
		return "https://hub.docker.com/_/" + escape(name)
	case 1:
		return "https://hub.docker.com/r/" + escape(name)
	}
	return ""
}

// hosted reports whether a name's first segment is a host: a registry other than
// the ecosystem's public one.
func hosted(name string) bool {
	first, _, ok := strings.Cut(name, "/")
	return ok && (strings.ContainsAny(first, ".:") || first == "localhost")
}

// Repository is the web page of the repository a package's source lives in, or ""
// when nothing says where that is. git is the checkout the package was built from
// ("<repository URL>#<commit>"), origin where it was installed from, and index the
// index it resolves from; the name is read last, where it is a repository path.
//
// Implements: REQ-MOD-014
func Repository(ecosystem, name, index, origin, git string) string {
	if git != "" {
		repository, commit, _ := strings.Cut(git, "#")
		if web := webURL(repository); web != "" {
			return atCommit(web, commit)
		}
	}
	if web := webURL(origin); web != "" {
		return web
	}
	switch ecosystem {
	case "actions":
		owner, rest, _ := strings.Cut(name, "/")
		repository, _, _ := strings.Cut(rest, "/")
		if owner == "" || repository == "" {
			return ""
		}
		return githubHost(index) + "/" + escape(owner) + "/" + escape(repository)
	case "elm":
		if strings.Count(name, "/") == 1 {
			return "https://github.com/" + escape(name)
		}
	}
	return forgePath(name)
}

// forges are the hosts whose repositories are owner/name, found under a name that
// starts with the host: a Go module, a Swift package, a submodule.
var forges = map[string]bool{"github.com": true, "gitlab.com": true, "codeberg.org": true, "bitbucket.org": true}

// forgePath is the repository a name like github.com/owner/repository/sub names.
func forgePath(name string) string {
	name, _, _ = strings.Cut(name, "//")
	segments := strings.Split(name, "/")
	if len(segments) < 3 || !forges[segments[0]] || segments[1] == "" || segments[2] == "" {
		return ""
	}
	return "https://" + segments[0] + "/" + escape(segments[1]) + "/" + escape(strings.TrimSuffix(segments[2], ".git"))
}

// githubHost is the web host behind a GitHub REST API: https://github.com for the
// public one, the enterprise server's own for https://ghe.example/api/v3.
func githubHost(index string) string {
	u, err := url.Parse(index)
	if index == "" || err != nil || u.Host == "" || u.Host == "api.github.com" {
		return "https://github.com"
	}
	return "https://" + u.Host
}

// webURL is the page of the repository a VCS reference names - git+https://,
// git::ssh://, git@host:owner/repo.git, an archive of a forge's - or "" when
// reference is a local path, a plain archive or nothing at all.
func webURL(reference string) string {
	reference = strings.TrimSpace(reference)
	for _, prefix := range []string{"git+", "git::", "hg+"} {
		reference = strings.TrimPrefix(reference, prefix)
	}
	if user, rest, ok := strings.Cut(reference, "@"); ok && !strings.Contains(user, "/") && !strings.Contains(user, ":") {
		if host, p, ok := strings.Cut(rest, ":"); ok && !strings.HasPrefix(p, "//") {
			reference = "https://" + host + "/" + p
		}
	}
	u, err := url.Parse(reference)
	if err != nil || u.Host == "" {
		return ""
	}
	switch u.Scheme {
	case "http", "https", "ssh", "git":
	default:
		return ""
	}
	p, _, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "//")
	p, _, _ = strings.Cut(p, "/-/")
	segments := strings.Split(strings.Trim(p, "/"), "/")
	if forges[u.Hostname()] && u.Hostname() != "gitlab.com" && len(segments) > 2 {
		segments = segments[:2]
	}
	last := len(segments) - 1
	if segments[last] == "" {
		return ""
	}
	if archive(segments[last]) && len(segments) > 2 {
		return ""
	}
	// pip's git+https://host/owner/repo.git@v1.2, the revision after the path.
	segments[last], _, _ = strings.Cut(segments[last], "@")
	segments[last] = strings.TrimSuffix(segments[last], ".git")
	if archive(segments[last]) {
		return ""
	}
	return "https://" + u.Hostname() + "/" + strings.Join(segments, "/")
}

func archive(name string) bool {
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".zip", ".whl", ".gem", ".jar"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// atCommit is a forge repository's page at one commit; a host whose layout is not
// known gets the repository itself.
func atCommit(repository, commit string) string {
	if commit == "" {
		return repository
	}
	u, _ := url.Parse(repository)
	switch u.Hostname() {
	case "github.com":
		return repository + "/tree/" + url.PathEscape(commit)
	case "gitlab.com":
		return repository + "/-/tree/" + url.PathEscape(commit)
	case "codeberg.org":
		return repository + "/src/commit/" + url.PathEscape(commit)
	case "bitbucket.org":
		return repository + "/src/" + url.PathEscape(commit)
	}
	return repository
}

// escape escapes each segment of a slash-separated name for a URL path.
func escape(name string) string {
	segments := strings.Split(name, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}
