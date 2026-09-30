package index

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/ada"
	"github.com/sarumaj/depphunter-cli/internal/lang/opam"
	"github.com/sarumaj/depphunter-cli/internal/store"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// Client asks package indexes what a package depends on, for the ecosystems whose
// answer is not already in the repository. It is used only with --online, and only
// against indexes this machine's own configuration names: an index a repository
// points at is never fetched from, however plainly it asks.
type Client struct {
	config  *Config
	http    *http.Client
	cache   *store.Store
	auth    *auth.Store
	timeout time.Duration

	mu   sync.Mutex
	seen map[string][]lang.Target // answers already given, including empty ones
	// failed is when each question the index would not answer was last asked. It is
	// asked again once failRetry has passed: one client serves every re-analysis in
	// --watch, so a failure kept as an answer would last as long as the process.
	failed map[string]time.Time
	// feeds is what a NuGet service index resolved to: the same answer for every
	// package on that feed, and one request rather than one per package.
	feeds map[string]string
	// repositories is what the PACKAGES file of each CRAN-like repository lists: one
	// request per repository rather than one per package.
	repositories map[string]map[string][]dependency
	// rocks is what each rocks server's manifest lists: rock -> versions, one
	// request per server rather than one per rock.
	rocks map[string]map[string][]string
	// cpanModules is which distribution provides each module, per MetaCPAN API: a
	// release lists the modules it requires, and the map's packages are
	// distributions.
	cpanModules map[string]string
	// cpanMirrors is what each CPAN mirror's 02packages lists, read once.
	cpanMirrors memo[*cpanPackages]
	// opamCopies are the repository copies opam keeps, an archive read once.
	opamCopies memo[*opamCopy]
	// listings are directories of public index repositories GitHub listed.
	listings memo[[]string]
	// juliaRegistries is where each package's files are in a Julia registry
	// other than General, from its Registry.toml: read once per registry.
	juliaRegistries memo[*juliaPaths]
	// juliaCopies are the registries installed in a Julia depot, an archive
	// read once.
	juliaCopies memo[*juliaCopy]
	// wallyConfigs are the config.json files of Wally registries, read once.
	wallyConfigs memo[*wallyConfiguration]
	// conanTokens are the tokens Conan remotes handed out for this machine's
	// logins, asked for once per remote (see conanGet).
	conanTokens memo[string]
	// flatPages are the files of each Python flat index, read once (see pypiFlat).
	flatPages memo[[]simpleFile]
	// qlSystems is what each Quicklisp dist's system index lists: one request
	// per dist version rather than one per project.
	qlSystems map[string]*qlIndex
	// qlFailed is each dist whose distinfo or system index could not be read, with
	// when and why: every project of the dist is answered with that failure until
	// failRetry has passed, instead of asking the dist again for each one.
	qlFailed map[string]qlFailure
	// located is where each package asked about was found (see Located).
	located map[string]located
	// resolutionReport is the report this run is writing, if anybody is reading it. It is set per
	// analysis - one client serves every re-analysis in --watch - so it is guarded
	// like the rest.
	resolutionReport *trace.Report
}

// Trace points the client at the report of the analysis now running. Passing nil
// stops it recording. internal/analyze calls this; nothing else needs to.
func (c *Client) Trace(r *trace.Report) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resolutionReport = r
}

// report records one question, if anybody is listening.
func (c *Client) report(l trace.Lookup) {
	c.mu.Lock()
	r := c.resolutionReport
	c.mu.Unlock()
	r.Add(l)
}

// note records what the report should say once rather than per question, if anybody
// is listening. The plugin is the walk's.
//
// Implements: REQ-TRC-017
func (c *Client) note(code, message string) {
	c.mu.Lock()
	r := c.resolutionReport
	c.mu.Unlock()
	r.Note(trace.Note{Code: code, Message: message})
}

// NewClient prepares the client. directory holds the cached answers; ttl is how long one
// stays usable; credentials are what this machine holds for its indexes. The packages
// that must not be asked of a public index are the configuration's (Config.Private).
func NewClient(config *Config, directory string, ttl, timeout time.Duration, credentials *auth.Store) *Client {
	return &Client{
		config:       config,
		http:         &http.Client{Timeout: timeout},
		cache:        store.New(directory, ttl),
		auth:         credentials,
		timeout:      timeout,
		seen:         map[string][]lang.Target{},
		failed:       map[string]time.Time{},
		feeds:        map[string]string{},
		repositories: map[string]map[string][]dependency{},
		rocks:        map[string]map[string][]string{},
		cpanModules:  map[string]string{},
		qlSystems:    map[string]*qlIndex{},
		qlFailed:     map[string]qlFailure{},
		located:      map[string]located{},
	}
}

