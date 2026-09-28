package index

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// The Simple repository API is what every PyPI index serves, and all that many
// private ones do: GitLab, AWS CodeArtifact, Azure Artifacts, Google Artifact
// Registry, devpi and plain Nexus or Artifactory repositories answer pip's
// <index>/<name>/ and nothing at the Warehouse JSON API. A page lists a project's
// files; a file whose index extracted its metadata (PEP 658) has that metadata
// beside it at <file>.metadata, which is where Requires-Dist is.

// simpleAccept asks for the JSON form of a page (PEP 691) and takes HTML (PEP 503)
// from an index that has only that.
const simpleAccept = "application/vnd.pypi.simple.v1+json, application/vnd.pypi.simple.v1+html;q=0.2, text/html;q=0.01"

// errNoMetadata is a release whose files carry no metadata of their own. It is not
// "not found": the index has the package, so the next index is not asked, and the
// report says why there is no answer. A wheel holds its METADATA, but a wheel can
// be hundreds of megabytes, and depphunter does not download archives.
var errNoMetadata = errors.New("the index serves no metadata file (PEP 658) for this release, and depphunter does not download archives")

// simpleFile is one file a project page lists.
type simpleFile struct {
	name, url string
	metadata  bool              // the index serves <url>.metadata (PEP 658 / 714)
	hashes    map[string]string // the metadata file's hashes, when the page gives them
	yanked    bool              // PEP 592
}

// pypiSimple reads a distribution's dependencies through the Simple API: the
// project page, the release asked for (the newest final, non-yanked one when the
// version is not pinned), and the Requires-Dist of one of its files' metadata, a
// wheel's by preference. File URLs are taken relative to the page (after any
// redirect), and each request carries whatever this machine's credential store has
// for its own URL - a file on another host or outside the index's path gets that
// host's or path's credential, or none.
//
// Implements: REQ-SUP-067
func (c *Client) pypiSimple(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	page := strings.TrimRight(index, "/") + "/" + pypiName(t.Package) + "/"
	files, err := c.simpleFiles(ctx, page)
	if err != nil {
		return nil, err
	}
	version, chosen := pickRelease(files, t.Package, t.Version)
	if len(chosen) == 0 {
		return nil, fmt.Errorf("%s: %w", page, errAbsent)
	}
	for _, f := range chosen {
		if !f.metadata {
			continue
		}
		body, err := c.accept(ctx, metadataURL(f.url), "*/*")
		if notFound(err) {
			continue // advertised but not there: another file's may be
		}
		if err != nil {
			return nil, err
		}
		if want := f.hashes["sha256"]; want != "" {
			if sum := sha256.Sum256(body); !strings.EqualFold(hex.EncodeToString(sum[:]), want) {
				return nil, fmt.Errorf("%s: the metadata does not match the hash the index gives for it", metadataURL(f.url))
			}
		}
		return requiresDist(metadataRequires(body)), nil
	}
	return nil, fmt.Errorf("%s %s: %w", page, version, errNoMetadata)
}

// simpleFiles reads a project page, in whichever form the index answered with.
func (c *Client) simpleFiles(ctx context.Context, page string) ([]simpleFile, error) {
	resp, err := c.do(ctx, page, simpleAccept, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &statusError{url: page, status: resp.Status, code: resp.StatusCode}
	}
	body, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	base := resp.Request.URL // relative links are relative to where the page was served
	media, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if strings.HasSuffix(media, "+json") || media == "application/json" {
		return simpleJSON(body, base)
	}
	return simpleHTML(body, base), nil
}

