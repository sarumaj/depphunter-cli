// Package editor opens files in the user's editor from command templates such as
// "code -g {file}:{line}".
package editor

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// known maps GUI editor binaries to templates. Terminal editors (vim, nano, helix…)
// are deliberately absent: launched from the server they would fight it for the terminal.
var known = []struct{ bin, template string }{
	{"code", "code -g {file}:{line}"},
	{"cursor", "cursor -g {file}:{line}"},
	{"codium", "codium -g {file}:{line}"},
	{"zed", "zed {file}:{line}"},
	{"subl", "subl {file}:{line}"},
	{"idea", "idea --line {line} {file}"},
	{"goland", "goland --line {line} {file}"},
	{"pycharm", "pycharm --line {line} {file}"},
	{"webstorm", "webstorm --line {line} {file}"},
	{"mate", "mate -l {line} {file}"},
	{"kate", "kate --line {line} {file}"},
	{"gedit", "gedit +{line} {file}"},
	{"gvim", "gvim +{line} {file}"},
	{"emacsclient", "emacsclient -n +{line} {file}"},
}

// Detect picks a template: $VISUAL or $EDITOR when it names a known GUI editor,
// otherwise the first known editor on PATH; "" when none is found.
func Detect(getenv func(string) string, lookPath func(string) (string, error)) string {
	for _, v := range []string{getenv("VISUAL"), getenv("EDITOR")} {
		if fields := strings.Fields(v); len(fields) > 0 {
			base := strings.TrimSuffix(filepath.Base(fields[0]), ".exe")
			for _, k := range known {
				if k.bin == base {
					return strings.Replace(k.template, k.bin, fields[0], 1)
				}
			}
		}
	}
	for _, k := range known {
		if _, err := lookPath(k.bin); err == nil {
			return k.template
		}
	}
	return ""
}

// Command expands template for file (absolute) and line. Arguments are split like a
// shell would split plain words and quoted strings, but no shell is involved, so file
// names cannot inject commands.
func Command(template, file string, line int) (*exec.Cmd, error) {
	args, err := split(template)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 || !strings.Contains(template, "{file}") {
		return nil, errors.New("editor template must name a program and contain {file}")
	}
	r := strings.NewReplacer("{file}", file, "{line}", strconv.Itoa(max(line, 1)))
	for i := range args {
		args[i] = r.Replace(args[i])
	}
	return exec.Command(args[0], args[1:]...), nil
}

func split(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg := false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote, inArg = r, true
		case r == ' ' || r == '\t':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in editor template %q", s)
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}
