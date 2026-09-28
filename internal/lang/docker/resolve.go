package docker

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/oci"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	root  string
	files map[string]bool

	mu           sync.Mutex
	environments map[string]map[string]string // directory -> what its .env file sets
}

// A Compose file's include: and cross-file extends: resolve to several imports.
var _ lang.Expander = (*resolver)(nil)

func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, environments: map[string]map[string]string{}}
	for _, f := range all {
		r.files[f.Path] = true
	}
	return r
}

// Implements: REQ-DOCKER-003, REQ-DOCKER-004, REQ-DOCKER-007, REQ-DOCKER-009
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	directory := path.Dir(file)
	return r.target(rawImport, directory, directory)
}

// target resolves rawImport with the paths it holds read from directory, and a Compose value
// interpolated with the .env file of environmentDirectory: the directory of the Compose file that
// was asked for, which Compose takes as the project's.
func (r *resolver) target(rawImport lang.RawImport, directory, environmentDirectory string) lang.Target {
	switch rawImport.Name {
	case kindBuild:
		// Compose reads a build context from the Compose file's own directory.
		return r.local(path.Join(directory, rawImport.Module))
	case kindImage:
		return imageTarget(rawImport.Module, rawImport.Module)
	case kindCompose:
		v, _ := expand(rawImport.Module, r.environment(environmentDirectory), true)
		written, _ := expand(rawImport.Module, nil, true)
		return imageTarget(v, written)
	}
	return lang.Target{}
}

// imageTarget resolves a container reference. A variable left in the name means the image
// itself is chosen at build time: it is unresolved, under the reference as written
// (with its defaults, never a value of an .env file). One left only in the tag or
// digest names a known image at a version nobody can read off the file, which is
// kept as written and pins nothing.
func imageTarget(reference, written string) lang.Target {
	t := oci.Image(reference)
	if strings.Contains(t.Package, "$") {
		return lang.Target{Ecosystem: oci.Ecosystem, Package: written, Unresolved: true}
	}
	if strings.Contains(t.Version, "$") {
		t.Pinned, t.Requested = false, ""
	}
	return t
}

// local is the project file p, when the repository has it.
func (r *resolver) local(p string) lang.Target {
	if strings.HasPrefix(p, "../") || p == ".." || !r.files[p] {
		return lang.Target{}
	}
	return lang.Target{Local: p}
}

// Expand resolves what another Compose file gives this one: the files include:
// lists, and the image and build a service extends from another file.
//
// Implements: REQ-DOCKER-009
func (r *resolver) Expand(file string, rawImport lang.RawImport) ([]lang.Import, bool) {
	if rawImport.Name != kindInclude && rawImport.Name != kindExtends {
		return nil, false
	}
	return r.expand(file, rawImport, 0), true
}

func (r *resolver) expand(file string, rawImport lang.RawImport, depth int) []lang.Import {
	directory := path.Dir(file)
	if rawImport.Name == kindInclude {
		// An included file is read with its own directory as the project's, and so
		// with its own .env file.
		var t lang.Target
		if p, ok := r.path(directory, rawImport.Module, directory); ok {
			t = r.local(p)
		}
		out := []lang.Import{{Spec: rawImport.Spec, Line: rawImport.Line, Target: t}}
		if t.Local == "" || composeFile(t.Local) || depth >= maxExtends {
			return out // a Compose file by name is analyzed on its own
		}
		if source := r.read(t.Local); source != nil {
			for _, imported := range r.apply(t.Local, extractCompose(source), depth+1) {
				imported.Spec, imported.Line = imported.Spec+" ("+t.Local+")", rawImport.Line
				out = append(out, imported)
			}
		}
		return out
	}
	services := r.services(file)
	image, build, buildFile, via := inherit(file, services, field(services, rawImport.Module),
		func(from, written string) (string, *yaml.Node) {
			p, ok := r.path(path.Dir(from), written, directory)
			if !ok || !r.files[p] {
				return "", nil
			}
			return p, r.services(p)
		})
	var out []lang.Import
	for _, h := range via {
		var t lang.Target
		if h.file != "" {
			t = lang.Target{Local: h.file}
		}
		out = append(out, lang.Import{Spec: "extends: " + h.written, Line: rawImport.Line, Target: t})
	}
	extraction := &lang.Extraction{}
	serviceImports(extraction, image, build)
	for _, rawImport := range extraction.Imports {
		out = append(out, lang.Import{Spec: rawImport.Spec, Line: rawImport.Line, Target: r.target(rawImport, path.Dir(buildFile), directory)})
	}
	return out
}