// simpleJSON reads a PEP 691 project page.
func simpleJSON(body []byte, base *url.URL) ([]simpleFile, error) {
	var doc struct {
		Files []struct {
			Filename         string          `json:"filename"`
			URL              string          `json:"url"`
			CoreMetadata     json.RawMessage `json:"core-metadata"`
			DistInfoMetadata json.RawMessage `json:"dist-info-metadata"`
			Yanked           json.RawMessage `json:"yanked"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	var out []simpleFile
	for _, f := range doc.Files {
		u, err := base.Parse(f.URL)
		if err != nil {
			continue
		}
		meta := f.CoreMetadata
		if len(meta) == 0 {
			meta = f.DistInfoMetadata // the name before PEP 714
		}
		file := simpleFile{name: f.Filename, url: u.String(), yanked: truthy(f.Yanked)}
		var hashes map[string]string
		if json.Unmarshal(meta, &hashes) == nil && hashes != nil {
			file.metadata, file.hashes = true, hashes
		} else {
			file.metadata = truthy(meta)
		}
		out = append(out, file)
	}
	return out, nil
}

// truthy is a PEP 691 value that is either a boolean or says yes by being there:
// yanked is true or a reason, metadata true or its hashes.
func truthy(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && !bytes.Equal(raw, []byte("false")) && !bytes.Equal(raw, []byte("null"))
}

var (
	anchorTag = regexp.MustCompile(`(?is)<a\s([^>]*)>(.*?)</a\s*>`)
	baseTag   = regexp.MustCompile(`(?is)<base\s([^>]*)>`)
	attribute = regexp.MustCompile(`([^\s=/>"']+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?`)
	innerTags = regexp.MustCompile(`<[^>]*>`)
)

// simpleHTML reads a PEP 503 project page: one anchor per file, the filename as its
// text, PEP 592's data-yanked, and PEP 658's data-dist-info-metadata (PEP 714's
// data-core-metadata) as "true" or the metadata file's hash.
func simpleHTML(body []byte, base *url.URL) []simpleFile {
	if m := baseTag.FindSubmatch(body); m != nil {
		if href, ok := attributes(m[1])["href"]; ok {
			if u, err := base.Parse(href); err == nil {
				base = u
			}
		}
	}
	var out []simpleFile
	for _, m := range anchorTag.FindAllSubmatch(body, -1) {
		attrs := attributes(m[1])
		href, ok := attrs["href"]
		if !ok {
			continue
		}
		u, err := base.Parse(href)
		if err != nil {
			continue
		}
		file := simpleFile{url: u.String(), name: strings.TrimSpace(html.UnescapeString(innerTags.ReplaceAllString(string(m[2]), "")))}
		if file.name == "" {
			file.name, _ = url.PathUnescape(path.Base(u.Path))
		}
		_, file.yanked = attrs["data-yanked"]
		meta, ok := attrs["data-core-metadata"]
		if !ok {
			meta, ok = attrs["data-dist-info-metadata"]
		}
		if ok && !strings.EqualFold(meta, "false") {
			file.metadata = true
			if name, value, ok := strings.Cut(meta, "="); ok {
				file.hashes = map[string]string{strings.ToLower(name): value}
			}
		}
		out = append(out, file)
	}
	return out
}

// attributes reads a tag's attributes, lower-cased names to unescaped values ("" for
// one written without a value).
func attributes(tag []byte) map[string]string {
	out := map[string]string{}
	for _, m := range attribute.FindAllSubmatch(tag, -1) {
		out[strings.ToLower(string(m[1]))] = html.UnescapeString(string(m[2]) + string(m[3]) + string(m[4]))
	}
	return out
}

// metadataURL is where PEP 658 puts a file's metadata: its URL, without the hash
// fragment, with .metadata appended to the path.
func metadataURL(file string) string {
	file, _, _ = strings.Cut(file, "#")
	if i := strings.Index(file, "?"); i >= 0 {
		return file[:i] + ".metadata" + file[i:]
	}
	return file + ".metadata"
}

// pickRelease chooses the files of the release asked for: the pinned version, yanked
// or not (PEP 592 keeps a yanked release for whoever pins it), or else the newest
// release that is neither yanked nor a pre-release - a pre-release only when there
// is nothing else, as pip does. Files come back in the order their metadata is
// worth trying: advertised metadata first, then wheels before source archives.
func pickRelease(files []simpleFile, pkg, want string) (string, []simpleFile) {
	name := pypiName(pkg)
	type release struct {
		file    simpleFile
		version string
		v       pep440
		ok      bool
	}
	var all []release
	for _, f := range files {
		if version, ok := distVersion(f.name, name); ok {
			v, parsed := parsePEP440(version)
			all = append(all, release{f, version, v, parsed})
		}
	}
	var version string
	var target pep440
	var targetOK bool
	if lang.Pinned(want) {
		version = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(want), "=="))
		target, targetOK = parsePEP440(version)
	} else {
		for _, pre := range []bool{false, true} {
			for _, r := range all {
				if r.ok && !r.file.yanked && (pre || !r.v.prerelease()) && (!targetOK || r.v.compare(target) > 0) {
					version, target, targetOK = r.version, r.v, true
				}
			}
			if targetOK {
				break
			}
		}
		if !targetOK {
			return "", nil
		}
	}
	var chosen []simpleFile
	for _, r := range all {
		same := r.version == version
		if targetOK && r.ok {
			same = r.v.compare(target) == 0
		}
		if same && (lang.Pinned(want) || !r.file.yanked) {
			chosen = append(chosen, r.file)
		}
	}
	slices.SortStableFunc(chosen, func(a, b simpleFile) int {
		rank := func(f simpleFile) int {
			r := 0
			if !f.metadata {
				r += 2
			}
			if !strings.HasSuffix(strings.ToLower(f.name), ".whl") {
				r++
			}
			return r
		}
		return rank(a) - rank(b)
	})
	return version, chosen
}

// sdistSuffixes are the source archives a simple index serves.
var sdistSuffixes = []string{".tar.gz", ".zip", ".tar.bz2", ".tgz", ".tar.xz"}

// distVersion is the version in a wheel's or source archive's filename, when the
// file is of the distribution named (compared as PEP 503 does). A wheel's name has
// no "-" (PEP 427 escapes it); a source archive's may, so every split is tried
// and only one that leaves a PEP 440 version counts (lib-extra-1.0 is not lib's).
func distVersion(filename, name string) (string, bool) {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".whl") {
		parts := strings.Split(filename[:len(filename)-len(".whl")], "-")
		if len(parts) < 5 || pypiName(parts[0]) != name {
			return "", false
		}
		return parts[1], true
	}
	for _, suffix := range sdistSuffixes {
		if !strings.HasSuffix(lower, suffix) {
			continue
		}
		stem := filename[:len(filename)-len(suffix)]
		for i := range len(stem) {
			if stem[i] != '-' || pypiName(stem[:i]) != name {
				continue
			}
			if _, ok := parsePEP440(stem[i+1:]); ok {
				return stem[i+1:], true
			}
		}
	}
	return "", false
}

// metadataRequires is the Requires-Dist of a core metadata file: its headers stop
// at the first empty line, and a line that starts with white space continues the
// one before.
func metadataRequires(body []byte) []string {
	var out []string
	last := -1
	for _, line := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		if line == "" {
			break
		}
		if line[0] == ' ' || line[0] == '\t' {
			if last >= 0 {
				out[last] += " " + strings.TrimSpace(line)
			}
			continue
		}
		last = -1
		if key, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(key), "Requires-Dist") {
			out = append(out, strings.TrimSpace(value))
			last = len(out) - 1
		}
	}
	return out
}

