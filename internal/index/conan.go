package index

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// cSpell: ignore conanrc fnmatchcase

// ---------------------------------------------------------------- remotes

// ConanCenter reports whether a Conan remote is ConanCenter: center2.conan.io,
// or center.conan.io, which older configurations name (it serves the recipes
// ConanCenter had when it was frozen). It is the public index.
//
// Implements: REQ-SUP-076
func ConanCenter(remote string) bool {
	u, err := url.Parse(strings.TrimSpace(remote))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "center2.conan.io" || host == "center.conan.io"
}

// parseConanRemotes reads a Conan home's remotes.json, `{"remotes": [{"name",
// "url", "verify_ssl", "disabled", "allowed_packages", "remote_type"}]}`: Conan
// asks its remotes in order, the first that has a recipe answering, and no
// other, so they are Listed and ConanCenter is asked only when it is one of them.
// A disabled remote and a local-recipes-index (a folder, not a server) are left
// out. A file Conan cannot read names nothing; a home without one has
// ConanCenter alone, which Conan writes into it.
//
// Implements: REQ-SUP-076
func parseConanRemotes(data []byte, k sink) {
	var doc struct {
		Remotes []struct {
			URL             string   `json:"url"`
			Disabled        bool     `json:"disabled"`
			AllowedPackages []string `json:"allowed_packages"`
			RemoteType      string   `json:"remote_type"`
		} `json:"remotes"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	k.off(Conan)
	for _, r := range doc.Remotes {
		if r.Disabled || r.RemoteType == "local-recipes-index" || !strings.Contains(r.URL, "://") {
			continue
		}
		k.put(Conan, Source{URL: r.URL, Kind: Listed, conanAllowed: r.AllowedPackages})
	}
}

// projectConan reads the remotes of the Conan home a repository's .conanrc
// names (`conan_home=./.conan2`). Conan looks for a .conanrc from the directory
// it runs in upwards and takes its home over CONAN_HOME; here, from each
// conanfile's directory up to the repository's root, from the disk (the home is
// usually git-ignored). Whatever the home is - inside the repository or not -
// the repository chose it: its remotes are the repository's (one this machine's
// own remotes.json lists too is recorded, and trusted, as this machine's, which
// is read first), and its credentials.json is not read. A home with no
// remotes.json has ConanCenter.
//
// Implements: REQ-SUP-076
func (c *Config) projectConan(files []*scan.File, k sink) {
	seen := map[string]bool{}
	for _, f := range files {
		if base := path.Base(f.Path); base != "conanfile.txt" && base != "conanfile.py" {
			continue
		}
		root := filepath.Clean(strings.TrimSuffix(f.AbsolutePath, filepath.FromSlash(f.Path)))
		home := conanrcHome(filepath.Dir(f.AbsolutePath), root, c.m.Home)
		if home == "" || seen[home] {
			continue
		}
		seen[home] = true
		if data, err := os.ReadFile(filepath.Join(home, "remotes.json")); err == nil {
			parseConanRemotes(data, k)
		}
	}
}

// conanrcHome is the Conan home the first .conanrc from directory up to root
// names, as Conan reads the file: `conan_home=<value>` lines, a value starting
// with ./, .\ or .. relative to the .conanrc, ~ the user's home, else absolute.
// "" when there is none, or the first .conanrc names none (Conan then takes
// CONAN_HOME's).
func conanrcHome(directory, root, userHome string) string {
	for {
		data, err := os.ReadFile(filepath.Join(directory, ".conanrc"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				key, value, ok := strings.Cut(strings.TrimRight(line, "\r"), "=")
				if !ok || strings.HasPrefix(line, "#") || key != "conan_home" {
					continue
				}
				value = strings.TrimSpace(value)
				switch {
				case strings.HasPrefix(value, "./") || strings.HasPrefix(value, `.\`) || strings.HasPrefix(value, ".."):
					return filepath.Join(directory, filepath.FromSlash(value))
				case value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`):
					if userHome == "" {
						return ""
					}
					return filepath.Join(userHome, strings.TrimLeft(value[1:], `/\`))
				case filepath.IsAbs(value):
					return value
				}
				return ""
			}
			return ""
		}
		if directory == root || filepath.Dir(directory) == directory || !strings.HasPrefix(directory, root) {
			return ""
		}
		directory = filepath.Dir(directory)
	}
}

// conanCandidates are the remotes a Conan package is asked of: those candidates
// lists, less each remote whose allowed_packages no pattern of admits the
// reference (Conan's own filter), the version "*" for a range, as Conan searches
// with it, and for no version.
//
// Implements: REQ-SUP-076
func (c *Config) conanCandidates(t lang.Target) []candidate {
	if t.Version == "" || strings.HasPrefix(t.Version, "[") {
		t.Version = "*"
	}
	reference, ok := conanTargetReference(t)
	if !ok {
		return nil
	}
	return slices.DeleteFunc(c.candidates(Conan, t.Package, ""), func(k candidate) bool {
		for _, s := range c.sources[Conan] {
			if s.URL == k.url && len(s.conanAllowed) > 0 {
				return !slices.ContainsFunc(s.conanAllowed, func(pattern string) bool { return conanMatches(pattern, reference) })
			}
		}
		return false
	})
}

// conanTargetReference is the Conan reference a target names: its package and
// version, with the user, channel and revision its lang.Target.Registry holds.
func conanTargetReference(t lang.Target) (cpp.ConanReference, bool) {
	if t.Version == "" {
		return cpp.ConanReference{}, false
	}
	return cpp.ParseConanReference(t.Package + "/" + t.Version + t.Registry)
}

// conanMatches reports whether an allowed_packages pattern admits a reference, as
// Conan matches one: fnmatch against name/version[@user[/channel]] or the same
// with #revision, "!" or "~" before it negating, "@" at its end (or "@#" in it)
// requiring no user and channel.
func conanMatches(pattern string, r cpp.ConanReference) bool {
	negate := strings.HasPrefix(pattern, "!") || strings.HasPrefix(pattern, "~")
	if negate {
		pattern = pattern[1:]
	}
	bare := false
	if strings.HasSuffix(pattern, "@") {
		pattern, bare = strings.TrimSuffix(pattern, "@"), true
	} else if strings.Contains(pattern, "@#") {
		pattern, bare = strings.ReplaceAll(pattern, "@#", "#"), true
	}
	plain := r
	plain.Revision = ""
	short := plain.Qualifier()
	condition := fnmatch(pattern, r.Name+"/"+r.Version+short) ||
		fnmatch(pattern, r.Name+"/"+r.Version+r.Qualifier())
	if bare {
		condition = condition && r.User == "" && r.Channel == ""
	}
	return condition != negate
}

// fnmatch is Python's fnmatch.fnmatchcase: * is any run of characters, / too,
// ? one character, [...] a set ([!...] its complement).
func fnmatch(pattern, name string) bool {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			end := strings.IndexByte(pattern[i+1:], ']')
			if end < 0 {
				b.WriteString(`\[`)
				continue
			}
			set := pattern[i+1 : i+1+end]
			if strings.HasPrefix(set, "!") {
				set = "^" + set[1:]
			}
			b.WriteString("[" + strings.ReplaceAll(set, `\`, `\\`) + "]")
			i += end + 1
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	matched, err := regexp.MatchString(b.String(), name)
	return err == nil && matched
}

// ---------------------------------------------------------------- the lock

// readConanLock records the references a repository's conan.lock pins, by
// name, the first lock (by path) to pin one winning.
//
// Implements: REQ-SUP-076
func (c *Config) readConanLock(data []byte) {
	for _, r := range cpp.ConanLockReferences(data) {
		if c.conanLocks == nil {
			c.conanLocks = map[string]cpp.ConanReference{}
		}
		if _, taken := c.conanLocks[r.Name]; !taken {
			c.conanLocks[r.Name] = r
		}
	}
}

// conanPinned is a Conan package as the repository's conan.lock pins it, which
// Conan installs in place of what was required: the lock's version and revision
// when the lock pins the name (with the same user and channel) at the version
// required or at one the range required admits. Anything else is left as it is.
//
// Implements: REQ-SUP-076, REQ-CPP-011
func (c *Config) conanPinned(t lang.Target) lang.Target {
	r, ok := conanTargetReference(t)
	if !ok {
		return t
	}
	locked, ok := c.conanLocks[r.Name]
	if !ok || locked.User != r.User || locked.Channel != r.Channel ||
		!(strings.HasPrefix(r.Version, "[") && conanAdmits(r.Version, locked.Version) || conanCompare(r.Version, locked.Version) == 0) {
		return t
	}
	if t.Version != locked.Version {
		t.Requested = cmp.Or(t.Requested, t.Version)
	}
	t.Version, t.Registry, t.Pinned = locked.Version, locked.Qualifier(), true
	return t
}

// ---------------------------------------------------------------- the client

// conanRecipe reads a Conan package's dependencies from a remote through Conan's
// REST API v2, as Conan reads a recipe: a version range is resolved against the
// remote's `GET /v2/conans/search?q=<name>/*[@user/channel]` (the newest version
// the range admits, pre-releases only when it says include_prerelease); a
// revision the reference pins (as the repository's conan.lock does) is read as it is,
// else `GET /v2/conans/<name>/<version>/<user>/<channel>/latest` names the
// newest (user and channel "_" for none); then
// `.../revisions/<revision>/files/conanfile.py` is the recipe. Its `requires` -
// the attribute's strings and self.requires() calls, as the cpp plugin reads
// them - are the answer, which Client.targets pins as the repository's conan.lock
// pins them (conanPinned). Tool, build and test requirements are not a
// package's dependencies.
//
// Implements: REQ-SUP-076
func (c *Client) conanRecipe(ctx context.Context, remote string, t lang.Target) ([]dependency, error) {
	r, ok := conanTargetReference(t)
	if !ok {
		return nil, fmt.Errorf("%w: %s is no Conan reference", errAbsent, t.Package)
	}
	base := strings.TrimRight(remote, "/") + "/v2/conans/"
	if strings.HasPrefix(r.Version, "[") {
		version, err := c.conanResolve(ctx, remote, r)
		if err != nil {
			return nil, err
		}
		r.Version, r.Revision = version, "" // Conan ignores a revision beside a range
	}
	recipe := base + strings.Join([]string{url.PathEscape(r.Name), url.PathEscape(r.Version),
		url.PathEscape(cmp.Or(r.User, "_")), url.PathEscape(cmp.Or(r.Channel, "_"))}, "/")
	revision := r.Revision
	if revision == "" {
		body, err := c.conanGet(ctx, remote, recipe+"/latest", "application/json")
		if err != nil {
			return nil, err
		}
		var latest struct {
			Revision string `json:"revision"`
		}
		if err := json.Unmarshal(body, &latest); err != nil {
			return nil, err
		}
		if revision = latest.Revision; revision == "" {
			return nil, fmt.Errorf("%w: %s names no revision of %s/%s", errAbsent, remote, r.Name, r.Version)
		}
	}
	body, err := c.conanGet(ctx, remote, recipe+"/revisions/"+url.PathEscape(revision)+"/files/conanfile.py", "*/*")
	if err != nil {
		return nil, err
	}
	var out []dependency
	seen := map[string]bool{r.Name: true}
	for _, requirement := range cpp.RecipeRequirements(body) {
		d := requirement.Reference
		if requirement.Kind != "requires" || seen[d.Name] {
			continue
		}
		seen[d.Name] = true
		out = append(out, dependency{Name: d.Name, Version: d.Version, Registry: d.Qualifier()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// conanResolve is the newest version of a recipe a range admits on a remote, as
// Conan's range resolver reads the remote's search; a remote that lists none the
// range admits does not have the package.
func (c *Client) conanResolve(ctx context.Context, remote string, r cpp.ConanReference) (string, error) {
	pattern := r.Name + "/*"
	if r.User != "" {
		pattern += "@" + r.User + "/" + cmp.Or(r.Channel, "_")
	}
	body, err := c.conanGet(ctx, remote, strings.TrimRight(remote, "/")+"/v2/conans/search?"+url.Values{"q": {pattern}}.Encode(), "application/json")
	if err != nil {
		return "", err
	}
	var found struct {
		Results []string `json:"results"`
	}
	if err := json.Unmarshal(body, &found); err != nil {
		return "", err
	}
	best := ""
	for _, result := range found.Results {
		v, ok := cpp.ParseConanReference(result)
		if !ok || v.Name != r.Name || v.User != r.User || v.Channel != r.Channel || !conanAdmits(r.Version, v.Version) {
			continue
		}
		if best == "" || conanCompare(v.Version, best) > 0 {
			best = v.Version
		}
	}
	if best == "" {
		return "", fmt.Errorf("%w: %s lists no version of %s in %s", errAbsent, remote, r.Name, r.Version)
	}
	return best, nil
}

// conanGet asks a remote as Conan does: without a token first, and, when the
// remote answers 401 and this machine holds a login for it (auth.ConanLogin), with
// the token its `GET /v2/users/authenticate` hands back for that login (Basic),
// as a Bearer token. The token is asked for once per remote. A login goes only to
// a remote reached over https or on this machine. A remote that wants a login
// this machine does not have while Conan's auth_remote.py plugin could supply one
// is noted: the plugin is not run.
//
// Implements: REQ-SUP-076, REQ-AUTH-035, REQ-TRC-017
func (c *Client) conanGet(ctx context.Context, remote, address, media string) ([]byte, error) {
	if token, ok := c.conanTokens.known(remote); ok {
		return c.bearerGet(ctx, address, media, token)
	}
	body, err := c.accept(ctx, address, media)
	var status *statusError
	if !errors.As(err, &status) || status.code != http.StatusUnauthorized {
		return body, err
	}
	user, password, ok := c.auth.ConanLogin(remote)
	if !ok {
		if plugin := c.auth.ConanAuthPlugin(); plugin != "" {
			c.note(trace.NoteHelperNotRun, "Conan remote "+remote+" wants a login, and Conan's auth_remote.py plugin ("+plugin+
				") that would supply it is a program, which is not run: credentials.json or CONAN_LOGIN_USERNAME and "+
				"CONAN_PASSWORD provide one")
		}
		return nil, err
	}
	u, parseErr := url.Parse(remote)
	if parseErr != nil || u.Scheme != "https" && !loopbackHost(u.Hostname()) {
		return nil, err
	}
	token, err := c.conanTokens.get(remote, func() (string, error) {
		body, err := c.basicGet(ctx, strings.TrimRight(remote, "/")+"/v2/users/authenticate", user, password)
		return strings.TrimSpace(string(body)), err
	})
	if err != nil {
		return nil, err
	}
	return c.bearerGet(ctx, address, media, token)
}

// bearerGet is accept with a token in place of this machine's credentials.
func (c *Client) bearerGet(ctx context.Context, address, media, token string) ([]byte, error) {
	response, err := c.do(ctx, address, media, token)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &statusError{url: address, status: response.Status, code: response.StatusCode}
	}
	return readLimited(response)
}

// basicGet asks for address with a user and password as Basic credentials and
// nothing of this machine's own: what it sends is the credential (see conanGet).
//
// Implements: REQ-AUTH-035
func (c *Client) basicGet(ctx context.Context, address, user, password string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(user, password)
	response, err := c.send(ctx, request, address)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &statusError{url: address, status: response.Status, code: response.StatusCode}
	}
	return readLimited(response)
}

// ---------------------------------------------------------------- versions

// conanVersion is a Conan version split as Conan compares it: the dotted items
// of its main part, and those of its pre-release (after "-"); a build (after
// "+") is left out.
type conanVersion struct{ main, prerelease []string }

func parseConanVersion(v string) conanVersion {
	v, _, _ = strings.Cut(strings.TrimSpace(v), "+")
	main, prerelease, hasPrerelease := strings.Cut(v, "-")
	out := conanVersion{main: strings.Split(main, ".")}
	if hasPrerelease {
		out.prerelease = strings.Split(prerelease, ".")
	}
	return out
}

// conanCompareItems compares dotted items as Conan does: numbers as numbers,
// anything else as text, a missing item as 0.
func conanCompareItems(a, b []string) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := "0", "0"
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		xn, xErr := strconv.Atoi(x)
		yn, yErr := strconv.Atoi(y)
		switch {
		case xErr == nil && yErr == nil:
			if n := cmp.Compare(xn, yn); n != 0 {
				return n
			}
		default:
			if n := strings.Compare(x, y); n != 0 {
				return n
			}
		}
	}
	return 0
}

// conanCompare orders two Conan versions; a pre-release comes before the release.
func conanCompare(a, b string) int {
	x, y := parseConanVersion(a), parseConanVersion(b)
	if n := conanCompareItems(x.main, y.main); n != 0 {
		return n
	}
	switch {
	case x.prerelease == nil && y.prerelease == nil:
		return 0
	case x.prerelease == nil:
		return 1
	case y.prerelease == nil:
		return -1
	}
	return conanCompareItems(x.prerelease, y.prerelease)
}

// conanUpperBound is Conan's Version.upper_bound: the first index+1 items, the
// last one bumped, as the lowest pre-release of that version.
func conanUpperBound(v conanVersion, index int) string {
	if index >= len(v.main) {
		index = len(v.main) - 1
	}
	items := slices.Clone(v.main[:index+1])
	n, err := strconv.Atoi(items[index])
	if err != nil {
		return ""
	}
	items[index] = strconv.Itoa(n + 1)
	return strings.Join(items, ".") + "-"
}

// conanAdmits reports whether a version is in a Conan version range, as Conan 2
// reads one: "[>=1.0 <2]" (conditions all hold), alternatives separated by "||",
// "~1.2" (up to the next minor, or the next major for "~1"), "^1.2" (up to the
// next change of the first non-zero item), "1.2.*", "*", a bare version (that
// one), and ", include_prerelease" after them, without which no pre-release is
// admitted.
func conanAdmits(expression, version string) bool {
	expression = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(expression), "["), "]")
	expression, options, _ := strings.Cut(expression, ",")
	prerelease := strings.Contains(options, "include_prerelease")
	v := parseConanVersion(version)
	if v.prerelease != nil && !prerelease {
		return false
	}
	for _, alternative := range strings.Split(expression, "||") {
		if conanConditionsHold(strings.Fields(alternative), version) {
			return true
		}
	}
	return false
}

// conanConditionsHold reports whether a version meets every condition of one
// alternative of a range.
func conanConditionsHold(conditions []string, version string) bool {
	for _, condition := range conditions {
		if condition == "*" {
			continue
		}
		operator := ""
		for _, candidate := range []string{">=", "<=", ">", "<", "=", "~", "^"} {
			if strings.HasPrefix(condition, candidate) {
				operator = candidate
				break
			}
		}
		operand := strings.TrimPrefix(condition, operator)
		if operand == "" {
			return false
		}
		if (operator == ">=" || operator == "<") && !strings.ContainsAny(operand, "-+") {
			operand += "-" // from and below the lowest pre-release, as Conan bounds them
		}
		n := conanCompare(version, operand)
		switch operator {
		case ">=":
			if n < 0 {
				return false
			}
		case "<=":
			if n > 0 {
				return false
			}
		case ">":
			if n <= 0 {
				return false
			}
		case "<":
			if n >= 0 {
				return false
			}
		case "~", "^":
			low := parseConanVersion(operand)
			index := 0
			if operator == "~" && len(low.main) > 1 {
				index = 1
			}
			if operator == "^" {
				index = len(low.main) - 1
				for i, item := range low.main {
					if item != "0" {
						index = i
						break
					}
				}
			}
			high := conanUpperBound(low, index)
			if n < 0 || high == "" || conanCompare(version, high) >= 0 {
				return false
			}
		default:
			if strings.HasSuffix(operand, "*") {
				if !strings.HasPrefix(version, strings.TrimSuffix(operand, "*")) {
					return false
				}
			} else if n != 0 {
				return false
			}
		}
	}
	return true
}
