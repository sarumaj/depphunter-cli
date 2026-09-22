// Package trace records how an analysis reached the dependencies it drew, because
// neither half of that leaves a mark on the map. A tree that stops two levels down
// looks the same whether the dependencies end there, a lock file said nothing, or a
// proxy answered 404; a package from the company's Nexus is drawn like one from
// registry.npmjs.org, which is the whole of a dependency-confusion question.
//
// A Report holds every question the walk asked with who answered it - a lock file,
// an index, a kept answer, or nobody, and then why not - and every index the run
// knew about, where that was learned and what resolved from it. One is filled per
// analysis and read three ways: --explain writes it to the log, and the server
// serves it at /api/resolution as JSON or rendered.
package trace

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/graph"
)

// Answer says who answered one question about what a package depends on.
type Answer string

const (
	// FromLock is the repository's own answer: a lock file it carries.
	FromLock Answer = "lock"
	// FromIndex is a package index, asked over the network.
	FromIndex Answer = "index"
	// FromCache is an index's answer, kept on disk from an earlier run.
	FromCache Answer = "cache"
	// FromMemo is an index's answer, already given earlier in this run. It is not a
	// second question - it is the same one, and saying so is how a report of a
	// --watch re-analysis stays honest about what was actually asked.
	FromMemo Answer = "memo"
	// NoAnswer is nobody: Reason says which nobody.
	NoAnswer Answer = "none"
)

// Why nothing answered. These are sentences rather than codes because the report is
// read by whoever ran depphunter, and "private" on its own explains nothing.
const (
	ReasonOffline     = "no lock file records it, and --online was not given"
	ReasonUntrusted   = "its index is one only the repository names, which is never fetched from"
	ReasonPrivate     = "a private package is never named to a public index"
	ReasonNoVersion   = "no version to ask for: a proxy serves a document per version"
	ReasonUnsupported = "this ecosystem's index cannot be asked (an import names no artifact)"
	ReasonNoIndex     = "depphunter asks no index for this ecosystem"
)

// maxLookups is as many questions as one report keeps in full. Past it the counts go
// on being kept and the detail is dropped: -1 over a large lock file asks hundreds of
// thousands of times, and a report nobody can open explains nothing either.
const maxLookups = 20000

// maxListed is as many of one list as the written report prints before it says how
// many more there were. The JSON keeps all of them.
const maxListed = 50

// Source is one index the run knew about, and how it came to know.
type Source struct {
	Ecosystem string `json:"ecosystem"`
	URL       string `json:"url"`
	// Scope limits the source to part of the ecosystem: an npm scope, a Maven group.
	Scope string `json:"scope,omitempty"`
	// Origin is who named it - see the Origin constants in internal/index.
	Origin string `json:"origin"`
	// Trusted marks a source that may be fetched from. Anything else is recorded so
	// the map can say where a package claims to come from, and then left alone.
	Trusted bool `json:"trusted"`
}

// Use is one index and what ended up resolving from it.
type Use struct {
	Ecosystem string `json:"ecosystem"`
	Index     string `json:"index"`
	Trusted   bool   `json:"trusted"`
	Packages  int    `json:"packages"`
	Private   int    `json:"private"`
}

// Level is one round of the walk: every package known at that depth, asked together.
type Level struct {
	Plugin string `json:"plugin"`
	// Depth counts rounds past the packages the code imports directly: 0 asks those,
	// 1 asks what they answered with.
	Depth    int   `json:"depth"`
	Asked    int   `json:"asked"`
	Answered int   `json:"answered"`
	Added    int   `json:"added"`
	Edges    int   `json:"edges"`
	Millis   int64 `json:"ms"`
}

// Skip is an ecosystem the walk never entered, and why. It is the other half of the
// answer to "why does this package have no dependencies on the map": not every
// ecosystem keeps its graph somewhere depphunter was allowed to look.
type Skip struct {
	Plugin string `json:"plugin"`
	Reason string `json:"reason"`
}

// Request is one round trip an index question took. A container image takes three of
// them, and "which one failed" is the question this report exists to answer.
type Request struct {
	URL    string `json:"url"`
	Status string `json:"status"`
	Millis int64  `json:"ms"`
}

