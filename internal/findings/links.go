package findings

import (
	"context"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang/markdown"
	"github.com/sarumaj/depphunter-cli/internal/store"
)

// Why a link is reported, carried as the finding's Ref so the side panel can group
// them the way it groups a linter's rules.
const (
	refMissingFile   = "link/missing-file"
	refMissingAnchor = "link/missing-anchor"
	refUndefinedRef  = "link/undefined-reference"
	refGone          = "link/gone"
)

// checkLinks follows every link the repository's documentation carries and reports
// the ones that lead nowhere.
//
// The check is exact where it can be: a path either names something on disk or does
// not, and a fragment either names a heading of the file it points at or does not.
// Neither needs the network, so both run on every analysis. An http(s) link needs
// somebody else's server to answer and is checked only when web is given.
func checkLinks(ctx context.Context, root string, docs []string, web *Web,
	logFormat func(string, ...any)) ([]*Finding, bool) {
	var out []*Finding
	partial := false
	anchors := &anchorCache{root: root, read: map[string]map[string]bool{}}
	external := map[string][]*Finding{} // url -> where it is written

	for _, doc := range docs {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc)))
		if err != nil {
			logFormat("links: %s: %v", doc, err)
			partial = true
			continue
		}
		for _, l := range markdown.Links(src) {
			switch {
			case l.Ref != "":
				out = append(out, broken(doc, l, Medium, refUndefinedRef,
					"the reference ["+l.Ref+"] is never defined"))

			case strings.HasPrefix(l.Dest, "#"):
				if fragment := l.Dest[1:]; !anchors.of(doc, src)[anchor(fragment)] {
					out = append(out, broken(doc, l, Low, refMissingAnchor,
						"no heading in this file is named "+l.Dest))
				}

			case markdown.External(l.Dest):
				if web != nil {
					f := broken(doc, l, Low, refGone, "")
					external[l.Dest] = append(external[l.Dest], f)
				}

			default:
				target, ok := markdown.Target(doc, l.Dest)
				if !ok {
					continue // a mail address, a fragment of a URL, a path out of the repository
				}
				info, err := os.Stat(filepath.Join(root, filepath.FromSlash(target)))
				if err != nil {
					out = append(out, broken(doc, l, Medium, refMissingFile, target+" is not there"))
					continue
				}
				_, fragment, hasFragment := strings.Cut(l.Dest, "#")
				if !hasFragment || fragment == "" || info.IsDir() || !isMarkdown(target) {
					continue
				}
				if !anchors.of(target, nil)[anchor(fragment)] {
					out = append(out, broken(doc, l, Low, refMissingAnchor,
						"no heading in "+target+" is named #"+fragment))
				}
			}
		}
	}

	if len(external) > 0 {
		urls := make([]string, 0, len(external))
		for u := range external {
			urls = append(urls, u)
		}
		sort.Strings(urls)
		gone, unchecked := web.Check(ctx, urls, logFormat)
		for _, u := range urls {
			status, ok := gone[u]
			if !ok {
				continue
			}
			for _, f := range external[u] {
				f.Title = u + " answers " + strconv.Itoa(status)
				out = append(out, f)
			}
		}
		if unchecked > 0 {
			// Said, not marked: a host that refuses a robot or does not answer in
			// time is the ordinary case, and a set marked incomplete on every run
			// because of it would stop meaning anything.
			logFormat("links: %d of %d external links could not be checked", unchecked, len(urls))
		}
	}
	return out, partial
}

// broken is one link that leads nowhere, placed on the line that carries it.
func broken(doc string, l markdown.Link, severity Severity, ref, title string) *Finding {
	return &Finding{
		Kind: KindLink, Source: "links", Ref: ref, Severity: severity,
		Title: title, Detail: l.Spec,
		Path: doc, Line: l.Line, Column: l.Col,
	}
}

// anchor normalizes a fragment the way a renderer normalizes a heading, so that
// "#Package-indexes" finds the anchor of "## Package indexes".
func anchor(fragment string) string { return strings.ToLower(fragment) }

func isMarkdown(p string) bool {
	switch path.Ext(p) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}