// Dependencies implements lang.Transitive against the indexes. Every way out is
// recorded (internal/trace), because a question declined for what asking would
// disclose is indistinguishable on the map from a dependency that genuinely has none.
//
// Implements: REQ-SUP-019, REQ-SUP-038, REQ-SUP-039, REQ-TRC-005, REQ-TRC-006
func (c *Client) Dependencies(t lang.Target) []lang.Target {
	if c == nil || t.Package == "" {
		return nil
	}
	if t.Ecosystem == Conan {
		t = c.config.conanPinned(t) // what Conan installs is what its lock pins
	}
	l := trace.Lookup{Ecosystem: t.Ecosystem, Package: t.Package, Version: t.Version, Answer: trace.NoAnswer}
	// A package installed from a directory, an archive or a repository is on no index:
	// a question about it could only name it to somebody.
	// Implements: REQ-PY-015
	if t.Origin != "" {
		l.Reason = trace.ReasonInstalled
		c.report(l)
		return nil
	}
	candidates := c.config.candidatesFor(t)
	// Implements: REQ-SUP-065, REQ-TRC-017
	if t.Ecosystem == NuGet && c.config.nugetUnmapped(t.Package) {
		c.note(trace.NoteUnmapped, "NuGet.Config's packageSourceMapping covers no pattern of "+t.Package+
			": NuGet itself would not restore it (NU1100), depphunter asks the sources in their usual order")
	}
	// An organization's own package is not named to the world: asking the public
	// index about corp.example/billing would not answer anyway, and the request
	// itself is the disclosure. A private registry this machine configures is asked
	// as usual, whether it replaces the public index or is asked beside it.
	if kept, owned := c.config.nameable(t, candidates); owned {
		if len(kept) == 0 && len(candidates) > 0 {
			l.Index, l.Reason = candidates[0].url, trace.ReasonPrivate
			c.report(l)
			return nil
		}
		candidates = kept
	}
	if len(candidates) == 0 {
		l.Reason = trace.ReasonNoIndex
		// Implements: REQ-SUP-068
		if t.Ecosystem == OCI {
			if _, blocked := c.config.ociEndpoints(t.Package, t.Version); blocked {
				l.Reason = trace.ReasonBlocked
			}
		}
		c.report(l)
		return nil
	}
	// An index only the repository asks for is not fetched from, but it is not in
	// the way either: the package is asked of the others, and only when none of
	// them has it is the repository's index what it would have come from.
	var untrusted string
	var asked []candidate
	for _, k := range candidates {
		if k.known {
			asked = append(asked, k)
		} else if untrusted == "" {
			untrusted = k.url
		}
	}
	// A private organization's package answers only to its key: asked without one,
	// hex.pm would say "not found", which reads as a package that does not exist -
	// and a later repository in rebar3's order would be asked about a name the
	// organization's may hold. The question stops at the first organization this
	// machine has no key for: nobody after it is asked, and when nobody before it
	// has the package, the report says what is missing.
	// Implements: REQ-SUP-047, REQ-BEAM-013
	var noKey string
	for i, k := range asked {
		if k.keyed && !c.auth.Authorizes(k.url+"/packages/"+url.PathEscape(t.Package)) {
			noKey, asked = k.url, asked[:i]
			break
		}
	}
	if noKey != "" {
		c.note(trace.NoteNoKey, noHexKey(noKey))
	}
	if noKey != "" && len(asked) == 0 {
		l.Index, l.Reason = noKey, trace.ReasonNoKey
		c.report(l)
		return nil
	}
	if len(asked) == 0 {
		l.Index, l.Reason = untrusted, trace.ReasonUntrusted
		c.locate(t, untrusted, false)
		c.report(l)
		return nil
	}
	// Implements: REQ-SUP-066, REQ-TRC-017
	if t.Ecosystem == PyPI && len(asked) > 1 {
		if why := c.config.pythonMerges(); why != "" {
			c.note(trace.NoteMerged, why)
		}
	}
	if t.Ecosystem == Bazel {
		for _, k := range asked {
			if u, err := url.Parse(k.url); err == nil {
				if scope, ok := c.config.bazelHelperFor(u.Hostname()); ok {
					// Implements: REQ-BAZEL-011, REQ-TRC-017
					c.note(trace.NoteHelperNotRun, bazelHelperNote(k.url, scope))
				}
			}
		}
	}
	key := t.Ecosystem + " " + t.Package + "@" + t.Version + " " + t.Registry
	c.mu.Lock()
	cached, ok := c.seen[key]
	if at, failed := c.failed[key]; !ok && failed && time.Since(at) < failRetry {
		ok = true // not asked again so soon; the lookup that failed was reported
	}
	c.mu.Unlock()
	if ok {
		l.Index = asked[0].url
		if at, _, found := c.Located(t.Ecosystem, t.Package); found {
			l.Index = at
		}
		l.Answer, l.Dependencies = trace.FromMemo, len(cached)
		c.report(l)
		return cached
	}

	start := time.Now()
	a, index, err := c.ask(t, asked)
	l.Index = index
	switch {
	case err == nil:
		c.locate(t, index, true)
	case notFound(err) && noKey != "":
		a = answer{source: trace.NoAnswer, reason: trace.ReasonNoKey, requests: a.requests}
		l.Index = noKey
	case notFound(err) && untrusted != "":
		// Nowhere this machine may ask has it: it can only have come from the
		// index the repository names, which is what the map and the report say.
		// It is kept as a failure, not an answer, so that it is asked again once
		// the repository stops naming that index.
		a = answer{source: trace.NoAnswer, reason: trace.ReasonUntrusted, requests: a.requests}
		l.Index = untrusted
		c.locate(t, untrusted, false)
	default:
		// An index that will not answer is not an error the map can use - but it is
		// the whole of what the report has to say about this package.
		a = answer{source: trace.NoAnswer, reason: err.Error(), requests: a.requests}
	}
	l.Answer, l.Dependencies, l.Reason = a.source, len(a.dependencies), a.reason
	l.Requests, l.Millis = a.requests, time.Since(start).Milliseconds()
	c.report(l)
	c.mu.Lock()
	if err != nil {
		c.failed[key] = time.Now()
	} else {
		delete(c.failed, key)
		c.seen[key] = a.dependencies
	}
	c.mu.Unlock()
	return a.dependencies
}

