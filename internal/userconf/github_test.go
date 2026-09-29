package userconf

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// gh's configuration directory is GH_CONFIG_DIR, else gh below XDG_CONFIG_HOME,
// else %AppData%\GitHub CLI on Windows, else ~/.config/gh; hosts.yml holds a
// token per host, none when gh keeps it in the keyring.
//
// Verifies: REQ-SUP-064, REQ-AUTH-036
func TestGitHubCLIHosts(t *testing.T) {
	home, directory, xdg, appData := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	for _, testCase := range []struct {
		goos      string
		variables map[string]string
		want      string
	}{
		{"linux", nil, filepath.Join(home, ".config", "gh")},
		{"darwin", nil, filepath.Join(home, ".config", "gh")},
		{"linux", map[string]string{"GH_CONFIG_DIR": directory, "XDG_CONFIG_HOME": xdg}, directory},
		{"windows", map[string]string{"XDG_CONFIG_HOME": xdg, "APPDATA": appData}, filepath.Join(xdg, "gh")},
		{"windows", map[string]string{"APPDATA": appData}, filepath.Join(appData, "GitHub CLI")},
		{"windows", nil, filepath.Join(home, ".config", "gh")},
	} {
		if got := machine(t, home, testCase.goos, testCase.variables).GitHubCLIConfigDirectory(); got != testCase.want {
			t.Errorf("%s %v: got %s, want %s", testCase.goos, testCase.variables, got, testCase.want)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "hosts.yml"), []byte("GitHub.com:\n    oauth_token: gho_1\n    user: mona\nghe.acme.test:\n    user: mona\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []GitHubHost{{Host: "ghe.acme.test"}, {Host: "github.com", Token: "gho_1"}}
	if got := machine(t, home, "linux", map[string]string{"GH_CONFIG_DIR": directory}).GitHubCLIHosts(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for host, want := range map[string]string{
		"github.com": "https://api.github.com", "acme.ghe.com": "https://api.acme.ghe.com",
		"ghe.acme.test": "https://ghe.acme.test/api/v3", "": "",
	} {
		if got := GitHubAPI(host); got != want {
			t.Errorf("GitHubAPI(%q) = %q, want %q", host, got, want)
		}
	}
}

// PSResourceGet's repositories are in .NET's local application data directory
// (%LOCALAPPDATA%; ~/Library/Application Support on macOS when the file is there;
// else $XDG_DATA_HOME or ~/.local/share), PowerShellGet 2's below %LOCALAPPDATA%
// on Windows, else in PowerShell's cache directory.
//
// Verifies: REQ-SUP-064
func TestPowerShellRepositoryFiles(t *testing.T) {
	home, local, data, cache := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	for _, testCase := range []struct {
		goos               string
		variables          map[string]string
		resourceGet, getV2 string
	}{
		{"linux", nil, filepath.Join(home, ".local", "share", "PSResourceGet", "PSResourceRepository.xml"),
			filepath.Join(home, ".cache", "powershell", "PowerShellGet", "PSRepositories.xml")},
		{"linux", map[string]string{"XDG_DATA_HOME": data, "XDG_CACHE_HOME": cache},
			filepath.Join(data, "PSResourceGet", "PSResourceRepository.xml"),
			filepath.Join(cache, "powershell", "PowerShellGet", "PSRepositories.xml")},
		{"darwin", nil, filepath.Join(home, ".local", "share", "PSResourceGet", "PSResourceRepository.xml"),
			filepath.Join(home, ".cache", "powershell", "PowerShellGet", "PSRepositories.xml")},
		{"windows", map[string]string{"LOCALAPPDATA": local},
			filepath.Join(local, "PSResourceGet", "PSResourceRepository.xml"),
			filepath.Join(local, "Microsoft", "Windows", "PowerShell", "PowerShellGet", "PSRepositories.xml")},
	} {
		m := machine(t, home, testCase.goos, testCase.variables)
		if got := m.PSResourceGetRepositories(); got != testCase.resourceGet {
			t.Errorf("%s %v: PSResourceGet %s, want %s", testCase.goos, testCase.variables, got, testCase.resourceGet)
		}
		if got := m.PowerShellGetRepositories(); got != testCase.getV2 {
			t.Errorf("%s %v: PowerShellGet %s, want %s", testCase.goos, testCase.variables, got, testCase.getV2)
		}
	}
	// .NET 8 moved macOS's local application data.
	moved := filepath.Join(home, "Library", "Application Support", "PSResourceGet", "PSResourceRepository.xml")
	touch(t, moved)
	if got := machine(t, home, "darwin", nil).PSResourceGetRepositories(); got != moved {
		t.Errorf("darwin with the file in Application Support: %s", got)
	}
}
