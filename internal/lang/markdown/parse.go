package markdown

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Link is one link a document carries, as written.
type Link struct {
	// Spec is the link as it appears, for the side panel to show.
	Spec string
	// Dest is where it points, with any fragment and query still attached and any
	// angle brackets and title taken off.
	Dest string
	Line int
	// Col is where on the line it starts, counted in bytes from 1, which is what
	// tells two broken links on one line apart.
	Col int
	// Ref is the label of a reference link whose definition is missing; Dest is then
	// empty. Such a link renders as literal text, which is a break that reading the
	// rendered page does not reveal.
	Ref string
}

// Heading is one heading of a document.
type Heading struct {
	Text  string
	Level int
	Line  int
}

var (
	// An inline link or image. The destination runs to the closing parenthesis,
	// balancing one level of nesting, which is as deep as destinations go.
	inlineLink = regexp.MustCompile(`!?\[(?:[^\[\]]|\[[^\[\]]*\])*\]\(([^()\s]*(?:\([^()]*\)[^()\s]*)*)(?:\s+(?:"[^"]*"|'[^']*'|\([^()]*\)))?\s*\)`)
	// A reference definition at the start of a line: [label]: destination "title"
	refDefinition = regexp.MustCompile(`^ {0,3}\[([^\]]+)\]:\s*(\S+)`)
	// A full or collapsed reference use: [text][label] or [label][]. The shortcut
	// form, [label] alone, is not read: it cannot be told from bracketed prose.
	refUse = regexp.MustCompile(`!?\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\[([^\]]*)\]`)
	// An autolink: <https://example.test>
	autolink = regexp.MustCompile(`<((?:https?|ftp):[^>\s]+)>`)
	// href and src of raw HTML, which is how a README carries its badges.
	htmlAttr = regexp.MustCompile(`(?i)<[a-z][^>]*\b(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	// An anchor a document sets itself: <a name="x">, id="x", or the {#x} that
	// several renderers take at the end of a heading.
	htmlAnchor   = regexp.MustCompile(`(?i)\b(?:name|id)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	customAnchor = regexp.MustCompile(`\{#([^}\s]+)\}\s*$`)
	codeSpan     = regexp.MustCompile("`+[^`]*`+")
	atx          = regexp.MustCompile(`^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$`)
	setext       = regexp.MustCompile(`^ {0,3}(=+|-+)\s*$`)
)

// Links reads every link in a document: inline links and images, reference
// definitions, autolinks, the href and src of raw HTML, and the reference uses whose
// definition is missing.
//
// Fenced code is skipped, and so are code spans, because a link inside either is
// printed rather than followed and checking it would report a defect in an example.
//
// Implements: REQ-MD-003, REQ-MD-004, REQ-MD-006, REQ-MD-009
func Links(src []byte) []Link {
	var out []Link
	defined := map[string]bool{}
	type use struct {
		label, spec string
		line, col   int
	}
	var uses []use

	for i, raw := range split(src) {
		if raw.code {
			continue
		}
		// Code spans are blanked rather than cut, so that a match index still points
		// into the line as written - which is where the quoted link is taken from.
		line, text := i+1, codeSpan.ReplaceAllStringFunc(raw.text, blank)

		if m := refDefinition.FindStringSubmatch(text); m != nil {
			defined[label(m[1])] = true
			out = append(out, Link{Spec: strings.TrimSpace(text), Dest: destination(m[2]), Line: line, Col: 1})
			continue
		}
		for _, at := range inlineLink.FindAllStringSubmatchIndex(text, -1) {
			if dest := destination(group(text, at, 1)); dest != "" {
				out = append(out, Link{Spec: group(raw.text, at, 0), Dest: dest, Line: line, Col: at[0] + 1})
			}
		}
		for _, at := range autolink.FindAllStringSubmatchIndex(text, -1) {
			out = append(out, Link{Spec: group(raw.text, at, 0), Dest: group(text, at, 1), Line: line, Col: at[0] + 1})
		}
		for _, at := range htmlAttr.FindAllStringSubmatchIndex(text, -1) {
			if dest := destination(group(text, at, 1) + group(text, at, 2)); dest != "" {
				out = append(out, Link{
					Spec: strings.TrimSpace(group(raw.text, at, 0)), Dest: dest, Line: line, Col: at[0] + 1,
				})
			}
		}
		for _, at := range refUse.FindAllStringSubmatchIndex(text, -1) {
			name := group(text, at, 2)
			if strings.TrimSpace(name) == "" {
				name = group(text, at, 1) // [label][] names itself
			}
			uses = append(uses, use{label(name), group(raw.text, at, 0), line, at[0] + 1})
		}
	}
	// A definition may follow its uses, so the uses are judged once the whole
	// document has been read.
	for _, u := range uses {
		if u.label != "" && !defined[u.label] {
			out = append(out, Link{Spec: u.spec, Ref: u.label, Line: u.line, Col: u.col})
		}
	}
	return out
}

// Headings reads the document's headings, which are what a fragment names and what a
// Markdown file expands into on the map.
//
// Implements: REQ-MD-002
func Headings(src []byte) []Heading {
	var out []Heading
	all := split(src)
	for i, raw := range all {
		if raw.code {
			continue
		}
		if m := atx.FindStringSubmatch(raw.text); m != nil {
			out = append(out, Heading{Text: m[2], Level: len(m[1]), Line: i + 1})
			continue
		}
		// Setext: the heading is the line above a row of = or -, and a row with
		// nothing above it is a horizontal rule.
		if i > 0 && !all[i-1].code && setext.MatchString(raw.text) {
			above := strings.TrimSpace(all[i-1].text)
			if above == "" || atx.MatchString(all[i-1].text) {
				continue
			}
			level := 2
			if strings.HasPrefix(strings.TrimSpace(raw.text), "=") {
				level = 1
			}
			out = append(out, Heading{Text: above, Level: level, Line: i})
		}
	}
	return out
}

// Anchors is every name a fragment in this document may point at: the slug a renderer
// gives each heading, with GitHub's numbering where one repeats, plus the anchors the
// document sets itself.
//
// Implements: REQ-MD-008
func Anchors(src []byte) map[string]bool {
	out := map[string]bool{}
	seen := map[string]int{}
	for _, h := range Headings(src) {
		text := h.Text
		if m := customAnchor.FindStringSubmatch(text); m != nil {
			out[strings.ToLower(m[1])] = true
			text = customAnchor.ReplaceAllString(text, "")
		}
		slug := Slug(text)
		if slug == "" {
			continue
		}
		if n := seen[slug]; n > 0 {
			out[slug+"-"+strconv.Itoa(n)] = true
		} else {
			out[slug] = true
		}
		seen[slug]++
	}
	for _, m := range htmlAnchor.FindAllSubmatch(src, -1) {
		if name := string(m[1]) + string(m[2]); name != "" {
			out[strings.ToLower(name)] = true
		}
	}
	return out
}

// Slug is the anchor a renderer derives from a heading: its text with the formatting
// taken off, lower-cased, everything but letters, digits, spaces, hyphens and
// underscores dropped, and the spaces turned into hyphens.
//
// Implements: REQ-MD-008
func Slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(plain(text)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// emphasis is a run wrapped in underscores. Asterisks, backticks and tildes are cut
// wherever they appear, but an underscore is as often part of a word as it is a mark,
// and a slug keeps the one in snake_case.
var emphasis = regexp.MustCompile(`_{1,2}([^_\s][^_]*?)_{1,2}`)

// plain takes the inline formatting off a heading, keeping what is read: the text of
// a link rather than its destination, the contents of code and emphasis rather than
// their marks.
func plain(text string) string {
	text = inlineLink.ReplaceAllStringFunc(text, func(s string) string {
		if i := strings.Index(s, "]("); i > 0 {
			return strings.TrimPrefix(s[:i], "!")[1:]
		}
		return s
	})
	text = emphasis.ReplaceAllString(text, "$1")
	return strings.TrimSpace(strings.NewReplacer("`", "", "*", "", "~", "").Replace(text))
}

// destination strips what surrounds a link's target: the angle brackets it may be
// wrapped in, and the title that may follow it.
func destination(d string) string {
	d = strings.TrimSpace(d)
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(d, "<"), ">"))
}