// ask puts the question to each index in turn and returns the first answer, with the
// index that gave it. An index that does not have the package (404, 410, or an
// answer that does not list it) passes the question on; any other failure ends it,
// unless the index is one a failure of any kind moves on from (GOPROXY's "|"): an
// index that is down is not the same as one that said no, and the next one's answer
// might be a different package of the same name. A cached answer from any of them
// is taken before anything is sent.
//
// Implements: REQ-SUP-063
func (c *Client) ask(t lang.Target, asked []candidate) (answer, string, error) {
	for _, k := range asked {
		if a, ok := c.cached(t, k.url); ok {
			return a, k.url, nil
		}
	}
	var requests []trace.Request
	var err error
	index := ""
	for _, k := range asked {
		var a answer
		index = k.url
		a, err = c.lookup(t, k.url)
		if k.keyed && forbidden(err) {
			// Implements: REQ-SUP-047, REQ-TRC-017
			c.note(trace.NoteForbidden, "Hex organization "+path.Base(k.url)+" refused the key sent to "+k.url+
				" (403): a key made by `mix hex.organization auth` without `--key` holds only the repository "+
				"permission, and the API needs api:read - use a user key or one generated with --permission api:read")
		}
		requests = append(requests, a.requests...)
		a.requests = requests
		if err == nil {
			return a, index, nil
		}
		if !notFound(err) && !k.onError {
			return answer{requests: requests}, index, err
		}
	}
	return answer{requests: requests}, index, err
}

// locate records which index a package was found on, or - when only an index the
// repository names can have it - that one, for Located.
func (c *Client) locate(t lang.Target, index string, known bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.located[t.Ecosystem+" "+t.Package] = located{index, known}
}

type located struct {
	index string
	known bool
}

