package proto

import (
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// protocRoot is a directory a build script passes to protoc with -I or --proto_path.
type protocRoot struct {
	script    string // the directory of the script
	directory string // the import root, from the repository root
}

// buildScript reports whether a file is one that runs protoc in a build: a Makefile,
// a shell or PowerShell script, a justfile, a Taskfile or a CMake file.
func buildScript(p string) bool {
	base := path.Base(p)
	switch base {
	case "Makefile", "makefile", "GNUmakefile", "justfile", "Justfile", ".justfile",
		"Taskfile.yml", "Taskfile.yaml", "CMakeLists.txt":
		return true
	}
	switch strings.ToLower(path.Ext(base)) {
	case ".mk", ".sh", ".bash", ".zsh", ".ps1", ".cmake":
		return true
	}
	return false
}

// assignment is a variable a Makefile or shell script sets on a line of its own.
var assignment = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*(?:::=|:=|\?=|\+=|=)\s*(.*)$`)

// reference is a use of a variable: $(NAME), ${NAME} or $NAME.
var reference = regexp.MustCompile(`\$\(([A-Za-z_][A-Za-z0-9_]*)\)|\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// workingDirectory are the variables and commands that name the directory a script runs in.
var workingDirectory = []string{"$(CURDIR)", "$(PWD)", "${PWD}", "$PWD", "$(pwd)", "${CURDIR}", "`pwd`", "$(shell pwd)"}

// readProtocRoots finds the import roots the repository's build scripts give protoc:
// -I<dir>, -I <dir>, -I=<dir>, --proto_path=<dir> and --proto_path <dir> (a list
// separated by ':' or ';' too) in a script that mentions protoc. A variable the
// script sets on a line of its own is expanded, and the directory the script runs
// in stands for itself; a root is kept when it is a directory of the repository,
// read from the script's directory or else from the repository root (where scripts
// are often run from).
//
// Implements: REQ-PROTO-004
func readProtocRoots(all []*scan.File, directories map[string]bool) []protocRoot {
	var out []protocRoot
	for _, f := range all {
		if !buildScript(f.Path) || f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil || !strings.Contains(string(source), "protoc") && !strings.Contains(string(source), "proto_path") {
			continue
		}
		script := path.Dir(f.Path)
		for _, d := range protocFlags(string(source)) {
			for _, p := range []string{path.Join(script, d), path.Clean(d)} {
				if directories[p] && p != ".." && !strings.HasPrefix(p, "../") {
					if r := (protocRoot{script, p}); !slices.Contains(out, r) {
						out = append(out, r)
					}
					break
				}
			}
		}
	}
	return out
}

// protocFlags lists the directories a script's -I and --proto_path flags name, as
// written relative to the directory it runs in; absolute ones and ones a variable
// the script does not set is left in are dropped.
func protocFlags(source string) []string {
	source = strings.NewReplacer("\\\r\n", " ", "\\\n", " ").Replace(source)
	lines := strings.Split(source, "\n")
	variables := map[string]string{}
	for _, line := range lines {
		if m := assignment.FindStringSubmatch(line); m != nil {
			variables[m[1]] = strings.TrimSpace(variables[m[1]] + " " + m[2])
		}
	}
	var out []string
	for _, line := range lines {
		line = expandVariables(line, variables)
		fields := strings.Fields(line)
		for i := 0; i < len(fields); i++ {
			f := strings.Trim(fields[i], `"'`)
			var value string
			switch {
			case f == "-I" || f == "--proto_path":
				if i+1 < len(fields) {
					i++
					value = fields[i]
				}
			case strings.HasPrefix(f, "--proto_path="):
				value = strings.TrimPrefix(f, "--proto_path=")
			case strings.HasPrefix(f, "-I"):
				value = strings.TrimPrefix(strings.TrimPrefix(f, "-I"), "=")
			default:
				continue
			}
			value = strings.Trim(value, `"'`)
			if strings.Contains(value, `\`) || len(value) > 2 && value[1] == ':' && value[2] == '/' {
				continue // a Windows path: this machine's
			}
			for _, d := range strings.FieldsFunc(value, func(r rune) bool { return r == ':' || r == ';' }) {
				if d == "" || strings.ContainsAny(d, "$`()*") || path.IsAbs(d) {
					continue
				}
				out = append(out, d)
			}
		}
	}
	return out
}

// expandVariables replaces what names the current directory with ".", and the variables
// set in variables with their values, a few levels deep.
func expandVariables(line string, variables map[string]string) string {
	for _, c := range workingDirectory {
		line = strings.ReplaceAll(line, c+"/", "")
		line = strings.ReplaceAll(line, c, ".")
	}
	for range 4 {
		if !strings.Contains(line, "$") {
			break
		}
		line = reference.ReplaceAllStringFunc(line, func(match string) string {
			m := reference.FindStringSubmatch(match)
			if v, ok := variables[m[1]+m[2]+m[3]]; ok {
				return v
			}
			return match
		})
	}
	return line
}

// protocRootsFor are the protoc import roots an importer sees: those of the scripts
// in its directory or above it, the nearest script first, then every other one.
func (r *resolver) protocRootsFor(file string) []string {
	var near, far []string
	for d := range lang.Ancestors(file) {
		for _, root := range r.protoc {
			if root.script == d && !slices.Contains(near, root.directory) {
				near = append(near, root.directory)
			}
		}
	}
	for _, root := range r.protoc {
		if !slices.Contains(near, root.directory) && !slices.Contains(far, root.directory) {
			far = append(far, root.directory)
		}
	}
	return append(near, far...)
}
