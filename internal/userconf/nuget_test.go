package userconf

import (
	"path/filepath"
	"reflect"
	"testing"
)

// NuGet's machine-wide files are every *.config of its Config directory: under
// %ProgramFiles(x86)% on Windows (%ProgramFiles% without it), under
// NUGET_COMMON_APPLICATION_DATA when set, else /Library/Application Support on
// macOS and /etc/opt on Linux.
//
// Verifies: REQ-SUP-064
func TestNuGetMachineConfigs(t *testing.T) {
	pf86, programFiles, common := t.TempDir(), t.TempDir(), t.TempDir()
	for _, directory := range []string{pf86, programFiles, common} {
		touch(t, filepath.Join(directory, "NuGet", "Config", "b.Config"))
		touch(t, filepath.Join(directory, "NuGet", "Config", "a.config"))
		touch(t, filepath.Join(directory, "NuGet", "Config", "notes.txt"))
	}
	files := func(directory string) []string {
		return []string{filepath.Join(directory, "NuGet", "Config", "a.config"), filepath.Join(directory, "NuGet", "Config", "b.Config")}
	}
	m := machine(t, t.TempDir(), "windows", map[string]string{"ProgramFiles(x86)": pf86, "ProgramFiles": programFiles, "NUGET_COMMON_APPLICATION_DATA": common})
	if got := m.NuGetMachineConfigs(); !reflect.DeepEqual(got, files(pf86)) {
		t.Errorf("windows: %v", got)
	}
	m = machine(t, t.TempDir(), "windows", map[string]string{"ProgramFiles": programFiles})
	if got := m.NuGetMachineConfigs(); !reflect.DeepEqual(got, files(programFiles)) {
		t.Errorf("windows without (x86): %v", got)
	}
	m = machine(t, t.TempDir(), "linux", map[string]string{"NUGET_COMMON_APPLICATION_DATA": common})
	if got := m.NuGetMachineConfigs(); !reflect.DeepEqual(got, files(common)) {
		t.Errorf("NUGET_COMMON_APPLICATION_DATA: %v", got)
	}
	m = machine(t, t.TempDir(), "linux", nil)
	touch(t, filepath.Join(SystemRoot, "etc", "opt", "NuGet", "Config", "corp.config"))
	if got := m.NuGetMachineConfigs(); !reflect.DeepEqual(got, []string{filepath.Join(SystemRoot, "etc", "opt", "NuGet", "Config", "corp.config")}) {
		t.Errorf("linux: %v", got)
	}
	m = machine(t, t.TempDir(), "darwin", nil)
	touch(t, filepath.Join(SystemRoot, "Library", "Application Support", "NuGet", "Config", "corp.config"))
	if got := m.NuGetMachineConfigs(); !reflect.DeepEqual(got, []string{filepath.Join(SystemRoot, "Library", "Application Support", "NuGet", "Config", "corp.config")}) {
		t.Errorf("darwin: %v", got)
	}
}

// NuGetPackageSourceCredentials_<source> is found whatever the case of its prefix,
// by the source name lower-cased; an empty one is no credential.
//
// Verifies: REQ-AUTH-022
func TestNuGetCredentialVariables(t *testing.T) {
	m := machine(t, "", "linux", map[string]string{
		"NuGetPackageSourceCredentials_Corp Feed": "Username=a;Password=b",
		"NUGETPACKAGESOURCECREDENTIALS_nuget.org": "Username=c;Password=d",
		"NuGetPackageSourceCredentials_empty":     "",
		"NuGetPackageSourceCredentials_":          "Username=x;Password=y",
		"OTHER_NuGetPackageSourceCredentials_x":   "Username=x;Password=y",
	})
	want := map[string]string{"corp feed": "Username=a;Password=b", "nuget.org": "Username=c;Password=d"}
	if got := m.NuGetCredentialVariables(); !reflect.DeepEqual(got, want) {
		t.Errorf("%v", got)
	}
}