// label normalizes a reference's name, which is matched case-insensitively.
func label(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// group is submatch n of a match found by index, or empty where it did not
// participate. Matching by index is what gives each link its column.
func group(text string, at []int, n int) string {
	if 2*n+1 >= len(at) || at[2*n] < 0 {
		return ""
	}
	return text[at[2*n]:at[2*n+1]]
}

// line is one line of a document and whether it is inside fenced code.
type line struct {
	text string
	code bool
}

// split cuts a document into lines, marking the fenced code - the fence lines
// included. A fence closes on a run of the same character at least as long as the one
// that opened it, which is what lets a ```` block hold ``` lines.
//
// Implements: REQ-MD-004
func split(src []byte) []line {
	var out []line
	fence := ""
	for _, text := range strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n") {
		marker := fenceOn(text)
		switch {
		case fence == "":
			out = append(out, line{text, marker != ""})
			fence = marker
		case marker != "" && marker[0] == fence[0] && len(marker) >= len(fence):
			out = append(out, line{text, true})
			fence = ""
		default:
			out = append(out, line{text, true})
		}
	}
	return out
}

// fenceOn is the fence a line opens or closes with, or empty.
func fenceOn(text string) string {
	trimmed := strings.TrimLeft(text, " ")
	for _, c := range []byte{'`', '~'} {
		if n := runLen(trimmed, c); n >= 3 {
			return trimmed[:n]
		}
	}
	return ""
}

func runLen(s string, c byte) int {
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	return n
}

func blank(s string) string { return strings.Repeat(" ", len(s)) }
