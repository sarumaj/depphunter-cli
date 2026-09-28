package docker

import (
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// extractCompose reads a Compose file's services. A service that is built depends on
// the Dockerfile that builds it; its image:, if any, is only the name the result is
// tagged with, so it is not a dependency. A service that is not built depends on the
// image it pulls. Extra build contexts that name an image (docker-image://) are
// images too, and so are the base images of a Dockerfile written inline. Values are
// interpolated as Compose does from their ${VAR:-default}, and by the resolver from
// the .env file beside the Compose file (REQ-DOCKER-003). A service takes the
// image: and build: of the service it extends; one extended from another file, and
// the files the top-level include: lists, are left to the resolver, which reads
// those files. Services become the file's symbols.
//
// Implements: REQ-DOCKER-007, REQ-DOCKER-008, REQ-DOCKER-009
func extractCompose(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return ex // a Compose file that does not parse has no dependencies
	}
	for _, item := range includes(field(&doc, "include")) {
		if p := text(item); p != "" && !remote(p) && !path.IsAbs(p) {
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: "include: " + p, Module: p, Name: kindInclude, Line: item.Line})
		}
	}
	var symbols lang.SymbolSet
	services := field(&doc, "services")
	for _, svc := range pairs(services) {
		symbols.Add(svc.key.Value, "service", svc.key.Line)
		image, build, _, via := inherit("", services, svc.value, nil)
		if len(via) > 0 {
			// The service extends one of another file: the resolver reads it.
			ex.Imports = append(ex.Imports, lang.RawImport{
				Spec: "extends: " + via[0].written, Module: svc.key.Value, Name: kindExtends, Line: via[0].line,
			})
			continue
		}
		n := len(ex.Imports)
		serviceImports(ex, image, build)
		if ext := field(svc.value, "extends"); ext != nil {
			for i := n; i < len(ex.Imports); i++ {
				ex.Imports[i].Line = ext.Line // what the service takes, where it takes it
			}
		}
	}
	ex.Symbols = symbols.List()
	return ex
}

// includes lists the paths of a top-level include:, in its short form (a path) and
// its long form (path:, a path or a list of them).
func includes(n *yaml.Node) []*yaml.Node {
	if n != nil && n.Kind == yaml.ScalarNode {
		return []*yaml.Node{n}
	}
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	var out []*yaml.Node
	for _, item := range n.Content {
		if item.Kind == yaml.ScalarNode {
			out = append(out, item)
			continue
		}
		p := field(item, "path")
		if p != nil && p.Kind == yaml.SequenceNode {
			out = append(out, p.Content...)
		} else if p != nil {
			out = append(out, p)
		}
	}
	return out
}

// hop is an extends: that leads to another file: the file as written, where it is
// written, and the file it names from the repository root.
type hop struct {
	written, file string
	line          int
}

// maxExtends bounds a chain of extends:, which Compose refuses to let loop.
const maxExtends = 8

// inherit follows a service's extends: chain from the file it is written in: image:
// and build: are the first set along it, and buildFile is the file the build: is
// written in (its context is read from there). load reads the services of another
// file, named as written from the file whose extends: names it, and returns the
// file's repository path; nil stops the chain at the first such file, which via
// then lists.
func inherit(file string, services, svc *yaml.Node, load func(from, written string) (string, *yaml.Node)) (image, build *yaml.Node, buildFile string, via []hop) {
	for range maxExtends {
		if image == nil {
			image = field(svc, "image")
		}
		if build == nil {
			if build = field(svc, "build"); build != nil {
				buildFile = file
			}
		}
		ext := field(svc, "extends")
		name, other := text(ext), ""
		if ext != nil && ext.Kind == yaml.MappingNode {
			name, other = text(field(ext, "service")), text(field(ext, "file"))
		}
		if name == "" {
			return image, build, buildFile, via
		}
		if other != "" {
			if remote(other) || path.IsAbs(other) {
				return image, build, buildFile, via
			}
			via = append(via, hop{written: other, line: ext.Line})
			if load == nil {
				return image, build, buildFile, via
			}
			file, services = load(file, other)
			via[len(via)-1].file = file
			if services == nil {
				return image, build, buildFile, via
			}
		}
		if svc = field(services, name); svc == nil {
			return image, build, buildFile, via
		}
	}
	return image, build, buildFile, via
}

