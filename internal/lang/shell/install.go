package shell

import (
	"regexp"
	"strings"
)

// Packages a script installs with a language's package manager are dependencies of
// the project as much as the ones its manifests list: a CI script's
// "pip install awscli==1.32.0" or "go install ...golangci-lint@v1.55.2" decides what
// runs in the build. They land in the ecosystem the package manager serves (pypi,
// npm, go, crates, rubygems), under the same names and pinning rules as the
// manifests' packages, so vulnerability lookups and pinning reports cover them.
// System package managers (apt-get, apk, brew, dnf, yum) are not read (REQ-SHELL-010).

// install records the packages a package-manager command installs, and reports
// whether the command was one.
//
// Implements: REQ-SHELL-007, REQ-SHELL-008
func (x *extractor) install(words []*word, line int) bool {
	cmd := name(words[0])
	sub := func(i int) string {
		if i < len(words) {
			return name(words[i])
		}
		return ""
	}
	switch {
	case pipCommand.MatchString(cmd) && sub(1) == "install":
		x.pip(words, 2, line)
	case pythonCommand.MatchString(cmd) && sub(1) == "-m" && sub(2) == "pip" && sub(3) == "install":
		x.pip(words, 4, line)
	case cmd == "uv" && sub(1) == "pip" && sub(2) == "install", cmd == "uv" && sub(1) == "tool" && sub(2) == "install":
		x.pip(words, 3, line)
	case cmd == "pipx" && sub(1) == "install":
		x.pip(words, 2, line)
	case cmd == "npm" && (sub(1) == "install" || sub(1) == "i" || sub(1) == "add" || sub(1) == "isntall"),
		cmd == "pnpm" && (sub(1) == "add" || sub(1) == "install" || sub(1) == "i"),
		cmd == "yarn" && sub(1) == "add", cmd == "bun" && (sub(1) == "add" || sub(1) == "install" || sub(1) == "i"):
		x.npm(words, 2, line)
	case cmd == "yarn" && sub(1) == "global" && sub(2) == "add":
		x.npm(words, 3, line)
	case cmd == "go" && (sub(1) == "install" || sub(1) == "run" || sub(1) == "get"):
		x.golang(words, 2, line)
	case cmd == "cargo" && (sub(1) == "install" || sub(1) == "binstall"):
		x.cargo(words, 2, line)
	case cmd == "gem" && sub(1) == "install":
		x.gem(words, 2, line)
	default:
		return false
	}
	return true
}