// Located reports where a package asked about was found: the index that answered,
// or the index only the repository names when nothing this machine may ask had it
// (known false). ok is false for a package not asked, or asked without an answer
// either way. internal/analyze puts it on the map in place of the attribution made
// before anything was asked (Config.ForTarget), which cannot know which of several
// indexes has a package.
//
// Implements: REQ-SUP-018, REQ-SUP-063
func (c *Client) Located(ecosystem, packageName string) (index string, known, ok bool) {
	if c == nil {
		return "", false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.located[ecosystem+" "+packageName]
	return l.index, l.known, ok
}

// failRetry is how long a question an index did not answer is left before it is
// asked again.
//
// Implements: REQ-SUP-032
const failRetry = 5 * time.Minute

// answer is what one question came to: the dependencies, who provided them, and -
// when nobody did - why, with whatever went over the network on the way.
type answer struct {
	dependencies []lang.Target
	source       trace.Answer
	reason       string
	requests     []trace.Request
}

// cacheKey is where an index's answer about a package is kept on disk. A Julia
// package is its UUID: two registries' packages of one name are two packages.
func cacheKey(t lang.Target, index string) string {
	key := t.Ecosystem + "|" + index + "|" + t.Package + "|" + t.Version
	if t.Ecosystem == Julia && t.Registry != "" {
		key += "|" + strings.ToLower(t.Registry)
	}
	if t.Ecosystem == Conan && t.Registry != "" {
		key += "|" + t.Registry // the user, channel and revision name another recipe
	}
	if t.Ecosystem == Actions && t.Registry != "" {
		key += "|" + t.Registry // another action or workflow of the same repository
	}
	return key
}

// cached is an index's answer from the disk cache, if it has one.
func (c *Client) cached(t lang.Target, index string) (answer, bool) {
	dependencies, ok := store.Get[[]dependency](c.cache, cacheKey(t, index))
	if !ok {
		return answer{}, false
	}
	return answer{dependencies: c.targets(t, dependencies), source: trace.FromCache}, true
}

// lookup answers from the cache when it can, and from the index when it must.
//
// Implements: REQ-SUP-020, REQ-SUP-029, REQ-SUP-032, REQ-TRC-006
func (c *Client) lookup(t lang.Target, index string) (answer, error) {
	key := cacheKey(t, index)
	if a, ok := c.cached(t, index); ok {
		return a, nil
	}
	if t.Ecosystem == Buf && t.Registry == BufPlugin {
		// Implements: REQ-SUP-072
		return answer{source: trace.NoAnswer, reason: trace.ReasonPlugin}, nil
	}
	if (t.Ecosystem == Go || t.Ecosystem == CUE || t.Ecosystem == Conan || t.Ecosystem == Actions) && t.Version == "" ||
		t.Ecosystem == Opam && !opam.ExactVersion(t.Version) && c.unlisted(Opam, index) ||
		t.Ecosystem == Alire && !alireExact(t.Version) && (!ada.ValidConstraint(t.Version) || c.unlisted(Alire, index)) {
		// A module proxy serves a go.mod for one version, a CUE registry a
		// module.cue, GitHub an action.yml at one reference; without one there
		// is no document to ask for. An opam repository or Alire index served over HTTP
		// that cannot list its versions has none for a range either. Said here
		// rather than deeper down so the report can say it, instead of recording
		// an empty answer that looks like "no dependencies".
		return answer{source: trace.NoAnswer, reason: trace.ReasonNoVersion}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	// Every request this question makes is collected, so the report can say which of
	// an image's three round trips was the one that failed.
	made := &requestLog{}
	ctx = context.WithValue(ctx, requestLogKey{}, made)

	var dependencies []dependency
	var err error
	switch t.Ecosystem {
	case Go:
		dependencies, err = c.goModule(context.WithValue(ctx, goRequestKey{}, true), index, t)
	case NPM:
		dependencies, err = c.npmPackage(ctx, index, t)
	case PyPI:
		dependencies, err = c.pypiDistribution(ctx, index, t)
	case Cargo:
		dependencies, err = c.cargoCrate(ctx, index, t)
	case NuGet:
		dependencies, err = c.nugetPackage(ctx, index, t)
	case OCI:
		dependencies, err = c.ociBase(ctx, index, t)
	case Composer:
		dependencies, err = c.composerPackage(ctx, index, t)
	case RubyGems:
		dependencies, err = c.rubygemsPackage(ctx, index, t)
	case Pub:
		dependencies, err = c.pubPackage(ctx, index, t)
	case Hex:
		dependencies, err = c.hexPackage(ctx, index, t)
	case CRAN:
		dependencies, err = c.cranPackage(ctx, index, t)
	case Bioconductor:
		dependencies, err = c.bioconductorPackage(ctx, index, t)
	case Hackage:
		dependencies, err = c.hackagePackage(ctx, index, t)
	case TerraformModule:
		dependencies, err = c.terraformModule(ctx, index, t)
	case CocoaPods:
		dependencies, err = c.cocoapodsPod(ctx, index, t)
	case LuaRocks:
		dependencies, err = c.luarocksRock(ctx, index, t)
	case CPAN:
		dependencies, err = c.cpanDistribution(ctx, index, t)
	case Opam:
		dependencies, err = c.opamPackage(ctx, index, t)
	case Julia:
		dependencies, err = c.juliaPackage(ctx, index, t)
	case Bazel:
		dependencies, err = c.bazelModule(ctx, index, t)
	case Elm:
		dependencies, err = c.elmPackage(ctx, index, t)
	case PureScript:
		dependencies, err = c.purescriptPackage(ctx, index, t)
	case Dub:
		dependencies, err = c.dubPackage(ctx, index, t)
	case Alire:
		dependencies, err = c.alireCrate(ctx, index, t)
	case Quicklisp:
		dependencies, err = c.quicklispProject(ctx, index, t)
	case PuppetForge:
		dependencies, err = c.forgeModule(ctx, index, t)
	case Racket:
		dependencies, err = c.racketPackage(ctx, index, t)
	case Wally:
		dependencies, err = c.wallyPackage(ctx, index, t)
	case Buf:
		dependencies, err = c.bufModule(ctx, index, t)
	case CUE:
		dependencies, err = c.cueModule(ctx, index, t)
	case SwiftPM:
		dependencies, err = c.swiftPackage(ctx, index, t)
	case Conan:
		dependencies, err = c.conanRecipe(ctx, index, t)
	case Actions:
		dependencies, err = c.actionDependencies(ctx, index, t)
	case PowerShell:
		dependencies, err = c.powershellModule(ctx, index, t)
	case Maven:
		if !strings.Contains(t.Package, ":") {
			// A name without an artifact cannot be asked: a POM is addressed by
			// group *and* artifact. Every plugin names Maven packages
			// group:artifact; what is left is a Bazel hub target that no
			// artifact list or lock file names (maven:<escaped_name>). Saying
			// nothing beats guessing an artifact.
			// Implements: REQ-SUP-028
			return answer{source: trace.NoAnswer, reason: trace.ReasonUnsupported}, nil
		}
		dependencies, err = c.mavenArtifact(ctx, index, t)
	default:
		return answer{source: trace.NoAnswer, reason: trace.ReasonUnsupported}, nil
	}
	if err != nil {
		return answer{requests: made.taken()}, err
	}
	c.cache.Put(key, dependencies)
	return answer{dependencies: c.targets(t, dependencies), source: trace.FromIndex, requests: made.taken()}, nil
}

// requestLog collects what one question sent, in the order it sent it. The ecosystem
// functions know nothing about it: it rides on the context they already carry, and
// do writes to it.
//
// Implements: REQ-TRC-007
type requestLog struct {
	mu sync.Mutex
	at []trace.Request
}

type requestLogKey struct{}

func (l *requestLog) add(r trace.Request) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.at = append(l.at, r)
}

func (l *requestLog) taken() []trace.Request {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.at
}

// targets turns an index's answer into what the graph takes. The version travels with
// the name: without it the next level cannot be asked for at all - a module proxy
// serves a go.mod for a version, not for a module.
//
// A crate's dependency comes from the registry the index says it does: its own
// unless the index names another (see cargoSparse).
//
// Implements: REQ-SUP-029, REQ-SUP-063
func (c *Client) targets(from lang.Target, dependencies []dependency) []lang.Target {
	out := make([]lang.Target, 0, len(dependencies))
	for _, d := range dependencies {
		e := from.Ecosystem
		if d.Ecosystem != "" {
			e = d.Ecosystem
		}
		t := lang.Target{
			Ecosystem: e, Package: d.Name, Version: d.Version,
			Pinned: d.Pinned(e), Registry: d.registry(from),
		}
		if e == Conan {
			t = c.config.conanPinned(t) // a lock's pin wins over the recipe's range
		}
		out = append(out, t)
	}
	return out
}

// userAgent identifies depphunter to the indexes. crates.io refuses a request without
// one, and an index that is rate-limiting is owed a name to complain about.
const userAgent = "depphunter (+https://github.com/sarumaj/depphunter-cli)"

// maxBody is as much of an answer as any of this needs to read.
const maxBody = 8 << 20

// get performs one request, with whatever credentials this machine has for the host.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	return c.accept(ctx, url, "application/json")
}

