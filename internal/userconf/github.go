package userconf

import (
	"cmp"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------- GitHub CLI

// GitHubCLIConfigDirectory is where the GitHub CLI keeps hosts.yml, as gh finds
// it: `GH_CONFIG_DIR`, else gh below `XDG_CONFIG_HOME`, else `%AppData%\GitHub
// CLI` on Windows (when %AppData% is set), else ~/.config/gh.
//
// Implements: REQ-SUP-064
func (m Machine) GitHubCLIConfigDirectory() string {
	return cmp.Or(m.Environment("GH_CONFIG_DIR"), m.xdg("XDG_CONFIG_HOME", "gh"),
		m.on(m.under("APPDATA", "GitHub CLI"), "windows"), m.home(".config", "gh"))
}

// GitHubHost is a host `gh auth login` logged in to, from hosts.yml: its name,
// and the token gh keeps in the file ("" when gh keeps it in the system's
// keyring, which is not read).
type GitHubHost struct {
	Host, Token string
}

// GitHubCLIHosts reads the hosts of the GitHub CLI's hosts.yml, sorted by name:
// `<host>: {oauth_token: ..., user: ...}`.
//
// Implements: REQ-SUP-064, REQ-AUTH-036
func (m Machine) GitHubCLIHosts() []GitHubHost {
	name := join(m.GitHubCLIConfigDirectory(), "hosts.yml")
	if name == "" {
		return nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	var doc map[string]struct {
		Token string `yaml:"oauth_token"`
	}
	if yaml.Unmarshal(data, &doc) != nil {
		return nil
	}
	out := make([]GitHubHost, 0, len(doc))
	for host, entry := range doc {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			out = append(out, GitHubHost{Host: host, Token: strings.TrimSpace(entry.Token)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}

// GitHubAPI is the REST API of a GitHub host, as gh addresses it: api.github.com
// for github.com, api.<host> for a GitHub Enterprise Cloud host with data
// residency (<name>.ghe.com), and <host>/api/v3 for a GitHub Enterprise Server.
//
// Implements: REQ-AUTH-036
func GitHubAPI(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	switch {
	case host == "":
		return ""
	case host == "github.com", strings.HasSuffix(host, ".github.com"):
		return "https://api.github.com"
	case strings.HasSuffix(host, ".ghe.com"):
		return "https://api." + strings.TrimPrefix(host, "api.")
	}
	return "https://" + host + "/api/v3"
}

// ---------------------------------------------------------------- PowerShell

// PSResourceGetRepositories is PSResourceGet's PSResourceRepository.xml, which
// Register-PSResourceRepository writes, in .NET's local application data
// directory: `%LOCALAPPDATA%` on Windows; on macOS ~/Library/Application Support
// (.NET 8 and later, PowerShell 7.4) when the file is there; else
// `$XDG_DATA_HOME`, else ~/.local/share.
//
// Implements: REQ-SUP-064
func (m Machine) PSResourceGetRepositories() string {
	const file = "PSResourceRepository.xml"
	if m.GOOS == "windows" {
		return m.under("LOCALAPPDATA", "PSResourceGet", file)
	}
	dataHome := cmp.Or(m.xdg("XDG_DATA_HOME"), m.home(".local", "share"))
	macOS := m.on(firstFile(m.applicationSupport("PSResourceGet", file)), "darwin")
	return cmp.Or(macOS, join(dataHome, "PSResourceGet", file))
}

// PowerShellGetRepositories is PowerShellGet 2's PSRepositories.xml, which
// Register-PSRepository writes: below `%LOCALAPPDATA%` on Windows
// (Microsoft\Windows\PowerShell\PowerShellGet), else in PowerShell's cache
// directory (`$XDG_CACHE_HOME/powershell`, else ~/.cache/powershell).
//
// Implements: REQ-SUP-064
func (m Machine) PowerShellGetRepositories() string {
	const file = "PSRepositories.xml"
	if m.GOOS == "windows" {
		return m.under("LOCALAPPDATA", "Microsoft", "Windows", "PowerShell", "PowerShellGet", file)
	}
	return join(cmp.Or(m.xdg("XDG_CACHE_HOME"), m.home(".cache")), "powershell", "PowerShellGet", file)
}