// pep440 is a version as PEP 440 orders them. The local label (+ubuntu1) plays no
// part in picking a release, and is dropped.
type pep440 struct {
	epoch   int
	release []int
	pre     [2]int // kind (-1 a dev release of a final, 0 a, 1 b, 2 rc, 3 none) and number
	post    int    // -1 for none
	dev     int    // math.MaxInt for none
}

// pep440Pattern is PEP 440's own, with its alternative spellings.
var pep440Pattern = regexp.MustCompile(`(?i)^\s*v?(?:(\d+)!)?(\d+(?:\.\d+)*)` +
	`(?:[-_.]?(alpha|beta|preview|pre|a|b|c|rc)[-_.]?(\d+)?)?` +
	`(?:-(\d+)|[-_.]?(post|rev|r)[-_.]?(\d+)?)?` +
	`(?:[-_.]?(dev)[-_.]?(\d+)?)?` +
	`(?:\+[a-z0-9]+(?:[-_.][a-z0-9]+)*)?\s*$`)

// parsePEP440 reads a version; ok is false for one PEP 440 does not allow.
func parsePEP440(s string) (v pep440, ok bool) {
	m := pep440Pattern.FindStringSubmatch(s)
	if m == nil {
		return v, false
	}
	num := func(s string) int {
		if s == "" {
			return 0
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			ok = false
		}
		return n
	}
	ok = true
	v.epoch = num(m[1])
	for _, part := range strings.Split(m[2], ".") {
		v.release = append(v.release, num(part))
	}
	switch strings.ToLower(m[3]) {
	case "":
		v.pre[0] = 3
	case "a", "alpha":
		v.pre = [2]int{0, num(m[4])}
	case "b", "beta":
		v.pre = [2]int{1, num(m[4])}
	default:
		v.pre = [2]int{2, num(m[4])}
	}
	v.post, v.dev = -1, math.MaxInt
	switch {
	case m[5] != "":
		v.post = num(m[5])
	case m[6] != "":
		v.post = num(m[7])
	}
	if m[8] != "" {
		v.dev = num(m[9])
		if v.pre[0] == 3 && v.post < 0 {
			v.pre[0] = -1 // 1.0.dev1 comes before 1.0a1
		}
	}
	return v, ok
}

// prerelease reports whether the version is an alpha, beta, candidate or
// development release.
func (v pep440) prerelease() bool { return v.pre[0] != 3 || v.dev != math.MaxInt }

// compare orders two versions as PEP 440 does: 1.0.dev1 < 1.0a1 < 1.0 < 1.0.post1,
// and 1.0 == 1.0.0.
func (v pep440) compare(w pep440) int {
	if v.epoch != w.epoch {
		return cmp.Compare(v.epoch, w.epoch)
	}
	for i := 0; i < len(v.release) || i < len(w.release); i++ {
		var x, y int
		if i < len(v.release) {
			x = v.release[i]
		}
		if i < len(w.release) {
			y = w.release[i]
		}
		if x != y {
			return cmp.Compare(x, y)
		}
	}
	for _, d := range [...][2]int{{v.pre[0], w.pre[0]}, {v.pre[1], w.pre[1]}, {v.post, w.post}, {v.dev, w.dev}} {
		if d[0] != d[1] {
			return cmp.Compare(d[0], d[1])
		}
	}
	return 0
}
