// Package proto analyzes Protocol Buffers definitions (.proto) and Buf's configuration
// files (buf.yaml, buf.work.yaml, buf.lock, buf.gen.yaml).
//
// An import names a file relative to an import root. It resolves to a project file
// under the roots Buf's configuration declares (a v1 module's directory, a
// buf.work.yaml's directories, a v2 buf.yaml's module paths), else under the -I
// roots the repository's build scripts give protoc (protoc.go) and those protoc is
// usually pointed at (the repository root, proto/, protos/, api/,
// src/main/proto/, the importer's directory and its ancestors), else to a unique
// project file ending in the path. The well-known types (google/protobuf/*.proto) are
// the hidden protobuf-std island; other protos go to the buf island of Buf Schema
// Registry modules: the module a buf.yaml declares (pinned by buf.lock), else one a
// table of common third-party protos names, else, unresolved, the path's first
// directory (resolve.go).
//
// .proto files are read by a scanner of its own (lex.go, source.go), not the
// vendored tree-sitter grammar, which was slower and failed on editions and proto2
// groups (REQ-PROTO-009).
package proto

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoBuf = "buf"          // Buf Schema Registry modules and plugins
	ecoStd = "protobuf-std" // the protos protoc and buf ship
)

// The kinds of file, as Class names them.
const (
	classProto = "proto"
	classYAML  = "buf.yaml"
	classWork  = "buf.work.yaml"
	classLock  = "buf.lock"
	classGen   = "buf.gen.yaml"
)

// Implements: REQ-PROTO-001
type Plugin struct{}

func (Plugin) Name() string { return "proto" }
func (Plugin) Version() int { return 1 }

// fileClass says which kind of file p is, "" for none of them.
func fileClass(p string) string {
	base := path.Base(p)
	switch base {
	case classYAML, classWork, classLock, classGen:
		return base
	}
	if strings.HasPrefix(base, "buf.gen.") && strings.HasSuffix(base, ".yaml") {
		return classGen // a named template: buf generate --template buf.gen.go.yaml
	}
	if strings.EqualFold(path.Ext(base), ".proto") {
		return classProto
	}
	return ""
}

// Claims takes .proto files and Buf's buf.yaml, buf.work.yaml, buf.lock and
// generation templates (buf.gen.yaml, buf.gen.*.yaml).
//
// Implements: REQ-PROTO-001
func (Plugin) Claims(f *scan.File) bool { return !f.Binary && fileClass(f.Path) != "" }

// Class tells Buf's YAML files apart, which share an extension.
//
// Implements: REQ-PROTO-001
func (Plugin) Class(f *scan.File) string { return fileClass(f.Path) }

// Implements: REQ-PROTO-004, REQ-PROTO-005
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoBuf, Name: "Buf Schema Registry"},
		{ID: ecoStd, Name: "Protobuf well-known types", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-PROTO-001, REQ-PROTO-002, REQ-PROTO-003
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	class := fileClass(f.Path)
	if class == classProto {
		return readProto(src), nil
	}
	return readBuf(class, src).extraction(), nil
}
