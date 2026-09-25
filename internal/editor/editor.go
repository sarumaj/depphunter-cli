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

	"github.com/kballard/go-shellquote"
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
//
// Implements: REQ-SRV-006
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

// Command expands template for file (absolute) and line. The template is split with
// POSIX shell quoting rules (so quote Windows paths that contain backslashes), but no
// shell runs it: file names cannot inject commands.
//
// Implements: REQ-SEC-009, REQ-DIST-016, REQ-SRV-007
func Command(template, file string, line int) (*exec.Cmd, error) {
	args, err := shellquote.Split(template)
	if err != nil {
		return nil, fmt.Errorf("editor template %q: %w", template, err)
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
