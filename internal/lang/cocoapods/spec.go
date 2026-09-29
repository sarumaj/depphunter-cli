package cocoapods

import (
	"regexp"
	"strings"
)

// Spec is what a podspec says about its dependencies, shaped like the
// podspec.json the CocoaPods CDN serves: the root spec's dependencies, each
// subspec's, and the subspecs a dependency on the pod alone takes. Test and app
// specs are left out.
type Spec struct {
	Name string
	// Dependencies maps each pod depended on to its requirements.
	Dependencies    map[string][]string
	Subspecs        []Spec
	DefaultSubspecs []string
}

var (
	// specBlock opens a subspec, test spec or app spec block: `s.subspec 'Core' do |core|`.
	specBlock = regexp.MustCompile(`^(\w+)\.(subspec|test_spec|app_spec)\b\s*\(?\s*(?:['"]([^'"]*)['"])?.*\bdo\s*\|\s*(\w+)\s*\|`)
	// specRoot opens the specification: `Pod::Spec.new do |s|`.
	specRoot = regexp.MustCompile(`\bPod::Spec(?:ification)?\.new\b.*\bdo\s*\|\s*(\w+)\s*\|`)
	// specPlatformDependency is a dependency call, a platform's included:
	// `s.dependency`, `s.ios.dependency`.
	specPlatformDependency = regexp.MustCompile(`^(\w+)(?:\.(?:ios|osx|macos|tvos|watchos|visionos))?\.dependency\b`)
	// specDefaults assigns the default subspecs.
	specDefaults = regexp.MustCompile(`^(\w+)\.default_subspecs?\s*=\s*(.*)$`)
	// quotedWord is a string literal's content; wordList a %w[] list.
	quotedWord = regexp.MustCompile(`['"]([^'"]+)['"]`)
	wordList   = regexp.MustCompile(`%[wW][\[({<]([^\])}>]*)[\])}>]`)
)

// ReadSpec reads a Ruby podspec for the index client, as lightly as the plugin's
// own reader (readPodspec): statements, a `dependency` call's name and
// requirement strings, and which block each call is in by the variable it is made
// on - the root spec's, a subspec's (a nested subspec counts as its outermost
// subspec, which a dependency on it takes whole), or a test or app spec's, which
// is dropped. Nothing is evaluated: a dependency in a loop or behind a
// condition counts as written, and one whose name is computed is not read.
//
// Implements: REQ-SUP-074
func ReadSpec(source []byte) Spec {
	root := Spec{Dependencies: map[string][]string{}}
	rootName := ""
	// owner maps a block variable to the subspec it belongs to (an index into
	// root.Subspecs), -1 for the root spec, or -2 for a test or app spec.
	owner := map[string]int{}
	for _, s := range statements(string(source)) {
		text := s.text
		if m := specRoot.FindStringSubmatch(text); m != nil {
			rootName = m[1]
			owner[rootName] = -1
			continue
		}
		if m := specBlock.FindStringSubmatch(text); m != nil {
			parent, ok := owner[m[1]]
			if !ok {
				continue
			}
			switch {
			case m[2] != "subspec":
				owner[m[4]] = -2
			case parent == -1:
				root.Subspecs = append(root.Subspecs, Spec{Name: m[3], Dependencies: map[string][]string{}})
				owner[m[4]] = len(root.Subspecs) - 1
			default:
				owner[m[4]] = parent
			}
			continue
		}
		if m := specName.FindStringSubmatch(text); m != nil && root.Name == "" && strings.HasPrefix(text, rootName+".") {
			root.Name = m[2]
			continue
		}
		if m := specDefaults.FindStringSubmatch(text); m != nil && owner[m[1]] == -1 && rootName != "" {
			root.DefaultSubspecs = words(m[2])
			continue
		}
		m := specPlatformDependency.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		at, ok := owner[m[1]]
		if !ok || at == -2 {
			continue
		}
		arguments, ok := call(text[strings.Index(text, "dependency"):], "dependency")
		if !ok || len(arguments) == 0 {
			continue
		}
		name, ok := literal(arguments[0])
		if !ok || name == "" {
			continue
		}
		d := &declaration{name: name}
		podArguments(d, arguments[1:])
		var requirements []string
		if d.requirements != "" {
			requirements = strings.Split(d.requirements, ", ")
		}
		target := &root
		if at >= 0 {
			target = &root.Subspecs[at]
		}
		target.Dependencies[name] = append(target.Dependencies[name], requirements...)
	}
	return root
}

// words reads the names a default_subspecs value lists: string literals, a
// %w[] list, or an array of either.
func words(value string) []string {
	var out []string
	for _, m := range wordList.FindAllStringSubmatch(value, -1) {
		out = append(out, strings.Fields(m[1])...)
	}
	if len(out) > 0 {
		return out
	}
	for _, m := range quotedWord.FindAllStringSubmatch(value, -1) {
		out = append(out, m[1])
	}
	return out
}