// accept is get for the answers that are not JSON: a nuspec, a sparse index line, an
// image manifest that has to name the media types it will take.
func (c *Client) accept(ctx context.Context, url, media string) ([]byte, error) {
	response, err := c.do(ctx, url, media, "")
	if err != nil {
		return nil, err
	}
	return readOK(response, url)
}

// getJSON is get decoded: it asks for address and decodes the JSON answer into a T.
// Credentials, the report's record of the request and the "not found" reading of
// its errors are get's own; an answer that is not JSON is an error.
func getJSON[T any](ctx context.Context, c *Client, address string) (T, error) {
	return acceptJSON[T](ctx, c, address, "application/json")
}

// acceptJSON is getJSON asking for another media type: a registry's own JSON type
// (application/vnd.pub.v2+json) or any answer at all (*/*).
func acceptJSON[T any](ctx context.Context, c *Client, address, media string) (T, error) {
	var doc T
	body, err := c.accept(ctx, address, media)
	if err == nil {
		err = json.Unmarshal(body, &doc)
	}
	return doc, err
}

// statusError is an index's answer other than 200 OK.
type statusError struct {
	url, status string
	code        int
}

func (e *statusError) Error() string { return e.url + ": " + e.status }

// errAbsent is an index's answer that does not list the package asked about: a
// Composer repository whose metadata has no such package, a feed with no version of
// it. It is "not found" as much as a 404 is.
var errAbsent = errors.New("the index does not list this package")

