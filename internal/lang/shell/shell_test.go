package shell

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a small project's scripts: a build script finding its libraries by
// every common idiom for its own directory (a SCRIPT_DIR variable, dirname "$0",
// ${BASH_SOURCE%/*}) and the repository root, running other scripts by path and
// through interpreters, with paths it cannot know (the home directory, absolute
// paths, unknown variables) and text that only looks like commands (a comment, a
// here-document, strings, a case inside a command substitution); an extensionless
// script with a bash shebang; a CI script installing tools with five package
// managers; a zsh plugin (${0:A:h}, global aliases, typeset); a bats test with load
// and run; and two direnv .envrc files.
//
// Verifies: REQ-SHELL-001, REQ-SHELL-002, REQ-SHELL-004, REQ-SHELL-005, REQ-SHELL-006
// Verifies: REQ-SHELL-007, REQ-SHELL-008, REQ-SHELL-010
func TestSourcesInvocationsAndInstalls(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	local := func(p string) lang.Target { return lang.Target{Local: p} }
	packageName := func(ecosystem, name, version string, pinned, floating bool) lang.Target {
		return lang.Target{Ecosystem: ecosystem, Package: name, Version: version, Pinned: pinned, Floating: floating}
	}
	common, logb := local("scripts/lib/common.sh"), local("scripts/lib/log.bash")
	imports := map[string]map[string]lang.Target{
		"scripts/build.sh": {
			`source "$SCRIPT_DIR/lib/common.sh"`:       common,
			`. "$(dirname "$0")/lib/log.bash"`:         logb,
			`source "${BASH_SOURCE%/*}/lib/common.sh"`: common,
			`"$SCRIPT_DIR/deploy"`:                     local("scripts/deploy"),
			`bash scripts/lib/log.bash`:                logb,
			`python3 -u "$ROOT_DIR/scripts/gen.py"`:    local("scripts/gen.py"),
			`./scripts/deploy`:                         local("scripts/deploy"),
		},
		"scripts/lib/common.sh": {`source "$(dirname "${BASH_SOURCE[0]}")/log.bash"`: logb},
		"scripts/lib/log.bash":  {},
		"scripts/deploy": {
			`source "$DIR/lib/common.sh"`:    common,
			`"$DIR"/../scripts/lib/log.bash`: logb,
		},
		"scripts/ci.sh": {
			"pip install pip":                       packageName(ecosystemPyPI, "pip", "", false, true),
			"pip install -r requirements-dev.txt":   local("requirements-dev.txt"),
			`pip install "requests[socks]==2.31.0"`: packageName(ecosystemPyPI, "requests", "2.31.0", true, false),
			`pip install 'flask>=2.0'`:              packageName(ecosystemPyPI, "flask", ">=2.0", false, false),
			"python3 -m pip install black==24.1.0":  packageName(ecosystemPyPI, "black", "24.1.0", true, false),
			"npm install typescript@5.3.3":          packageName(ecosystemNPM, "typescript", "5.3.3", true, false),
			"npm install @angular/cli@^17":          packageName(ecosystemNPM, "@angular/cli", "^17", false, false),
			"npm install yarn":                      packageName(ecosystemNPM, "yarn", "", false, true),
			"go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_VERSION}": packageName(ecosystemGo, "github.com/golangci/golangci-lint", "v1.55.2", true, false),
			"go install golang.org/x/tools/cmd/goimports@latest":                                 packageName(ecosystemGo, "golang.org/x/tools", "latest", false, true),
			"go install golang.org/x/tools/gopls@v0.14.2":                                        packageName(ecosystemGo, "golang.org/x/tools/gopls", "v0.14.2", true, false),
			"cargo install ripgrep":                                                              packageName(ecosystemCrates, "ripgrep", "13.0.0", true, false),
			"cargo install cargo-edit@0.12":                                                      packageName(ecosystemCrates, "cargo-edit", "0.12", false, false),
			"gem install bundler":                                                                packageName(ecosystemGems, "bundler", "2.5.3", true, false),
			"gem install rake:13.1.0":                                                            packageName(ecosystemGems, "rake", "13.1.0", true, false),
			"gem install rubocop":                                                                packageName(ecosystemGems, "rubocop", "", false, true),
		},
		"tools/setup.zsh": {
			"source ${0:A:h}/../scripts/lib/common.sh": common,
			`source "${ZSH_PLUGIN_DIR}/helpers.zsh"`:   local("tools/helpers.zsh"),
		},
		"tools/helpers.zsh": {},
		"test/app.bats": {
			"load test_helper":       local("test/test_helper.bash"),
			"load 'helpers/missing'": {},
			`source "$BATS_TEST_DIRNAME/../scripts/lib/common.sh"`: common,
			`"$BATS_TEST_DIRNAME/../scripts/build.sh"`:             local("scripts/build.sh"),
		},
		"test/test_helper.bash": {},
		".envrc": {
			"source_env_if_exists .envrc.local": {},
			"dotenv":                            local(".env"),
			"source_env sub":                    local("sub/.envrc"),
		},
		"sub/.envrc": {"source_up": local(".envrc"), "dotenv_if_exists ../.env": local(".env")},
	}
	for file, want := range imports {
		t.Run(file, func(t *testing.T) { langtest.CheckImports(t, results[file], want) })
	}
	if len(results) != len(imports) {
		var got []string
		for p := range results {
			got = append(got, p)
		}
		t.Errorf("analyzed %v, want the %d files above (scripts/notes has no shebang, scripts/gen.py is Python)", got, len(imports))
	}
}

