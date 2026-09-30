package puppet

import "testing"

// A repository URL given by an anchor is read as the URL.
//
// Verifies: REQ-LANG-032
func TestFixturesAnchors(t *testing.T) {
	const fixtures = `fixtures:
  repositories:
    stdlib:
      repo: &repository https://github.com/puppetlabs/puppetlabs-stdlib.git
      ref: 9.0.0
    stdlib_copy:
      repo: *repository
      ref: 9.0.0
`
	dependencies := readFixtures([]byte(fixtures))
	if len(dependencies) != 2 || dependencies[1].origin != dependencies[0].origin || dependencies[0].origin == "" {
		for _, d := range dependencies {
			t.Errorf("%+v", d)
		}
	}
}
