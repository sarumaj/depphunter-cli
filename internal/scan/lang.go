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
	".m": "Objective-C", ".swift": "Swift", ".dart": "Dart",
	".rb": "Ruby", ".rake": "Ruby", ".gemspec": "Ruby", ".ru": "Ruby", ".php": "PHP", ".phtml": "PHP", ".pl": "Perl", ".lua": "Lua", ".r": "R", ".rmd": "R Markdown", ".qmd": "Quarto", ".rprofile": "R",
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
	"DESCRIPTION": "R", "NAMESPACE": "R", "renv.lock": "R", "packrat.lock": "R",
	"cabal.project": "Cabal", "cabal.project.freeze": "Cabal", "cabal.project.local": "Cabal",
	"stack.yaml": "Haskell", "stack.yaml.lock": "Haskell", "package.yaml": "Haskell",
	".terraform.lock.hcl": "Terraform", "terragrunt.hcl": "Terragrunt",
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