// notFound reports whether an index said it does not have a package - 404 or 410
// (the go command's rule for moving on to the next proxy), or an answer that does
// not list it - as opposed to failing to answer at all.
//
// Implements: REQ-SUP-063
func notFound(err error) bool {
	var s *statusError
	if errors.As(err, &s) {
		return s.code == http.StatusNotFound || s.code == http.StatusGone
	}
	return errors.Is(err, errAbsent)
}

// forbidden reports whether an index refused the credentials it was sent (403).
func forbidden(err error) bool {
	var s *statusError
	return errors.As(err, &s) && s.code == http.StatusForbidden
}

// noHexKey is the note for a Hex organization this machine holds no key for.
func noHexKey(repository string) string {
	return "no key on this machine for Hex organization " + path.Base(repository) + " (" + repository + "): its packages " +
		"are not asked; `mix hex.organization auth " + path.Base(repository) + " --key KEY`, a user key " +
		"(HEX_API_KEY, `mix hex.user auth`) or HEX_REPOS_KEY provides one"
}

// readLimited reads an answer, and no more of it than any of this has any use for.
func readLimited(response *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(response.Body, maxBody))
}

// readOK reads an answer that must be 200 OK and closes it. Any other status is a
// statusError for address, so that a 404 or 410 reads as "not found" (notFound).
func readOK(response *http.Response, address string) ([]byte, error) {
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &statusError{url: address, status: response.Status, code: response.StatusCode}
	}
	return readLimited(response)
}

// do makes one request and hands back the response unread. bearer, when given, is
// sent instead of this machine's own credentials - it is the token a registry handed
// out for this one pull (see ociToken).
//
// Implements: REQ-SUP-033, REQ-TRC-007
func (c *Client) do(ctx context.Context, url, media, bearer string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", media)
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	} else if goRequest, _ := ctx.Value(goRequestKey{}).(bool); goRequest {
		c.auth.ApplyGo(request)
	} else {
		c.auth.Apply(request)
	}
	return c.send(ctx, request, url)
}

// goRequestKey marks the context of a question the go command would ask a module
// proxy, whose credentials GOAUTH decides (auth.Store.ApplyGo).
type goRequestKey struct{}

// postForm posts a form and reads the answer, sending nothing of this machine's
// credentials: what the form holds is the credential (see ociToken).
//
// Implements: REQ-AUTH-030
func (c *Client) postForm(ctx context.Context, address string, form url.Values) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := c.send(ctx, request, address)
	if err != nil {
		return nil, err
	}
	return readOK(response, address)
}

// send sends one request as depphunter, recording it for the report under address.
func (c *Client) send(ctx context.Context, request *http.Request, address string) (*http.Response, error) {
	request.Header.Set("User-Agent", userAgent)
	made, _ := ctx.Value(requestLogKey{}).(*requestLog)
	start := time.Now()
	response, err := c.http.Do(request)
	status := "ok"
	switch {
	case err != nil:
		status = err.Error()
	case response != nil:
		status = response.Status
	}
	made.add(trace.Request{URL: address, Status: status, Millis: time.Since(start).Milliseconds()})
	return response, err
}