var (
	pipCommand    = regexp.MustCompile(`^pip[0-9.]*$`)
	pythonCommand = regexp.MustCompile(`^python[0-9.]*$`)
	pipName       = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)\s*(\[[^\]]*\])?\s*(.*)$`)
	npmName       = regexp.MustCompile(`^(@[a-z0-9][a-z0-9._~-]*/)?[a-z0-9_][a-z0-9._~-]*$`)
	crateName     = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// text evaluates a package argument: literals, and variables whose value the file
// assigned; a variable whose value is unknown is kept as written (${VERSION}), so
// a version computed at run time shows but pins nothing.
func (x *extractor) text(w *word) (string, bool) {
	var b strings.Builder
	for _, p := range w.parts {
		switch p.kind {
		case pLit:
			b.WriteString(p.text)
		case pParam:
			if v, ok := x.vars[p.text]; ok && !strings.ContainsAny(v, markers) {
				b.WriteString(v)
			} else {
				b.WriteString("${" + p.text + "}")
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

// args walks a package manager's arguments from index i: options (and the values
// of those in valued) are skipped, and each remaining argument is passed to fn with
// its text. An option fn handles itself makes it return true.
func (x *extractor) args(words []*word, i int, valued map[string]bool, opt func(o string, next *word) bool, fn func(w *word, s string)) {
	for ; i < len(words); i++ {
		s, ok := x.text(words[i])
		if !ok {
			continue
		}
		if strings.HasPrefix(s, "-") {
			var next *word
			if i+1 < len(words) {
				next = words[i+1]
			}
			if opt != nil && opt(s, next) || valued[s] {
				i++
			}
			continue
		}
		fn(words[i], s)
	}
}

func (x *extractor) pkg(spec, eco, pkgName, version string, line int) {
	x.add(spec, pkgName, eco+"@"+version, line)
}

var pipValued = set("-c --constraint -i --index-url --extra-index-url -t --target --prefix -f --find-links --root --platform --python-version --implementation --abi --src --upgrade-strategy --progress-bar --trusted-host --cache-dir --log --timeout --retries --proxy --no-binary --only-binary -C --config-settings --python --with --index")

// pip reads pip install (and uv's and pipx's): requirement specifiers become pypi
// packages, -r files edges to those files.
func (x *extractor) pip(words []*word, i, line int) {
	prefix := raw(words[:i])
	x.args(words, i, pipValued, func(o string, next *word) bool {
		switch {
		case o == "-r" || o == "--requirement":
			if next != nil {
				x.path(prefix+" "+o+" "+next.raw, next, kindFile, line)
			}
			return true
		case o == "-e" || o == "--editable":
			return true
		}
		return false
	}, func(w *word, s string) {
		if strings.ContainsAny(s, "/:") || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "$") {
			return // a path, an archive, a URL
		}
		s, _, _ = strings.Cut(s, ";")
		m := pipName.FindStringSubmatch(s)
		if m == nil {
			return
		}
		version := strings.ReplaceAll(strings.TrimSpace(m[3]), " ", "")
		x.pkg(prefix+" "+w.raw, ecoPyPI, m[1], version, line)
	})
}

var npmValued = set("--prefix --registry --tag --cache --userconfig -w --workspace --omit --include --save-prefix --filter -C --dir --cwd")

// npm reads npm install/add (and pnpm's, yarn's and bun's): name[@version].
func (x *extractor) npm(words []*word, i, line int) {
	prefix := raw(words[:i])
	x.args(words, i, npmValued, nil, func(w *word, s string) {
		pkgName, version := s, ""
		if at := strings.LastIndexByte(s, '@'); at > 0 {
			pkgName, version = s[:at], s[at+1:]
		}
		if !npmName.MatchString(strings.ToLower(pkgName)) {
			return // a path, a tarball, a git URL, an alias (npm:...)
		}
		x.pkg(prefix+" "+w.raw, ecoNPM, pkgName, version, line)
	})
}

var goValued = set("-tags -ldflags -gcflags -asmflags -mod -modfile -o -p -pkgdir -toolexec -overlay -pgo -exec -C")

// golang reads go install/run/get of a package at a version (pkg@version); without a
// version the command builds the current module's own packages.
func (x *extractor) golang(words []*word, i, line int) {
	prefix := raw(words[:i])
	x.args(words, i, goValued, nil, func(w *word, s string) {
		pkg, version, ok := strings.Cut(s, "@")
		if !ok || strings.HasPrefix(pkg, ".") || strings.HasPrefix(pkg, "/") || !strings.Contains(pkg, ".") {
			return
		}
		x.pkg(prefix+" "+w.raw, ecoGo, goModule(pkg), version, line)
	})
}

// goModule guesses the module a package path belongs to: a /cmd/ directory is a
// module's commands, and on the big forges a module is host/owner/repo (plus a
// major version suffix). Elsewhere the path is kept.
func goModule(pkg string) string {
	if nested[pkg] {
		return pkg
	}
	if i := strings.Index(pkg, "/cmd/"); i > 0 && !nested[pkg[:i+4]] {
		pkg = pkg[:i]
	}
	segs := strings.Split(pkg, "/")
	switch segs[0] {
	case "github.com", "gitlab.com", "bitbucket.org", "codeberg.org":
		n := 3
		if len(segs) > 3 && majorSuffix.MatchString(segs[3]) {
			n = 4
		}
		if len(segs) > n {
			segs = segs[:n]
		}
	case "golang.org":
		if len(segs) > 3 && !nested[strings.Join(segs[:4], "/")] {
			segs = segs[:3]
		}
	}
	return strings.Join(segs, "/")
}

var majorSuffix = regexp.MustCompile(`^v[0-9]+$`)

// nested are well-known tools that are modules of their own inside another module's
// repository.
var nested = map[string]bool{
	"golang.org/x/tools/gopls":                      true,
	"google.golang.org/grpc/cmd/protoc-gen-go-grpc": true,
}

var cargoValued = set("--version --vers --git --branch --tag --rev --path --root --index --registry -F --features --target --target-dir --profile -j --jobs --bin --example --config -Z")

// cargo reads cargo install: crates from crates.io, name[@version] or --version.
// Crates from --git or --path are not crates.io's.
func (x *extractor) cargo(words []*word, i, line int) {
	prefix := raw(words[:i])
	version := ""
	from := false
	x.args(words, i, nil, func(o string, next *word) bool {
		if next != nil && (o == "--version" || o == "--vers") {
			version, _ = x.text(next)
		}
		from = from || o == "--git" || o == "--path"
		return cargoValued[o]
	}, func(*word, string) {})
	if from {
		return
	}
	x.args(words, i, cargoValued, nil, func(w *word, s string) {
		crate, v, _ := strings.Cut(s, "@")
		if v == "" {
			v = version
		}
		if crateName.MatchString(crate) {
			x.pkg(prefix+" "+w.raw, ecoCrates, crate, v, line)
		}
	})
}

var gemValued = set("-v --version -i --install-dir -n --bindir -s --source --platform -P --trust-policy")

// gem reads gem install: name[:version] or -v.
func (x *extractor) gem(words []*word, i, line int) {
	prefix := raw(words[:i])
	version := ""
	skip := false
	x.args(words, i, nil, func(o string, next *word) bool {
		if next != nil && (o == "-v" || o == "--version") {
			version, _ = x.text(next)
		}
		skip = skip || o == "-g" || o == "--file"
		return gemValued[o]
	}, func(*word, string) {})
	if skip {
		return
	}
	x.args(words, i, gemValued, nil, func(w *word, s string) {
		g, v, _ := strings.Cut(s, ":")
		if v == "" {
			v = version
		}
		if strings.HasSuffix(g, ".gem") || !crateName.MatchString(strings.ReplaceAll(g, ".", "_")) {
			return
		}
		x.pkg(prefix+" "+w.raw, ecoGems, g, v, line)
	})
}

func set(s string) map[string]bool {
	m := map[string]bool{}
	for _, f := range strings.Fields(s) {
		m[f] = true
	}
	return m
}