// Verifies: REQ-SHELL-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	want := map[string]map[string]string{
		"scripts/build.sh": {
			"SCRIPT_DIR": "var", "ROOT_DIR": "var", "VERSION": "const", "BUILD_MODE": "var",
			"build": "function", "clean": "function",
		},
		"scripts/lib/common.sh": {"COMMON_LOADED": "var", "die": "function", "ll": "alias"},
		"scripts/lib/log.bash":  {"log_info": "function", "log_warn": "function"},
		"scripts/deploy":        {"DIR": "var", "deploy_main": "function"},
		"scripts/ci.sh":         {"GOLANGCI_VERSION": "var"},
		"tools/setup.zsh": {
			"ZSH_PLUGIN_DIR": "var", "G": "alias", "gs": "alias", "EDITOR": "var", "zsh_greet": "function",
		},
		"tools/helpers.zsh":     {"ZSH_HELPERS": "var"},
		"test/app.bats":         {"setup": "function", "build prints usage": "test"},
		"test/test_helper.bash": {"FIXTURES": "var"},
		".envrc":                {"DATABASE_URL": "var"},
		"sub/.envrc":            {},
	}
	for file, w := range want {
		langtest.CheckSymbols(t, results[file], w)
	}
	lines := map[string]int{}
	for _, s := range results["scripts/build.sh"].Symbols {
		lines[s.Name] = s.Line
	}
	if lines["build"] != 19 || lines["clean"] != 32 || lines["VERSION"] != 7 {
		t.Errorf("symbol lines: %v", lines)
	}
}

// The scanner keeps going through what trips simpler readers: a "$" before ")", a
// case pattern's ")" inside a command substitution, arrays with parentheses in
// strings and comments, backquotes, here-documents with quoted and tab-stripped
// delimiters, and comments that are not comments (${#x}, a#b).
//
// Verifies: REQ-SHELL-002, REQ-SHELL-010
func TestScannerSurvives(t *testing.T) {
	source := `#!/bin/sh
grep -E "(^| )${key}( |$)" file
v=$(case "$x" in a) echo ")" ;; (b|c) echo b ;; esac)
arr=(
  one # a comment with ) and '
  "two )"
)
n=${#arr[@]} m=a#b
old=` + "`dirname \\`pwd\\``" + `
cat <<'EOF'
source not-this.sh
EOF
	cat <<-END
	./not-this-either.sh
	END
empty=()
x=$(f() { source in-function.sh; }; f)
source after.sh
`
	extraction := extract([]byte(source), ".sh")
	var specs []string
	for _, rawImport := range extraction.Imports {
		specs = append(specs, rawImport.Spec)
	}
	want := []string{"source in-function.sh", "source after.sh"}
	if !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
	if n := len(extraction.Imports); n == 2 && extraction.Imports[1].Line != 18 {
		t.Errorf("source after.sh on line %d, want 18", extraction.Imports[1].Line)
	}
}

