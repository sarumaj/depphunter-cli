package r

import (
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// R Markdown (.Rmd) and Quarto (.qmd) documents are prose with code chunks. Only the
// R chunks are code of the project's: ```{r label, echo=FALSE} up to the closing
// fence. Each chunk is lexed on its own with its own line numbers, so what the
// document defines and imports points at its lines, and a chunk that leaves a
// bracket open does not swallow the next one. Chunks of other engines (python,
// bash, sql) are skipped.

var (
	chunkOpen   = regexp.MustCompile("^(\\s*)(`{3,})\\s*\\{\\s*[rR]([\\s,}].*)?$")
	fence       = regexp.MustCompile("^\\s*(`{3,})\\s*$")
	childOption = regexp.MustCompile(`\bchild\s*=\s*(?:c\()?\s*["']([^"']+)["']`)
	include     = regexp.MustCompile(`\{\{<\s*include\s+([^\s>]+)\s*>\}\}`)
)

// extractDocument reads a document's R chunks, and the documents it pulls in: a
// knitr chunk's child option and Quarto's include shortcode.
//
// Implements: REQ-R-001, REQ-R-004
func extractDocument(source []byte) *lang.Extraction {
	lines := strings.Split(string(source), "\n")
	var tokens []token
	var comments []comment
	var children []lang.RawImport
	seen := map[string]bool{}
	child := func(p string, line int) {
		p = path.Clean(p)
		if !seen[p] {
			seen[p] = true
			children = append(children, lang.RawImport{Spec: "child: " + p, Module: p, Name: kindChild, Line: line})
		}
	}
	for i := 0; i < len(lines); i++ {
		for _, m := range include.FindAllStringSubmatch(lines[i], -1) {
			child(m[1], i+1)
		}
		m := chunkOpen.FindStringSubmatch(lines[i])
		if m == nil {
			// Any other fenced block is skipped whole: a chunk shown as text in
			// a ````markdown block is not code.
			if trimmed := strings.TrimSpace(lines[i]); strings.HasPrefix(trimmed, "```") {
				n := len(trimmed) - len(strings.TrimLeft(trimmed, "`"))
				for i++; i < len(lines); i++ {
					if f := fence.FindStringSubmatch(lines[i]); f != nil && len(f[1]) >= n {
						break
					}
				}
			}
			continue
		}
		for _, c := range childOption.FindAllStringSubmatch(m[3], -1) {
			child(c[1], i+1)
		}
		start := i + 1
		end := start
		for end < len(lines) {
			if f := fence.FindStringSubmatch(lines[end]); f != nil && len(f[1]) >= len(m[2]) {
				break
			}
			end++
		}
		body := strings.Join(lines[start:min(end, len(lines))], "\n")
		t, c := lex([]byte(body), start+1)
		tokens = append(tokens, t...)
		tokens = append(tokens, token{k: tSeparator, line: end + 1})
		comments = append(comments, c...)
		i = end
	}
	extraction := extractCode(newCode(tokens), comments)
	extraction.Imports = append(children, extraction.Imports...)
	return extraction
}
