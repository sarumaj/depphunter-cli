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
// interpolated as Compose does from their ${VAR:-default}; a variable only the
// environment or an .env file sets is left unexpanded (REQ-DOCKER-003). Services
// become the file's symbols.
//
// Implements: REQ-DOCKER-007, REQ-DOCKER-008
func extractCompose(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return ex // a Compose file that does not parse has no dependencies
	}
	var symbols lang.SymbolSet
	for _, svc := range pairs(field(&doc, "services")) {
		symbols.Add(svc.key.Value, "service", svc.key.Line)
		image, build := field(svc.value, "image"), field(svc.value, "build")
		if build == nil {
			if ref := text(image); ref != "" {
				v, _ := expand(ref, nil, true)
				ex.Imports = append(ex.Imports, lang.RawImport{
					Spec: "image: " + ref, Module: v, Name: kindImage, Line: image.Line,
				})
			}
			continue
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
			continue
		}
		p := path.Join(context, dockerfile)
		ex.Imports = append(ex.Imports, lang.RawImport{
			Spec: "build: " + p, Module: p, Name: kindBuild, Line: build.Line,
		})
	}
	ex.Symbols = symbols.List()
	return ex
}

// addContext records an additional build context when it is an image.
func addContext(ex *lang.Extraction, value string, line int) {
	ref, ok := strings.CutPrefix(value, "docker-image://")
	if !ok || ref == "" {
		return // a directory, a URL or another service's build
	}
	v, _ := expand(ref, nil, true)
	ex.Imports = append(ex.Imports, lang.RawImport{
		Spec: "additional context: " + ref, Module: v, Name: kindImage, Line: line,
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