// serviceImports records what a service with this image: and build: depends on.
func serviceImports(ex *lang.Extraction, image, build *yaml.Node) {
	if build == nil {
		if ref := text(image); ref != "" {
			ex.Imports = append(ex.Imports, lang.RawImport{
				Spec: "image: " + ref, Module: ref, Name: kindCompose, Line: image.Line,
			})
		}
		return
	}
	context, dockerfile := text(build), "Dockerfile"
	if build.Kind == yaml.MappingNode {
		context = text(field(build, "context"))
		if d := text(field(build, "dockerfile")); d != "" {
			dockerfile = d
		} else if inline := field(build, "dockerfile_inline"); inline != nil {
			// Built from a Dockerfile written into this file: its images are this
			// file's, on the lines they are written on.
			dockerfile = ""
			for _, im := range extractDockerfile([]byte(text(inline))).Imports {
				im.Line += inline.Line
				ex.Imports = append(ex.Imports, im)
			}
		}
		for _, extra := range pairs(field(build, "additional_contexts")) {
			addContext(ex, text(extra.value), extra.value.Line)
		}
		if list := field(build, "additional_contexts"); list != nil && list.Kind == yaml.SequenceNode {
			for _, item := range list.Content {
				_, value, _ := strings.Cut(text(item), "=")
				addContext(ex, value, item.Line)
			}
		}
	}
	if context == "" {
		context = "."
	}
	// A remote context (a Git URL, an archive) is built from somewhere else, and a
	// Dockerfile given by absolute path is outside the repository.
	if dockerfile == "" || remote(context) || path.IsAbs(dockerfile) || path.IsAbs(context) {
		return
	}
	p := path.Join(context, dockerfile)
	ex.Imports = append(ex.Imports, lang.RawImport{
		Spec: "build: " + p, Module: p, Name: kindBuild, Line: build.Line,
	})
}

// addContext records an additional build context when it is an image.
func addContext(ex *lang.Extraction, value string, line int) {
	ref, ok := strings.CutPrefix(value, "docker-image://")
	if !ok || ref == "" {
		return // a directory, a URL or another service's build
	}
	ex.Imports = append(ex.Imports, lang.RawImport{
		Spec: "additional context: " + ref, Module: ref, Name: kindCompose, Line: line,
	})
}

// remote reports whether a build context lies outside the repository's files.
func remote(context string) bool {
	return strings.Contains(context, "://") || strings.HasPrefix(context, "git@") ||
		strings.HasPrefix(context, "github.com/")
}

// ---------------------------------------------------------------- YAML helpers

// mapping unwraps documents and aliases and returns n when it is a mapping.
func mapping(n *yaml.Node) *yaml.Node {
	for n != nil && (n.Kind == yaml.DocumentNode || n.Kind == yaml.AliasNode) {
		if n.Kind == yaml.AliasNode {
			n = n.Alias
		} else if len(n.Content) > 0 {
			n = n.Content[0]
		} else {
			return nil
		}
	}
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

type pair struct{ key, value *yaml.Node }

// pairs lists a mapping's entries in order, aliases followed. A merge key ("<<: *base",
// the usual way a Compose file shares settings between services) contributes the
// entries of the mappings it names, after the mapping's own and only where the
// mapping does not set the key itself.
func pairs(n *yaml.Node) []pair {
	m := mapping(n)
	if m == nil {
		return nil
	}
	var out, merged []pair
	seen := map[string]bool{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		v := m.Content[i+1]
		if v.Kind == yaml.AliasNode {
			v = v.Alias
		}
		if m.Content[i].Value == "<<" && m.Content[i].Tag == "!!merge" {
			sources := []*yaml.Node{v}
			if v.Kind == yaml.SequenceNode {
				sources = v.Content
			}
			for _, src := range sources {
				merged = append(merged, pairs(src)...)
			}
			continue
		}
		seen[m.Content[i].Value] = true
		out = append(out, pair{m.Content[i], v})
	}
	for _, p := range merged {
		if !seen[p.key.Value] {
			seen[p.key.Value] = true
			out = append(out, p)
		}
	}
	return out
}

// field returns the value of key in a mapping, or nil.
func field(n *yaml.Node, key string) *yaml.Node {
	for _, p := range pairs(n) {
		if p.key.Value == key {
			return p.value
		}
	}
	return nil
}

// text returns a scalar's value, "" for anything else.
func text(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return strings.TrimSpace(n.Value)
}
