package scan

import (
	"bytes"
	"path"
	"strings"
)

var byExt = map[string]string{
	".go": "Go",
	".js": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript", ".jsx": "JavaScript",
	".ts": "TypeScript", ".mts": "TypeScript", ".cts": "TypeScript", ".tsx": "TypeScript",
	".py": "Python", ".pyi": "Python",
	".rs": "Rust", ".java": "Java", ".kt": "Kotlin", ".kts": "Kotlin", ".scala": "Scala", ".sc": "Scala",
	".cs": "C#", ".fs": "F#", ".vb": "Visual Basic",
	".c": "C", ".h": "C", ".cc": "C++", ".cpp": "C++", ".cxx": "C++", ".c++": "C++", ".hpp": "C++", ".hh": "C++",
	".hxx": "C++", ".h++": "C++", ".ipp": "C++", ".inl": "C++",
	".m": "Objective-C", ".mm": "Objective-C++", ".podspec": "Ruby", ".swift": "Swift", ".dart": "Dart",
	".rb": "Ruby", ".rake": "Ruby", ".gemspec": "Ruby", ".ru": "Ruby", ".php": "PHP", ".phtml": "PHP", ".pl": "Perl", ".lua": "Lua", ".luau": "Luau", ".tl": "Teal", ".rockspec": "Lua", ".r": "R", ".rmd": "R Markdown", ".qmd": "Quarto", ".rprofile": "R",
	".ex": "Elixir", ".exs": "Elixir", ".erl": "Erlang", ".hrl": "Erlang", ".hs": "Haskell", ".lhs": "Haskell", ".hs-boot": "Haskell", ".hsc": "Haskell", ".cabal": "Cabal", ".clj": "Clojure",
	".zig": "Zig", ".nim": "Nim", ".jl": "Julia",
	".sh": "Shell", ".bash": "Shell", ".zsh": "Shell", ".ksh": "Shell", ".bats": "Shell", ".zsh-theme": "Shell", ".ps1": "PowerShell", ".psm1": "PowerShell", ".psd1": "PowerShell",
	".html": "HTML", ".htm": "HTML", ".css": "CSS", ".scss": "CSS", ".sass": "CSS", ".less": "CSS",
	".vue": "Vue", ".svelte": "Svelte", ".astro": "Astro",
	".json": "JSON", ".yaml": "YAML", ".yml": "YAML", ".toml": "TOML", ".xml": "XML",
	".md": "Markdown", ".mdx": "Markdown", ".rst": "reStructuredText", ".txt": "Text",
	".sql": "SQL", ".proto": "Protobuf", ".graphql": "GraphQL", ".tf": "Terraform",
	".tofu": "OpenTofu", ".tfvars": "Terraform", ".hcl": "HCL", ".cmake": "CMake",
}

var byName = map[string]string{
	"Makefile": "Make", "go.mod": "Go", "go.sum": "Go",
	"CMakeLists.txt": "CMake", "CMakePresets.json": "CMake", "CMakeUserPresets.json": "CMake",
	"Jenkinsfile": "Groovy", "Gemfile": "Ruby", "Rakefile": "Ruby", "Guardfile": "Ruby", "Capfile": "Ruby",
	"rebar.config": "Erlang", "rebar.lock": "Erlang", "mix.lock": "Elixir",
	"luarocks.lock": "Lua", ".luacheckrc": "Lua", ".busted": "Lua",
	"DESCRIPTION": "R", "NAMESPACE": "R", "renv.lock": "R", "packrat.lock": "R",
	"cabal.project": "Cabal", "cabal.project.freeze": "Cabal", "cabal.project.local": "Cabal",
	"stack.yaml": "Haskell", "stack.yaml.lock": "Haskell", "package.yaml": "Haskell",
	".terraform.lock.hcl": "Terraform", "terragrunt.hcl": "Terragrunt",
	"Podfile": "Ruby", "Podfile.lock": "YAML", "Cartfile": "Carthage", "Cartfile.private": "Carthage", "Cartfile.resolved": "Carthage",
	"buf.yaml": "Buf", "buf.work.yaml": "Buf", "buf.lock": "Buf", "buf.gen.yaml": "Buf",
	".envrc": "Shell", ".profile": "Shell", ".bashrc": "Shell", ".bash_profile": "Shell",
	".bash_login": "Shell", ".bash_logout": "Shell", ".bash_aliases": "Shell", ".zshrc": "Shell",
	".zshenv": "Shell", ".zprofile": "Shell", ".zlogin": "Shell", ".zlogout": "Shell", ".kshrc": "Shell",
}

