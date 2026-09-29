package racket

import "github.com/sarumaj/depphunter-cli/internal/lang"

// CatalogDependencies reads a package catalog's entry for a package - what
// `<catalog>/pkg/<name>` answers, a `read`-able hash table - and returns its
// `dependencies`, in the shape of an info.rkt's deps, as the targets the
// resolver makes of them: "base" and "racket" are the base collections, a
// `#:version` is a minimum, a source URL names the package raco derives from
// it. When the table itself has no dependencies, its `versions` table's
// `default` entry is read, as a catalog serving version-specific entries
// writes them. ok is false when the answer is not a hash table, which raco
// takes to mean the catalog does not have the package.
//
// Implements: REQ-RACKET-012
func CatalogDependencies(source []byte) (dependencies []lang.Target, ok bool) {
	module := read(source, false, false)
	if len(module.Forms) == 0 {
		return nil, false
	}
	entry, ok := hashTable(module.Forms[0])
	if !ok {
		return nil, false
	}
	value, listed := entry["dependencies"]
	if !listed {
		if versions, isTable := hashTable(entry["versions"]); isTable {
			if fallback, isTable := hashTable(versions["default"]); isTable {
				value = fallback["dependencies"]
			}
		}
	}
	r := &resolver{}
	for _, e := range elements(Unquote(value)) {
		if d, ok := readDependency(e); ok {
			dependencies = append(dependencies, r.dependencyTarget(d, ""))
		}
	}
	return dependencies, true
}

// hashTable reads #hash((key . value) ...) into its entries, by the symbol
// (or, in a versions table, the string) each is keyed by.
func hashTable(n *Node) (map[string]*Node, bool) {
	n = Unquote(n)
	if n == nil || n.Kind != List || n.Tag != "hash" && n.Tag != "hasheq" && n.Tag != "hasheqv" && n.Tag != "hashalw" {
		return nil, false
	}
	out := map[string]*Node{}
	for _, pair := range n.Kids {
		if pair.Kind != List || len(pair.Kids) != 3 || pair.Kids[1].Text != "." {
			continue
		}
		if key := pair.Kids[0]; key.Kind == Symbol || key.Kind == String {
			out[key.Text] = pair.Kids[2]
		}
	}
	return out, true
}
