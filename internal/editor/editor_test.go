package editor

import (
	"errors"
	"reflect"
	"testing"
)

// Verifies: REQ-SEC-009, REQ-DIST-016, REQ-SRV-007
func TestCommand(t *testing.T) {
	command, err := Command(`"/opt/My Editor/bin/ed" --goto '{file}:{line}'`, "/src/a b.go", 12)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/opt/My Editor/bin/ed", "--goto", "/src/a b.go:12"}; !reflect.DeepEqual(command.Args, want) {
		t.Errorf("args %q, want %q", command.Args, want)
	}
	// A hostile file name stays one argument.
	command, _ = Command("code -g {file}", "/x; rm -rf ~", 0)
	if len(command.Args) != 3 || command.Args[2] != "/x; rm -rf ~" {
		t.Errorf("args %q", command.Args)
	}
	for _, bad := range []string{"", "code -g", `code "{file}`} {
		if _, err := Command(bad, "/f", 1); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

// Verifies: REQ-SRV-006
func TestDetect(t *testing.T) {
	environment := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
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
		name        string
		environment map[string]string
		path        []string
		want        string
	}{
		{"GUI $VISUAL wins", map[string]string{"VISUAL": "/usr/local/bin/zed", "EDITOR": "vim"}, []string{"code"}, "/usr/local/bin/zed {file}:{line}"},
		{"terminal $EDITOR ignored", map[string]string{"EDITOR": "nvim"}, []string{"subl"}, "subl {file}:{line}"},
		{"nothing found", nil, nil, ""},
	}
	for _, c := range cases {
		if got := Detect(environment(c.environment), onPath(c.path...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