// goModule reads a module's own requirements from its go.mod, which a module proxy
// serves on its own - no archive, no checkout. A version is guaranteed: lookup turns
// a module without one away, so that the report can say why rather than record an
// empty answer.
//
// Implements: REQ-SUP-021
func (c *Client) goModule(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	escaped, err := module.EscapePath(t.Package)
	if err != nil {
		return nil, err
	}
	version, err := module.EscapeVersion(t.Version)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, fmt.Sprintf("%s/%s/@v/%s.mod", index, escaped, version))
	if err != nil {
		return nil, err
	}
	// Lax parsing: this is someone else's go.mod, and one line this version of the
	// parser dislikes should not cost the whole module's requirements.
	f, err := modfile.ParseLax("go.mod", body, nil)
	if err != nil {
		return nil, err
	}
	var out []dependency
	for _, r := range f.Require {
		if !r.Indirect { // the proxy's go.mod lists what this module itself needs
			out = append(out, dependency{Name: r.Mod.Path, Version: r.Mod.Version})
		}
	}
	return out, nil
}

// npmPackage reads a version's dependencies from the registry.
//
// Implements: REQ-SUP-022
func (c *Client) npmPackage(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	version := t.Version
	if !lang.PinnedSemver(version) {
		version = "latest" // a range names no document to ask for
	}
	doc, err := getJSON[struct {
		Dependencies map[string]string `json:"dependencies"`
	}](ctx, c, fmt.Sprintf("%s/%s/%s", index, strings.ReplaceAll(t.Package, "/", "%2f"), version))
	if err != nil {
		return nil, err
	}
	out := make([]dependency, 0, len(doc.Dependencies))
	for name, constraint := range doc.Dependencies {
		// A range is what the package asked for; it is not a version, and the next
		// request falls back to the current one.
		out = append(out, dependency{Name: name, Version: constraint})
	}
	return out, nil
}

// pypiDistribution reads requires-dist from the JSON API. An index that has no JSON
// API - it answers 404, or something that is not JSON - is read through the Simple
// API instead (pypiSimple); PyPI itself has the JSON API, and is not asked twice. A
// flat index is read as the page or directory of files it is (pypiFlat).
//
// Implements: REQ-SUP-023, REQ-SUP-067
func (c *Client) pypiDistribution(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	if page, local := c.config.pythonFlat(index); page != "" {
		return c.pypiFlat(ctx, page, local, t)
	}
	host := strings.TrimSuffix(strings.TrimSuffix(index, "/simple"), "/simple/")
	url := fmt.Sprintf("%s/pypi/%s/json", host, t.Package)
	if lang.Pinned(t.Version) {
		url = fmt.Sprintf("%s/pypi/%s/%s/json", host, t.Package, t.Version)
	}
	doc, err := getJSON[struct {
		Info struct {
			RequiresDist []string `json:"requires_dist"`
		} `json:"info"`
	}](ctx, c, url)
	var syntax *json.SyntaxError
	if (notFound(err) || errors.As(err, &syntax)) && !c.config.Public(PyPI, index) {
		return c.pypiSimple(ctx, index, t)
	}
	if err != nil {
		return nil, err
	}
	return requiresDist(doc.Info.RequiresDist), nil
}

// extraMarker is a requirement installed only when an extra is asked for.
var extraMarker = regexp.MustCompile(`\bextra\s*==`)

// requiresDist turns Requires-Dist entries into dependencies. "certifi
// (>=2017.4.17)" is one; anything guarded by an extra is installed only when that
// extra is asked for, and is left out. Other environment markers are kept: the
// platform the map is drawn for is not known.
func requiresDist(requirements []string) []dependency {
	var out []dependency
	for _, requirement := range requirements {
		if extraMarker.MatchString(requirement) {
			continue
		}
		if name := requirementName(requirement); name != "" {
			out = append(out, dependency{Name: name, Version: requirementConstraint(requirement, name)})
		}
	}
	return out
}

