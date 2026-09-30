package objc

import (
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cocoapods"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	include cpp.Includes
	pods    *cocoapods.Index
	named   map[string][]string // directory name -> directories so named, shallowest first
	byBase  map[string][]string // file name -> the project files so named
}

// Implements: REQ-OBJC-004, REQ-OBJC-006
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{include: cpp.NewIncludes(root, all), pods: cocoapods.Read(root, all), named: map[string][]string{},
		byBase: map[string][]string{}}
	seen := map[string]bool{}
	for _, f := range all {
		r.byBase[path.Base(f.Path)] = append(r.byBase[path.Base(f.Path)], f.Path)
		for d := path.Dir(f.Path); d != "." && !seen[d]; d = path.Dir(d) {
			seen[d] = true
			if !vendored(d) {
				r.named[path.Base(d)] = append(r.named[path.Base(d)], d)
			}
		}
	}
	for _, directories := range r.named {
		sort.Slice(directories, func(i, j int) bool { return lang.ShallowestFirst(directories[i], directories[j]) })
	}
	return r
}

func vendored(d string) bool {
	for _, segment := range strings.Split(d, "/") {
		if segment == "Pods" || segment == "Carthage" {
			return true
		}
	}
	return false
}

// Resolve follows an include, a module import or a manifest's dependency.
//
// Implements: REQ-OBJC-004, REQ-OBJC-005, REQ-OBJC-006, REQ-OBJC-007, REQ-OBJC-010, REQ-OBJC-011
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindPod:
		return r.pods.Pod(file, rawImport.Module)
	case kindCarthage:
		return r.pods.Cart(file, strings.TrimPrefix(rawImport.Module, "carthage:"))
	case kindModule:
		return r.module(file, rawImport.Module)
	}
	name := strings.ReplaceAll(rawImport.Module, `\`, "/")
	quoted := rawImport.Name == cpp.Quoted
	if directory, _, ok := strings.Cut(name, "/"); ok && !quoted {
		if t, ok := apple(directory); ok {
			return t
		}
	}
	t := r.include.Resolve(file, rawImport, func(name string, quoted bool) (lang.Target, bool) {
		return r.external(file, name, quoted)
	})
	if t.Local != "" {
		if v, ok := r.pods.Vendored(file, t.Local); ok {
			return v // a header of a pod checked into Pods/, or of a Carthage checkout
		}
	}
	return t
}

// apple is the apple-sdk target of an Apple framework's directory; <objc/runtime.h>
// is the Objective-C runtime's.
func apple(directory string) (lang.Target, bool) {
	if directory == "objc" {
		return lang.Target{Ecosystem: swift.AppleEcosystem, Package: "ObjectiveC"}, true
	}
	if name, ok := swift.AppleSDK(directory); ok {
		return lang.Target{Ecosystem: swift.AppleEcosystem, Package: name}, true
	}
	return lang.Target{}, false
}

// appleHeaders are headers of Apple's SDKs included without their framework's
// directory, which the cpp plugin's system headers do not list.
var appleHeaders = map[string]string{
	"AssertMacros.h": "CoreServices", "compression.h": "Compression", "Block.h": "Darwin",
	"notify.h": "Darwin", "asl.h": "Darwin", "copyfile.h": "Darwin", "libproc.h": "Darwin",
	"sandbox.h": "Darwin", "launch.h": "Darwin", "xpc.h": "XPC",
}

// appleDirectories are directories of Darwin headers the cpp plugin's system directories
// do not list.
var appleDirectories = map[string]string{"pthread": "Darwin", "malloc": "Darwin", "xpc": "XPC", "CommonCrypto": "CommonCrypto"}

// external names a header that is neither the project's nor a standard or system
// header: a framework header of a pod or Carthage dependency (<Pod/Header.h>), a
// bare header named like a pod ("AFNetworking.h", which CocoaPods puts on the header
// path), a framework built by the project (a directory of that name), or - under a
// Podfile or podspec - a pod no manifest declares.
//
// Implements: REQ-OBJC-006, REQ-OBJC-014
func (r *resolver) external(file, name string, quoted bool) (lang.Target, bool) {
	directory, _, framework := strings.Cut(name, "/")
	if framework {
		if t, ok := apple(directory); ok {
			return t, true // "UIKit/UIKit.h", quoted
		}
		if framework, ok := appleDirectories[directory]; ok {
			return lang.Target{Ecosystem: swift.AppleEcosystem, Package: framework}, true
		}
	} else if framework, ok := appleHeaders[name]; ok {
		return lang.Target{Ecosystem: swift.AppleEcosystem, Package: framework}, true
	}
	if !framework {
		base := strings.TrimSuffix(name, path.Ext(name))
		if t, ok := r.pods.Module(file, base); ok {
			return t, true
		}
		return lang.Target{}, false
	}
	if quoted && strings.HasPrefix(name, ".") {
		return lang.Target{}, false
	}
	if t, ok := r.pods.Module(file, directory); ok {
		return r.inPod(t, name), true
	}
	if directories := r.named[directory]; len(directories) > 0 {
		return lang.Target{Local: nearest(file, directories)}, true
	}
	if r.pods.Governed(file) {
		return lang.Target{Ecosystem: cocoapods.Ecosystem, Package: directory, Unresolved: true}, true
	}
	return lang.Target{}, false
}

// inPod narrows a pod the project builds itself (a path pod, its own podspec) to
// the header an include names, when exactly one file of that name is in the pod's
// directory: <LocalKit/LKThing.h> is LocalKit/Sources/LKThing.h.
func (r *resolver) inPod(t lang.Target, include string) lang.Target {
	if t.Local == "" {
		return t
	}
	directory := t.Local
	if strings.Contains(path.Base(directory), ".podspec") {
		directory = path.Dir(directory)
	}
	var found []string
	for _, p := range r.byBase[path.Base(include)] {
		if directory == "." || strings.HasPrefix(p, directory+"/") {
			found = append(found, p)
		}
	}
	if len(found) == 1 {
		return lang.Target{Local: found[0]}
	}
	return t
}

// module resolves `@import Module.Sub;` by its first component: an Apple framework,
// a pod or Carthage dependency, a framework the project builds, else an unresolved
// pod under a CocoaPods manifest or an unknown C library.
//
// Implements: REQ-OBJC-005, REQ-OBJC-006
func (r *resolver) module(file, m string) lang.Target {
	m, _, _ = strings.Cut(m, ".")
	if t, ok := apple(m); ok {
		return t
	}
	if t, ok := r.pods.Module(file, m); ok {
		return t
	}
	if directories := r.named[m]; len(directories) > 0 {
		return lang.Target{Local: nearest(file, directories)}
	}
	if r.pods.Governed(file) {
		return lang.Target{Ecosystem: cocoapods.Ecosystem, Package: m, Unresolved: true}
	}
	return r.include.Library(file, m+"/"+m+".h")
}

// Dependencies implements lang.Transitive: Podfile.lock's graph, and what the
// checkouts of Carthage dependencies declare.
//
// Implements: REQ-OBJC-008, REQ-OBJC-011
func (r *resolver) Dependencies(t lang.Target) []lang.Target { return r.pods.Dependencies(t) }

// Installed says a Carthage dependency's dependencies come from its checkout.
func (r *resolver) Installed(t lang.Target) bool { return r.pods.Installed(t) }

// nearest picks the directory sharing the longest path prefix with the file.
func nearest(file string, directories []string) string {
	best, bestLength := directories[0], -1
	for _, d := range directories {
		n := 0
		fileParts, directoryParts := strings.Split(file, "/"), strings.Split(d, "/")
		for n < len(fileParts) && n < len(directoryParts) && fileParts[n] == directoryParts[n] {
			n++
		}
		if n > bestLength {
			best, bestLength = d, n
		}
	}
	return best
}