// anchorCache reads a linked-to document once, however many links point into it.
type anchorCache struct {
	root string
	read map[string]map[string]bool
}

// of is the anchors of a document. src saves a read where the caller has the file
// already; nil reads it.
func (c *anchorCache) of(p string, src []byte) map[string]bool {
	if have, ok := c.read[p]; ok {
		return have
	}
	if src == nil {
		var err error
		if src, err = os.ReadFile(filepath.Join(c.root, filepath.FromSlash(p))); err != nil {
			c.read[p] = nil
			return nil
		}
	}
	found := markdown.Anchors(src)
	c.read[p] = found
	return found
}

// ---------------------------------------------------------------- the web

// Web asks whether an http(s) link still answers. It exists only with --online, and
// it is deliberately hard to convince: a link is reported when a host says the thing
// is gone - 404 or 410 - and not when the host refuses a robot, rate-limits, times
// out or breaks. Those are the answers a checker gets from Cloudflare and from
// GitHub's own bot rules, and a checker that read them as rot would invent a finding
// for every link that works perfectly well in a browser.
type Web struct {
	http    *http.Client
	cache   *store.Store
	auth    *auth.Store
	timeout time.Duration
}

// NewWeb prepares the checker. dir holds the answers, which are kept for ttl: link
// rot is slow, and asking a hundred hosts on every re-analysis is the kind of thing
// that gets a tool blocked. credentials are what this machine holds, so that a link
// into a private repository or an internal wiki is checked rather than reported
// missing; each is sent only to the host it was written for.
func NewWeb(dir string, ttl, timeout time.Duration, credentials *auth.Store) *Web {
	return &Web{
		http:    &http.Client{Timeout: timeout},
		cache:   store.New(dir, ttl),
		auth:    credentials,
		timeout: timeout,
	}
}

// webWorkers is how many links are asked about at once. The other end is somebody
// else's server, and a repository's README alone can carry dozens.
const webWorkers = 8

// userAgent names depphunter to the hosts it asks, so one that is rate-limiting has
// somebody to complain about.
const userAgent = "depphunter link check (+https://github.com/sarumaj/depphunter-cli)"

// Check asks about each URL once. It answers with the ones a host said are gone, and
// with how many could not be checked at all.
func (w *Web) Check(ctx context.Context, urls []string, logFormat func(string, ...any)) (map[string]int, int) {
	gone := map[string]int{}
	unchecked := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	work := make(chan string)

	for range min(webWorkers, len(urls)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range work {
				status, ok := w.status(ctx, u)
				mu.Lock()
				switch {
				case !ok:
					unchecked++
				case status == http.StatusNotFound || status == http.StatusGone:
					gone[u] = status
				}
				mu.Unlock()
			}
		}()
	}
	for _, u := range urls {
		select {
		case work <- u:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
	return gone, unchecked
}

// status is what a host says about a URL, and whether it said anything usable. A
// HEAD is asked for first, because it is the question without the answer's body;
// hosts that will not take one are asked again with GET.
func (w *Web) status(ctx context.Context, u string) (int, bool) {
	if code, ok := store.Get[int](w.cache, "link|"+u); ok {
		return code, true
	}
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	code, err := w.ask(ctx, http.MethodHead, u)
	switch {
	case err != nil:
		return 0, false
	case code == http.StatusMethodNotAllowed || code == http.StatusNotImplemented || code == http.StatusForbidden:
		if code, err = w.ask(ctx, http.MethodGet, u); err != nil {
			return 0, false
		}
	}
	// A refusal and a rate limit say nothing about whether the page is there, so
	// they are not kept either: the next run may be let through.
	if code == http.StatusForbidden || code == http.StatusTooManyRequests || code >= 500 {
		return code, false
	}
	w.cache.Put("link|"+u, code)
	return code, true
}

func (w *Web) ask(ctx context.Context, method, u string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	// A link to a private repository or an internal wiki answers 404 to an
	// anonymous request, and a 404 is what this reports. The credential goes only to
	// a host this machine's own files name.
	w.auth.Apply(req)
	resp, err := w.http.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
