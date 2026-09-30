package docker

import (
	"reflect"
	"sort"
	"testing"
)

// A merge key that names its own mapping, a merge cycle through two anchors and an
// alias to its own container end the walk: what does not depend on the cycle is read
// as usual.
//
// Verifies: REQ-LANG-032
func TestComposeCycles(t *testing.T) {
	const compose = `x-self: &self
  <<: *self
  image: nginx:1.27
x-first: &first
  <<: &second
    <<: *first
    image: redis:7
  restart: always
include: &include [*include]
services:
  merged:
    <<: *self
  cycle:
    <<: *first
  looped: &looped
    image: busybox:1.36
    labels: *looped
`
	extraction := extractCompose([]byte(compose))
	var specs, symbols []string
	for _, rawImport := range extraction.Imports {
		specs = append(specs, rawImport.Spec)
	}
	for _, symbol := range extraction.Symbols {
		symbols = append(symbols, symbol.Name)
	}
	sort.Strings(specs)
	if want := []string{"image: busybox:1.36", "image: nginx:1.27", "image: redis:7"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
	if want := []string{"merged", "cycle", "looped"}; !reflect.DeepEqual(symbols, want) {
		t.Errorf("symbols %q, want %q", symbols, want)
	}
}
