// Package rego analyzes Rego (.rego), the policy language of Open Policy
// Agent, Conftest, Gatekeeper and Regal.
//
// `import data.a.b` names the policies of package a.b wherever they are in
// the repository - a package may span several files, and each is linked - or,
// for data.a.b.rule, the package that declares the rule (the longest package
// the path starts with). References to data.a.b... in rule bodies, and
// through an imported name, link the same way. import input, import rego.v1
// and import future.keywords are built in. OPA has no package manager: a
// package no file declares is external data or a bundle, and dropped.
//
// Rego is read by a lexer of its own (lex.go, source.go); there is no
// vendored grammar for it (REQ-REGO-006).
package rego

import (
	"path"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Implements: REQ-REGO-001
type Plugin struct{}

func (Plugin) Name() string { return "rego" }
func (Plugin) Version() int { return 1 }

// Implements: REQ-REGO-001
func (Plugin) Claims(f *scan.File) bool { return !f.Binary && path.Ext(f.Path) == ".rego" }

// Ecosystems: none, as a policy imports only the repository's own policies.
//
// Implements: REQ-REGO-005
func (Plugin) Ecosystems() []lang.Ecosystem { return nil }

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-REGO-002, REQ-REGO-003
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	return extractSource(src), nil
}