// apply resolves the imports of a file read for another one.
func (r *resolver) apply(file string, extraction *lang.Extraction, depth int) []lang.Import {
	var out []lang.Import
	for _, rawImport := range extraction.Imports {
		if rawImport.Name == kindInclude || rawImport.Name == kindExtends {
			out = append(out, r.expand(file, rawImport, depth)...)
			continue
		}
		out = append(out, lang.Import{Spec: rawImport.Spec, Line: rawImport.Line, Target: r.Resolve(file, rawImport)})
	}
	return out
}

// path is a file named in a Compose file of directory, interpolated with the .env file of
// environmentDirectory, from the repository root; false when it stays unknown or is absolute or
// remote. The caller checks the repository has it.
func (r *resolver) path(directory, written, environmentDirectory string) (string, bool) {
	v, ok := expand(written, r.environment(environmentDirectory), true)
	if !ok || v == "" || remote(v) || path.IsAbs(v) {
		return "", false
	}
	return path.Join(directory, v), true
}

// read returns a file of the repository, nil when it cannot be read or is too large
// to parse.
func (r *resolver) read(p string) []byte {
	absolute := filepath.Join(r.root, filepath.FromSlash(p))
	if info, err := os.Stat(absolute); err != nil || !info.Mode().IsRegular() || info.Size() > lang.MaxParseSize {
		return nil
	}
	source, err := os.ReadFile(absolute)
	if err != nil {
		return nil
	}
	return source
}

// services is the services: mapping of a Compose file of the repository.
func (r *resolver) services(p string) *yaml.Node {
	var doc yaml.Node
	if source := r.read(p); source == nil || yaml.Unmarshal(source, &doc) != nil {
		return nil
	}
	return field(&doc, "services")
}

// environment is what the .env file of directory sets, read once. The file is often left out of
// version control and so of the scan; it is read from disk. Its values serve only to
// interpolate the references resolved here: they are never reported.
//
// Implements: REQ-DOCKER-003
func (r *resolver) environment(directory string) map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	variables, ok := r.environments[directory]
	if !ok {
		variables = readEnvironment(r.read(path.Join(directory, ".env")))
		r.environments[directory] = variables
	}
	return variables
}

// readEnvironment reads an .env file as Compose does: NAME=value lines, `export ` allowed
// before the name, # comments; a value in single quotes is literal, one in double
// quotes or unquoted is interpolated with the names set above it, and an unquoted
// one ends at " #". A line without "=" (a name the environment passes) sets nothing,
// and neither does a name that holds a credential (secretName).
func readEnvironment(source []byte) map[string]string {
	variables := map[string]string{}
	for line := range strings.Lines(string(source)) {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" || strings.ContainsAny(name, " \t$") {
			continue
		}
		if secretName(name) {
			continue // not needed to name an image, and never to be shown
		}
		value = strings.TrimSpace(value)
		switch {
		case len(value) >= 2 && value[0] == '\'' && strings.IndexByte(value[1:], '\'') >= 0:
			variables[name] = value[1 : 1+strings.IndexByte(value[1:], '\'')]
			continue
		case len(value) >= 2 && value[0] == '"' && strings.IndexByte(value[1:], '"') >= 0:
			value = value[1 : 1+strings.IndexByte(value[1:], '"')]
		default:
			if i := strings.Index(value, " #"); i >= 0 {
				value = strings.TrimSpace(value[:i])
			}
		}
		variables[name], _ = expand(value, variables, true)
	}
	return variables
}

// secretName reports whether an .env name holds a credential (DB_PASSWORD,
// GITHUB_TOKEN, AWS_SECRET_ACCESS_KEY, API_KEY): such a value is not read at all,
// so no reference can carry it into a report.
func secretName(name string) bool {
	upper := strings.ToUpper(name)
	for _, word := range []string{"PASSWORD", "PASSWD", "SECRET", "TOKEN", "CREDENTIAL", "PRIVATE"} {
		if strings.Contains(upper, word) {
			return true
		}
	}
	return upper == "KEY" || strings.HasSuffix(upper, "_KEY") || strings.HasSuffix(upper, "APIKEY")
}
