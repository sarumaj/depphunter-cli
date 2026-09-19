package scan

import (
	"path"
	"strings"
)

var byExt = map[string]string{
	".go": "Go",
	".js": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript", ".jsx": "JavaScript",
	".ts": "TypeScript", ".mts": "TypeScript", ".cts": "TypeScript", ".tsx": "TypeScript",
	".py": "Python", ".pyi": "Python",
	".rs": "Rust", ".java": "Java", ".kt": "Kotlin", ".kts": "Kotlin", ".scala": "Scala",
	".cs": "C#", ".fs": "F#", ".vb": "Visual Basic",
	".c": "C", ".h": "C", ".cc": "C++", ".cpp": "C++", ".cxx": "C++", ".hpp": "C++", ".hh": "C++",
	".m": "Objective-C", ".swift": "Swift", ".dart": "Dart",
	".rb": "Ruby", ".php": "PHP", ".pl": "Perl", ".lua": "Lua", ".r": "R",
	".ex": "Elixir", ".exs": "Elixir", ".erl": "Erlang", ".hs": "Haskell", ".clj": "Clojure",
	".zig": "Zig", ".nim": "Nim", ".jl": "Julia",
	".sh": "Shell", ".bash": "Shell", ".zsh": "Shell", ".ps1": "PowerShell", ".psm1": "PowerShell", ".psd1": "PowerShell",
	".html": "HTML", ".htm": "HTML", ".css": "CSS", ".scss": "CSS", ".sass": "CSS", ".less": "CSS",
	".vue": "Vue", ".svelte": "Svelte",
	".json": "JSON", ".yaml": "YAML", ".yml": "YAML", ".toml": "TOML", ".xml": "XML",
	".md": "Markdown", ".mdx": "Markdown", ".rst": "reStructuredText", ".txt": "Text",
	".sql": "SQL", ".proto": "Protobuf", ".graphql": "GraphQL", ".tf": "Terraform",
}

var byName = map[string]string{
	"Dockerfile": "Docker", "Makefile": "Make", "go.mod": "Go", "go.sum": "Go",
	"CMakeLists.txt": "CMake", "Jenkinsfile": "Groovy",
}

// Language guesses a file's language from its name; "" means unknown.
func Language(p string) string {
	base := path.Base(p)
	if l, ok := byName[base]; ok {
		return l
	}
	return byExt[strings.ToLower(path.Ext(base))]
}
