package javascript

import (
	"net/url"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A git dependency ("github:owner/repo", "git+ssh://…", "https://…/repo.git") is
// fetched from its repository, not from the registry, and every lock file writes
// down the commit the install checked out:
//
//	package-lock.json v2/v3  "resolved": "git+ssh://git@github.com/o/r.git#<sha>"
//	package-lock.json v1     "version": "git+ssh://git@github.com/o/r.git#<sha>" (or "github:o/r#<sha>")
//	yarn.lock (classic)      resolved "https://codeload.github.com/o/r/tar.gz/<sha>" (or "git+…#<sha>")
//	yarn.lock (Berry)        resolution: "x@https://github.com/o/r.git#commit=<sha>"
//	pnpm-lock.yaml           resolution: {commit: <sha>, repo: …, type: git} or {tarball: https://codeload…}
//	bun.lock                 ["x@git+https://github.com/o/r.git#<sha>", {…}, "<sha>"]
//
// The commit is the version such a package is pinned to, and the repository is
// where it was installed from (Target.Origin): the registry has no say in it, and
// is not asked about it.

// hostedShorthands are the hosts npm's "<host>:owner/repo" shorthand names.
var hostedShorthands = map[string]string{"github": "github.com", "gitlab": "gitlab.com", "bitbucket": "bitbucket.org"}

// gitSource reads where a lock file says a package was fetched from as a git
// dependency: the repository, as a URL without credentials, and the full commit,
// "" when the record names none (a branch, a tag, Bun's abbreviated commit). ok is
// false for anything that is not a git dependency, a registry tarball above all,
// whose URL may end in "#<the tarball's SHA-1>".
//
// Implements: REQ-JS-019
func gitSource(resolved string) (repository, commit string, ok bool) {
	location, reference, _ := strings.Cut(strings.TrimSpace(resolved), "#")
	prefix, rest, _ := strings.Cut(location, ":")
	switch {
	case hostedShorthands[prefix] != "" && !strings.HasPrefix(rest, "//"):
		repository = "https://" + hostedShorthands[prefix] + "/" + strings.TrimSuffix(rest, ".git")
	case strings.HasPrefix(location, "https://codeload.github.com/"):
		// https://codeload.github.com/<owner>/<repository>/tar.gz/<commit>
		segments := strings.Split(strings.TrimPrefix(location, "https://codeload.github.com/"), "/")
		if len(segments) != 4 || segments[2] != "tar.gz" {
			return "", "", false
		}
		repository, reference = "https://github.com/"+segments[0]+"/"+segments[1], segments[3]
	case strings.HasPrefix(location, "git+"), strings.HasPrefix(location, "git://"), strings.HasPrefix(location, "ssh://"):
		repository = withoutCredentials(strings.TrimPrefix(location, "git+"))
	case strings.HasPrefix(location, "git@") && strings.Contains(location, ":"): // git@host:owner/repo.git
		repository = location
	// Yarn Berry writes a GitHub dependency as the repository's HTTPS URL, and its
	// reference as "commit=<sha>" (or "head=<branch>", "tag=<tag>").
	case (strings.HasPrefix(location, "https://") || strings.HasPrefix(location, "http://")) &&
		(strings.HasSuffix(location, ".git") || strings.Contains(reference, "=")):
		repository = withoutCredentials(location)
	default:
		return "", "", false
	}
	commit = reference
	if strings.Contains(reference, "=") {
		values, _ := url.ParseQuery(reference)
		commit = values.Get("commit")
	}
	if !lang.Commit(commit) {
		commit = ""
	}
	return repository, strings.ToLower(commit), repository != ""
}

// withoutCredentials drops what a URL's user information may hold of a secret: a
// password always, and the user of any scheme but SSH, where it is the account
// (git@) and a token is written as a user over HTTPS. An SSH URL in scp form
// behind the scheme (Bun's "ssh://git@host:owner/repo") is written with a path.
func withoutCredentials(location string) string {
	scheme, rest, ok := strings.Cut(location, "://")
	if !ok {
		return location
	}
	authority, path, _ := strings.Cut(rest, "/")
	if user, host, ok := cutLast(authority, "@"); ok {
		user, _, _ = strings.Cut(user, ":")
		authority = host
		if scheme == "ssh" && user != "" {
			authority = user + "@" + host
		}
	}
	if host, owner, ok := strings.Cut(authority, ":"); ok && scheme == "ssh" && owner != "" && !startsWithDigit(owner) {
		authority, path = host, owner+"/"+path
	}
	return scheme + "://" + authority + "/" + path
}

// cutLast is strings.Cut at the last separator.
func cutLast(s, separator string) (before, after string, found bool) {
	if i := strings.LastIndex(s, separator); i >= 0 {
		return s[:i], s[i+len(separator):], true
	}
	return s, "", false
}

// addGit records that each of names at version (a commit, or "" when the lock
// names none) is a git dependency on repository; the first lock file to say
// keeps it.
func (t *tree) addGit(names []string, version, repository string) {
	for _, n := range names {
		if _, ok := t.git[n+"@"+version]; !ok && n != "" && repository != "" {
			t.git[n+"@"+version] = repository
		}
	}
}

// origin is the repository a package at version was installed from, where a
// lock file installed it from one: "" for a registry package.
//
// Implements: REQ-JS-019
func (t *tree) origin(packageName, version string) string {
	return t.git[packageName+"@"+version]
}

// unpinnedNote is the sentence that reports the git dependencies a lock file
// names no commit for.
func unpinnedNote(names []string) string {
	noun := "dependencies"
	if len(names) == 1 {
		noun = "dependency"
	}
	return "the lock file names no commit for the git " + noun + " " + strings.Join(names, ", ") +
		" (only a branch or a tag): not pinned, and not asked about by commit"
}