// A file cut off inside a quote, an expansion, an escape, a here-document or a
// substitution is read up to its end without failing.
//
// Verifies: REQ-SHELL-002
func TestTruncatedScripts(t *testing.T) {
	for _, source := range []string{"echo 'abc", "x=\\", "echo ${a", "cat <<EOF\nsource a.sh", "v=$(source a.sh", "echo `ls", "a=(1 2", "case x in a) f", "f() {", "$'\\"} {
		extract([]byte(source), ".sh")
	}
}

// Verifies: REQ-SHELL-004, REQ-SHELL-010
func TestPathIdioms(t *testing.T) {
	for source, want := range map[string]string{
		`source "$(dirname "$0")/a.sh"`:                                                 "dir:a.sh",
		`source "$(dirname -- "$(readlink -f -- "$0")")/a.sh"`:                          "dir:a.sh",
		`source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)/a.sh"`: "dir:a.sh",
		`D="$(cd "$(dirname "$0")/.." && pwd)"; source "$D/a.sh"`:                       "dir:../a.sh",
		`source "${0:A:h}/a.zsh"`:                                                       "dir:a.zsh",
		`source ${${(%):-%x}:a:h}/a.zsh`:                                                "dir:a.zsh",
		`source "$0:A:h/a.zsh"`:                                                         "dir:a.zsh",
		`SRC="${BASH_SOURCE[0]}"; D="$(dirname "$SRC")"; . "$D/a.sh"`:                   "dir:a.sh",
		`R=$(git rev-parse --show-toplevel); source "$R/ci/a.sh"`:                       "root:ci/a.sh",
		`source "$PWD/a.sh"`:                                                            "cwd:a.sh",
		`source "${D:-$(dirname "$0")}/a.sh"`:                                           "dir:a.sh",
		`source lib/a.sh`:                                                               "cwd:lib/a.sh",
		`source "$PLUGIN_PATH/common/functions"`:                                        "any:common/functions",
		`source "$PLUGIN_PATH/functions"`:                                               "",
		`source ~/.bashrc`:                                                              "",
		`source "$HOME/x/a.sh"`:                                                         "",
		`source /etc/profile`:                                                           "",
		`source "$(dirname "$0")/lib/$1.sh"`:                                            "",
		`source "$(dirname "$0")"/*.sh`:                                                 "",
	} {
		extraction := extract([]byte(source), ".sh")
		got := ""
		if len(extraction.Imports) > 0 {
			got = extraction.Imports[len(extraction.Imports)-1].Module
		}
		if got != want {
			t.Errorf("%s: module %q, want %q", source, got, want)
		}
	}
}

// Verifies: REQ-SHELL-009
func TestEnvironmentPathsByTheirEnd(t *testing.T) {
	files := []*scan.File{{Path: "plugins/common/functions"}, {Path: "plugins/git/functions"}, {Path: "a/lib/x.sh"}, {Path: "b/lib/x.sh"}}
	r := newResolver(files)
	for module, want := range map[string]string{
		"any:common/functions":           "plugins/common/functions",
		"any:plugins/git/functions":      "plugins/git/functions",
		"any:lib/x.sh":                   "", // two files end so
		"any:available/common/functions": "",
	} {
		if got := r.Resolve("plugins/git/commands", lang.RawImport{Module: module, Name: kindSource}); got.Local != want {
			t.Errorf("%s: %+v, want %q", module, got, want)
		}
	}
}

// Verifies: REQ-SHELL-001
func TestClaims(t *testing.T) {
	for _, c := range []struct {
		f    scan.File
		want bool
	}{
		{scan.File{Path: "a.sh"}, true},
		{scan.File{Path: "lib/x.BASH"}, true},
		{scan.File{Path: "themes/r.zsh-theme"}, true},
		{scan.File{Path: "t/x.bats"}, true},
		{scan.File{Path: "home/.zshrc"}, true},
		{scan.File{Path: ".envrc"}, true},
		{scan.File{Path: "bin/deploy", Interpreter: "bash"}, true},
		{scan.File{Path: "bin/tool", Interpreter: "python3"}, false},
		{scan.File{Path: "bin/tool.py", Interpreter: "sh"}, false},
		{scan.File{Path: "a.sh", Binary: true}, false},
		{scan.File{Path: "README"}, false},
	} {
		if got := (Plugin{}).Claims(&c.f); got != c.want {
			t.Errorf("%s (%q): %v, want %v", c.f.Path, c.f.Interpreter, got, c.want)
		}
	}
}