// Language guesses a file's language from its name; "" means unknown.
//
// Implements: REQ-LANG-015
func Language(p string) string {
	base := path.Base(p)
	if l, ok := byName[base]; ok {
		return l
	}
	if Dockerfile(p) {
		return "Docker"
	}
	if strings.HasSuffix(base, ".app.src") {
		return "Erlang" // an OTP application resource file
	}
	if strings.HasSuffix(strings.ToLower(base), ".cmake.in") {
		return "CMake" // a package configuration template
	}
	if strings.HasSuffix(base, ".tf.json") || strings.HasSuffix(base, ".tfvars.json") {
		return "Terraform" // Terraform's JSON syntax
	}
	return byExt[strings.ToLower(path.Ext(base))]
}

// Dockerfile reports whether a file is a container build file by the names the
// tools look for or are commonly pointed at with -f: Dockerfile and Containerfile,
// a variant named after them (Dockerfile.dev), and one named for what it builds
// (api.Dockerfile). Case does not matter: build tools on a case-insensitive file
// system find a "dockerfile" as well. An ignore file kept beside a Dockerfile
// (Dockerfile.dockerignore) is not one.
//
// Implements: REQ-LANG-015, REQ-DOCKER-001
func Dockerfile(p string) bool {
	base := strings.ToLower(path.Base(p))
	if strings.HasSuffix(base, ".dockerignore") {
		return false
	}
	for _, name := range []string{"dockerfile", "containerfile"} {
		if base == name || strings.HasPrefix(base, name+".") || strings.HasSuffix(base, "."+name) {
			return true
		}
	}
	return false
}

// xmlDocument reports whether a file's first bytes open an XML document: an XML
// declaration or a document type, after an optional byte order mark and
// whitespace. Neither is valid in any language the extensions map to, so a
// ".ts" file that starts so is a Qt Linguist translation, not TypeScript.
//
// Implements: REQ-LANG-015
func xmlDocument(head []byte) bool {
	head = bytes.TrimLeft(bytes.TrimPrefix(head, []byte("\xef\xbb\xbf")), " \t\r\n")
	return bytes.HasPrefix(head, []byte("<?xml")) || bytes.HasPrefix(head, []byte("<!DOCTYPE"))
}

// objcMarker reports whether a line of a file's head starts with what only
// Objective-C (among the languages sharing ".h" and ".m") writes: a keyword of its
// own (@interface, @implementation, @protocol, @class, @import, @end) or `#import`.
// With preprocessor reports whether any preprocessor directive or a `//` comment
// counts too: a ".m" file is Objective-C when it has one, since neither MATLAB nor
// Mercury writes one, while a ".h" file with only those is a C header.
//
// Implements: REQ-LANG-015, REQ-OBJC-001
func objcMarker(head []byte, preprocessor bool) bool {
	for len(head) > 0 {
		line := head
		if i := bytes.IndexByte(head, '\n'); i >= 0 {
			line, head = head[:i], head[i+1:]
		} else {
			head = nil
		}
		line = bytes.TrimLeft(line, " \t\xef\xbb\xbf")
		switch {
		case len(line) < 2:
		case line[0] == '@':
			for _, kw := range []string{"interface", "implementation", "protocol", "class", "import", "end"} {
				if rest, ok := bytes.CutPrefix(line[1:], []byte(kw)); ok && (len(rest) == 0 || !identByte(rest[0])) {
					return true
				}
			}
		case line[0] == '#':
			d := bytes.TrimLeft(line[1:], " \t")
			if preprocessor || bytes.HasPrefix(d, []byte("import")) {
				return true
			}
		case preprocessor && line[0] == '/' && line[1] == '/':
			return true
		}
	}
	return false
}

func identByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// notObjC names what a ".m" file without any Objective-C marker is: Mercury when a
// line starts with a declaration (":- module"), else MATLAB.
func notObjC(head []byte) string {
	for _, line := range bytes.Split(head, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte(":-")) {
			return "Mercury"
		}
	}
	return "MATLAB"
}

// shells are the interpreters whose scripts are shell scripts.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "mksh": true, "ash": true}

// ShellInterpreter reports whether a "#!" line's program runs a shell script.
//
// Implements: REQ-SHELL-001
func ShellInterpreter(name string) bool { return shells[name] }

// interpreter reads the program a file's "#!" line names: its base name, or for
// "#!/usr/bin/env [-S] [NAME=value...] prog" the program env runs. Only the first
// line counts, and nothing is read beyond what measure has already peeked at.
//
// Implements: REQ-LANG-015, REQ-SHELL-001
func interpreter(head []byte) string {
	if !bytes.HasPrefix(head, []byte("#!")) {
		return ""
	}
	line := head[2:]
	if i := bytes.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	fields := strings.Fields(string(line))
	if len(fields) == 0 {
		return ""
	}
	prog := path.Base(fields[0])
	if prog != "env" {
		return prog
	}
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "-") || strings.Contains(f, "=") {
			continue // -S, -i, NAME=value
		}
		return path.Base(f)
	}
	return ""
}
