package editor

import (
	"errors"
	"reflect"
	"testing"
)

// Verifies: REQ-SEC-009, REQ-DIST-016, REQ-SRV-007
func TestCommand(t *testing.T) {
	cmd, err := Command(`"/opt/My Editor/bin/ed" --goto '{file}:{line}'`, "/src/a b.go", 12)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/opt/My Editor/bin/ed", "--goto", "/src/a b.go:12"}; !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args %q, want %q", cmd.Args, want)
	}
	// A hostile file name stays one argument.
	cmd, _ = Command("code -g {file}", "/x; rm -rf ~", 0)
	if len(cmd.Args) != 3 || cmd.Args[2] != "/x; rm -rf ~" {
		t.Errorf("args %q", cmd.Args)
	}
	for _, bad := range []string{"", "code -g", `code "{file}`} {
		if _, err := Command(bad, "/f", 1); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

// Verifies: REQ-SRV-006
func TestDetect(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	onPath := func(bins ...string) func(string) (string, error) {
		return func(b string) (string, error) {
			for _, x := range bins {
				if x == b {
					return "/usr/bin/" + b, nil
				}
			}
			return "", errors.New("not found")
		}
	}
	cases := []struct {
		name string
		env  map[string]string
		path []string
		want string
	}{
		{"GUI $VISUAL wins", map[string]string{"VISUAL": "/usr/local/bin/zed", "EDITOR": "vim"}, []string{"code"}, "/usr/local/bin/zed {file}:{line}"},
		{"terminal $EDITOR ignored", map[string]string{"EDITOR": "nvim"}, []string{"subl"}, "subl {file}:{line}"},
		{"nothing found", nil, nil, ""},
	}
	for _, c := range cases {
		if got := Detect(env(c.env), onPath(c.path...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