// Lookup is one question about one package, and what came of it.
type Lookup struct {
	Plugin    string `json:"plugin,omitempty"`
	Level     int    `json:"level"`
	Ecosystem string `json:"ecosystem"`
	Package   string `json:"package"`
	Version   string `json:"version,omitempty"`
	Answer    Answer `json:"answer"`
	// Index is the index the question went to, or would have gone to had it been
	// asked: a package skipped for being private still says which index was spared.
	Index  string `json:"index,omitempty"`
	Deps   int    `json:"deps"`
	Reason string `json:"reason,omitempty"`
	// Requests is what went over the network, when anything did.
	Requests []Request `json:"requests,omitempty"`
	Millis   int64     `json:"ms,omitempty"`
}

// Totals count what the detail adds up to, so the counts survive the cap on Lookups.
type Totals struct {
	Packages   int `json:"packages"`
	Transitive int `json:"transitive"`
	PrivatePkg int `json:"privatePackages"`
	Untrusted  int `json:"untrustedPackages"`
	Asked      int `json:"asked"`
	FromLock   int `json:"fromLock"`
	FromIndex  int `json:"fromIndex"`
	FromCache  int `json:"fromCache"`
	FromMemo   int `json:"fromMemo"`
	Unanswered int `json:"unanswered"`
	Failed     int `json:"failed"`
	Requests   int `json:"requests"`
}

// Report is one analysis's account of itself. A nil *Report records nothing, so the
// analysis does not have to know whether anybody is reading.
type Report struct {
	Root string `json:"root,omitempty"`
	// What the run was asked to do, so a report read on its own says what produced it.
	ResolveDepth   int       `json:"resolveDepth"`
	Online         bool      `json:"online"`
	Private        []string  `json:"private,omitempty"`
	TrustedIndexes []string  `json:"trustedIndexes,omitempty"`
	GeneratedAt    time.Time `json:"generatedAt"`

	Sources []Source `json:"sources,omitempty"`
	Indexes []Use    `json:"indexes,omitempty"`
	Levels  []Level  `json:"levels,omitempty"`
	Skipped []Skip   `json:"skipped,omitempty"`
	Lookups []Lookup `json:"lookups,omitempty"`
	// Dropped counts the questions past maxLookups: counted in Totals, not kept here.
	Dropped int    `json:"dropped,omitempty"`
	Totals  Totals `json:"totals"`

	// mu guards everything above from the walk's workers, which ask a whole level at
	// once. level and plugin are where the walk has got to: the client that records a
	// lookup is several calls below the loop that knows.
	mu     sync.Mutex
	level  int
	plugin string
}

// New starts a report for a run configured this way.
func New(resolveDepth int, online bool, private, trusted []string) *Report {
	return &Report{
		ResolveDepth: resolveDepth, Online: online,
		Private: append([]string(nil), private...), TrustedIndexes: append([]string(nil), trusted...),
	}
}

// SetSources records the indexes the run discovered.
func (r *Report) SetSources(s []Source) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Sources = s
}

// Enter says which plugin's walk is about to ask, and how far past the direct
// dependencies it has got. Plugins are walked one after another, so one current
// level is enough for the workers of all of them.
func (r *Report) Enter(plugin string, depth int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugin, r.level = plugin, depth
}

// Done closes the round Enter opened.
func (r *Report) Done(asked, answered, added, edges int, took time.Duration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Levels = append(r.Levels, Level{
		Plugin: r.plugin, Depth: r.level, Asked: asked, Answered: answered,
		Added: added, Edges: edges, Millis: took.Milliseconds(),
	})
}

// Skip records an ecosystem whose walk never started.
func (r *Report) Skip(plugin, reason string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Skipped = append(r.Skipped, Skip{Plugin: plugin, Reason: reason})
}