// requirementConstraint is what a requirement asks for, without its name, markers or
// brackets: "certifi (>=2017.4.17)" asks for ">=2017.4.17".
func requirementConstraint(requirement, name string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(requirement), name))
	if i := strings.Index(rest, ";"); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.TrimSpace(rest)
	rest = strings.TrimPrefix(rest, "(")
	rest = strings.TrimSuffix(rest, ")")
	if strings.HasPrefix(rest, "[") { // an extras list, not a version
		return ""
	}
	return strings.TrimSpace(rest)
}

// dependency is one dependency an index reported, with whatever it said about its version.
type dependency struct {
	Name    string `json:"n"`
	Version string `json:"v,omitempty"`
	// Ecosystem is the dependency's ecosystem when it is not the package's own: a Terraform
	// module requires providers as well as modules.
	Ecosystem string `json:"e,omitempty"`
	// Registry is where a crate's dependency is published, as a Cargo index line
	// says it: "" for the registry of the crate itself, crates.io for crates.io,
	// else the other registry's index URL. A Julia package's dependency's UUID.
	Registry string `json:"r,omitempty"`
}

// registry is the lang.Target.Registry of a dependency of from.
//
// Implements: REQ-SUP-047
func (d dependency) registry(from lang.Target) string {
	switch {
	case from.Ecosystem == Hex:
		// The Hex API does not say which repository a requirement is in; the
		// registry's rule is the parent's own unless it says otherwise.
		return from.Registry
	case from.Ecosystem == Julia:
		// A registry's Deps.toml names each dependency's UUID.
		return d.Registry
	case from.Ecosystem == Wally:
		// A Wally manifest names the registry its dependencies come from.
		return d.Registry
	case from.Ecosystem == Conan:
		// A recipe's requirement names its user, channel and revision.
		return d.Registry
	case from.Ecosystem == Actions:
		// An action's path inside its repository.
		return d.Registry
	case from.Ecosystem == PowerShell:
		// PowerShellGet installs a module's dependencies from the repository it
		// installs the module from.
		return from.Registry
	case from.Ecosystem != Cargo:
		return ""
	case d.Registry == "":
		return from.Registry // the same registry as the crate that depends on it
	case d.Registry == cratesIO:
		return ""
	}
	return d.Registry
}

// cratesIO is how a Cargo index names crates.io when a crate of another registry
// depends on one of its crates.
const cratesIO = "crates.io"

// Pinned reports whether the version this index gave fixes one release. A Go module
// proxy answers with the exact version the build selects; npm and PyPI answer with
// the constraint the package asked for, which usually does not.
func (d dependency) Pinned(ecosystem string) bool {
	if ecosystem == Maven {
		return lang.PinnedMaven(d.Version) // 1.2.3.RELEASE is one release, [1.0,2.0) is not
	}
	if ecosystem == Bazel || ecosystem == Buf {
		// A registry's version is one release, whatever its shape (1.2.0.bcr.1);
		// a Buf Schema Registry commit is one commit.
		return d.Version != ""
	}
	if ecosystem == Racket {
		return false // a catalog's #:version is a minimum
	}
	if ecosystem == Conan {
		// A reference's version is one version; a range is bracketed.
		return d.Version != "" && !strings.HasPrefix(d.Version, "[")
	}
	if ecosystem == Actions {
		return lang.Commit(d.Version) // a tag can be moved to other code
	}
	if ecosystem == OCI {
		// A tag is republished at will; only a digest pins an image, as
		// oci.Image reads a reference.
		return strings.Contains(d.Version, ":")
	}
	if ecosystem == PowerShell {
		// The Gallery writes one version as "[1.2.3]" (a bare version is one
		// version too, as PowerShellGet reads it); a range is not one.
		return powershellExact(d.Version) != ""
	}
	if ecosystem == Wally {
		// A bare version is a caret range in Wally; "=1.2.3" is one release.
		version, exact := strings.CutPrefix(strings.TrimSpace(d.Version), "=")
		return exact && lang.Pinned(version)
	}
	if ecosystem == NPM || ecosystem == Julia || ecosystem == Elm || ecosystem == PureScript {
		// A registry's compat "1" is every 1.x; only a whole "1.2.3" is one release
		// (Elm and PureScript packages' dependencies are always ranges).
		return lang.PinnedSemver(d.Version)
	}
	return lang.Pinned(d.Version)
}

// requirementName takes the distribution name off the front of a requirement.
func requirementName(requirement string) string {
	if i := strings.IndexAny(requirement, " <>=!~[;("); i >= 0 {
		requirement = requirement[:i]
	}
	return strings.TrimSpace(requirement)
}
