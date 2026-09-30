package dart

import "testing"

// A dependency's constraint given by an anchor is read as the constraint.
//
// Verifies: REQ-LANG-032
func TestPubspecAnchors(t *testing.T) {
	p, err := readPubspec([]byte("name: app\ndependencies:\n  path: &version ^1.9.0\n  http: *version\n"))
	if err != nil || len(p.dependencies) != 2 {
		t.Fatalf("readPubspec: %v, %d dependencies", err, len(p.dependencies))
	}
	if d := p.dependencies[1]; d.name != "http" || d.constraint != "^1.9.0" {
		t.Errorf("http: %+v", d)
	}
}