// Add records one question. The level and the plugin are the walk's, not the
// caller's: whoever answers is too far down to know either.
func (r *Report) Add(l Lookup) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	l.Plugin, l.Level = r.plugin, r.level
	r.Totals.Asked++
	r.Totals.Requests += len(l.Requests)
	switch l.Answer {
	case FromLock:
		r.Totals.FromLock++
	case FromIndex:
		r.Totals.FromIndex++
	case FromCache:
		r.Totals.FromCache++
	case FromMemo:
		r.Totals.FromMemo++
	case NoAnswer:
		r.Totals.Unanswered++
		if len(l.Requests) > 0 || l.Reason == "" {
			// Something was asked and it did not work out, as against a question
			// that was deliberately not asked at all.
			r.Totals.Failed++
		}
	}
	if len(r.Lookups) >= maxLookups {
		r.Dropped++
		return
	}
	r.Lookups = append(r.Lookups, l)
}

// Summarize reads the finished graph for what the walk itself cannot say: which index
// each package on the map ended up resolving from, and how many of them the map
// marks private or marks as coming from an index nothing here vouches for.
func (r *Report) Summarize(g *graph.Graph) {
	if r == nil || g == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Root = g.Root
	eco := map[string]string{} // ecosystem node id -> its id without the prefix
	for _, n := range g.Nodes {
		if n.Kind == graph.KindEcosystem {
			eco[n.ID] = strings.TrimPrefix(n.ID, "e:")
		}
	}
	use := map[[2]string]*Use{}
	for _, n := range g.Nodes {
		if n.Kind != graph.KindPackage || n.Index == "" {
			continue
		}
		r.Totals.Packages++
		if n.Transitive {
			r.Totals.Transitive++
		}
		if n.Private {
			r.Totals.PrivatePkg++
		}
		if n.IndexUnknown {
			r.Totals.Untrusted++
		}
		key := [2]string{eco[n.Parent], n.Index}
		u := use[key]
		if u == nil {
			u = &Use{Ecosystem: key[0], Index: n.Index, Trusted: !n.IndexUnknown}
			use[key] = u
		}
		u.Packages++
		if n.Private {
			u.Private++
		}
	}
	r.Indexes = r.Indexes[:0]
	for _, u := range use {
		r.Indexes = append(r.Indexes, *u)
	}
	sort.Slice(r.Indexes, func(i, j int) bool {
		if r.Indexes[i].Ecosystem != r.Indexes[j].Ecosystem {
			return r.Indexes[i].Ecosystem < r.Indexes[j].Ecosystem
		}
		return r.Indexes[i].Index < r.Indexes[j].Index
	})
}

// Finish puts the report in an order that does not depend on which worker came back
// first, so two runs over the same repository produce the same report.
func (r *Report) Finish() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.GeneratedAt = time.Now().UTC()
	sort.SliceStable(r.Lookups, func(i, j int) bool {
		a, b := r.Lookups[i], r.Lookups[j]
		switch {
		case a.Level != b.Level:
			return a.Level < b.Level
		case a.Ecosystem != b.Ecosystem:
			return a.Ecosystem < b.Ecosystem
		default:
			return a.Package < b.Package
		}
	})
}

// Walked reports whether there is anything to say about the walk: without
// --resolve-depth nothing past the direct dependencies was ever asked.
func (r *Report) Walked() bool { return r != nil && (len(r.Levels) > 0 || len(r.Skipped) > 0) }

// Reason is one cause of an unanswered question, with how often it came up.
type Reason struct {
	Count int
	Why   string
}

// Reasons counts the unanswered questions by what went wrong, commonest first.
// Fifty identical lines say less than one line and a count.
func (r *Report) Reasons() []Reason {
	if r == nil {
		return nil
	}
	count := map[string]int{}
	for _, l := range r.Unanswered() {
		count[or(l.Reason, requestSummary(l))]++
	}
	out := make([]Reason, 0, len(count))
	for why, n := range count {
		out = append(out, Reason{Count: n, Why: why})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Why < out[j].Why
	})
	return out
}

// Unanswered lists the questions nothing answered, worst first: the ones where
// something was asked and failed come before the ones nothing was asked about.
func (r *Report) Unanswered() []Lookup {
	if r == nil {
		return nil
	}
	var out []Lookup
	for _, l := range r.Lookups {
		if l.Answer == NoAnswer {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i].Requests) > len(out[j].Requests)
	})
	return out
}
