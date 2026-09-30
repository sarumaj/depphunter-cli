package perl

import "testing"

// A requirements section given by an anchor is read as the section.
//
// Verifies: REQ-LANG-032
func TestMetaAnchors(t *testing.T) {
	m := readMetaYAML([]byte("name: App\nrequires: &requires\n  Moo: 2.0\nbuild_requires: *requires\n"))
	phases := map[string]string{}
	for _, r := range m.requirements {
		phases[r.phase] = r.module + " " + r.version
	}
	if phases["runtime"] != "Moo 2.0" || phases["build"] != "Moo 2.0" {
		t.Errorf("requirements %+v", m.requirements)
	}
}
