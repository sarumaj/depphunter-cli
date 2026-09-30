package lang

// The ecosystem ids that more than one plugin records packages under. A
// package's graph id includes its ecosystem, so a plugin that reads another
// ecosystem's dependencies (Bazel's maven_install, a shell script's pip
// install, a ClojureScript require of an npm package) must spell it exactly as
// that ecosystem's own plugin does for the two to meet on one node.
const (
	EcosystemCrates    = "crates"
	EcosystemErlangStd = "erlang-std"
	EcosystemGo        = "go"
	EcosystemGoStd     = "go-std"
	EcosystemHex       = "hex"
	EcosystemJDK       = "jdk"
	EcosystemMaven     = "maven"
	EcosystemNode      = "node"
	EcosystemNPM       = "npm"
	EcosystemPyPI      = "pypi"
	EcosystemRubyGems  = "rubygems"
)
