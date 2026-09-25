package powershell

import (
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// builtin lists modules that ship with PowerShell or Windows (lower case).
//
// Implements: REQ-PS-008
var builtin = map[string]bool{}

func init() {
	// cSpell: disable
	for _, m := range strings.Fields(`microsoft.powershell.archive microsoft.powershell.core
		microsoft.powershell.diagnostics microsoft.powershell.host microsoft.powershell.management
		microsoft.powershell.security microsoft.powershell.utility microsoft.powershell.localaccounts
		microsoft.powershell.psresourceget microsoft.powershell.operation.validation
		microsoft.wsman.management cimcmdlets packagemanagement powershellget psreadline psdiagnostics
		threadjob psscheduledjob psworkflow dism netadapter netsecurity nettcpip netconnection dnsclient
		scheduledtasks smbshare storage bitlocker defender international pki applocker appx
		bitstransfer hyper-v servermanager windowsupdate printmanagement`) {
		builtin[m] = true
	}
	// cSpell: enable
}

type resolver struct {
	files    map[string]bool
	modules  map[string]string     // lower-case module name -> project manifest or module file
	declared map[string]moduleSpec // lower-case name -> RequiredModules entry of a project manifest
}

var requiredModules = regexp.MustCompile(`(?is)RequiredModules\s*=\s*`)

// Implements: REQ-PS-007, REQ-PS-010
func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, modules: map[string]string{}, declared: map[string]moduleSpec{}}
	for _, f := range all {
		r.files[f.Path] = true
		ext := strings.ToLower(path.Ext(f.Path))
		if ext != ".psd1" && ext != ".psm1" {
			continue
		}
		name := strings.ToLower(strings.TrimSuffix(path.Base(f.Path), path.Ext(f.Path)))
		if _, seen := r.modules[name]; !seen || ext == ".psd1" { // the manifest represents a module
			r.modules[name] = f.Path
		}
		if ext == ".psd1" {
			if data, err := os.ReadFile(f.Abs); err == nil {
				for _, spec := range moduleSpecs(requiredValue(string(data))) {
					r.declared[strings.ToLower(spec.name)] = spec
				}
			}
		}
	}
	return r
}

// requiredValue returns the text of a manifest's RequiredModules value.
func requiredValue(manifest string) string {
	manifest = stripComments(manifest)
	loc := requiredModules.FindStringIndex(manifest)
	if loc == nil {
		return ""
	}
	return valueAt(manifest[loc[1]:])
}

// Implements: REQ-PS-001, REQ-PS-007, REQ-PS-008, REQ-PS-010
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	ref := imp.Module
	if imp.Name == refPath || looksLikePath(ref) {
		if t, ok := r.localPath(ref, file); ok {
			return t
		}
		// A bare name in NestedModules may still be a module; paths that do not exist
		// (or depend on variables) point outside what we can see.
		if strings.ContainsAny(ref, `/\$`) || path.Ext(ref) != "" {
			return lang.Target{}
		}
	}
	lower := strings.ToLower(ref)
	if p, ok := r.modules[lower]; ok {
		return lang.Target{Local: p}
	}
	if builtin[lower] {
		return lang.Target{Ecosystem: ecoBuiltin, Package: ref}
	}
	if spec, ok := r.declared[lower]; ok {
		return lang.Target{
			Ecosystem: ecoGallery, Package: spec.name, Version: spec.version,
			Pinned: spec.exact && lang.Pinned(spec.version),
		}
	}
	if how, ok := strings.CutPrefix(imp.Name, refRequires); ok {
		// "=<version>" is a minimum (ModuleVersion), "==<version>" one version.
		v := strings.TrimPrefix(how, "=")
		exact := strings.HasPrefix(v, "=")
		v = strings.TrimPrefix(v, "=")
		return lang.Target{Ecosystem: ecoGallery, Package: ref, Version: v, Pinned: exact && lang.Pinned(v)}
	}
	return lang.Target{Ecosystem: ecoGallery, Package: ref, Unresolved: true}
}

func looksLikePath(s string) bool {
	switch strings.ToLower(path.Ext(s)) {
	case ".ps1", ".psm1", ".psd1", ".dll":
		return true
	}
	return strings.ContainsAny(s, `/\$`) || strings.HasPrefix(s, ".")
}

// localPath maps a script or module path, relative to the referring script or using
// $PSScriptRoot, to a project file.
//
// Implements: REQ-PS-003, REQ-PS-004
func (r *resolver) localPath(ref, file string) (lang.Target, bool) {
	dir := path.Dir(file)
	p := strings.ReplaceAll(ref, `\`, "/")
	anchored := false
	for _, v := range []string{"$($PSScriptRoot)", "${PSScriptRoot}", "$PSScriptRoot"} {
		if strings.Contains(p, v) {
			p, anchored = strings.ReplaceAll(p, v, dir), true
		}
	}
	if strings.Contains(p, "$") || strings.HasPrefix(p, "/") || (len(p) > 1 && p[1] == ':') {
		return lang.Target{}, false // other variables, absolute paths: outside the project
	}
	if !anchored {
		p = path.Join(dir, p)
	}
	p = path.Clean(p)
	base := path.Base(p)
	for _, candidate := range []string{p, p + ".psd1", p + ".psm1", p + ".ps1", path.Join(p, base+".psd1"), path.Join(p, base+".psm1")} {
		if r.files[candidate] {
			return lang.Target{Local: candidate}, true
		}
	}
	return lang.Target{}, false
}
