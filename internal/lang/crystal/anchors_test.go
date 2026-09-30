package crystal

import "testing"

// A dependency's version given by an anchor is read as the version.
//
// Verifies: REQ-LANG-032
func TestShardAnchors(t *testing.T) {
	sh := readShard([]byte("name: app\ndependencies:\n  kemal:\n    github: kemalcr/kemal\n    version: &version ~> 1.4\n  db:\n    github: crystal-lang/crystal-db\n    version: *version\n"))
	if d := sh.dependencies["db"]; d == nil || d.version != "~> 1.4" {
		t.Errorf("db: %+v", d)
	}
}
