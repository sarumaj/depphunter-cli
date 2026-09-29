package auth

import (
	"path/filepath"
	"testing"
)

// cSpell: ignore oauth

// gh's hosts.yml (in GH_CONFIG_DIR, else gh below XDG_CONFIG_HOME, else
// ~/.config/gh) holds a token per host; GH_TOKEN and GITHUB_TOKEN are
// github.com's, GH_ENTERPRISE_TOKEN an instance's. Each token goes to its own
// host's API alone: an Enterprise Server's to its /api/v3/ paths, github.com's
// to api.github.com, and neither to the other.
//
// Verifies: REQ-AUTH-036
func TestGitHubTokens(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".config", "gh", "hosts.yml"), `github.com:
    users:
        mona:
            oauth_token: gho_file
    oauth_token: gho_file
    user: mona
ghe.acme.test:
    oauth_token: ghe_file
    user: mona
keyring.test:
    user: mona
acme.ghe.com:
    oauth_token: residency
`)
	for _, test := range []struct {
		name      string
		variables map[string]string
		want      map[string]string
	}{
		{"hosts.yml", nil, map[string]string{
			"https://api.github.com/repos/o/r/contents/action.yml":       "Bearer gho_file",
			"https://ghe.acme.test/api/v3/repos/o/r/contents/action.yml": "Bearer ghe_file",
			"https://ghe.acme.test/o/r":                                  "",
			"https://api.acme.ghe.com/repos/o/r/contents/action.yml":     "Bearer residency",
			"https://keyring.test/api/v3/repos/o/r/contents/action.yml":  "",
			"https://raw.githubusercontent.com/o/r/v1/action.yml":        "",
		}},
		{"GH_TOKEN over GITHUB_TOKEN over the file", map[string]string{"GH_TOKEN": "gh", "GITHUB_TOKEN": "actions"},
			map[string]string{"https://api.github.com/repos/o/r": "Bearer gh"}},
		{"GITHUB_TOKEN", map[string]string{"GITHUB_TOKEN": "actions"},
			map[string]string{"https://api.github.com/repos/o/r": "Bearer actions"}},
		// On an Enterprise Server's runner GITHUB_TOKEN is the instance's.
		{"runner of an Enterprise Server", map[string]string{
			"GITHUB_TOKEN": "runner", "GITHUB_API_URL": "https://ghe.acme.test/api/v3",
		}, map[string]string{
			"https://ghe.acme.test/api/v3/repos/o/r": "Bearer runner",
			"https://api.github.com/repos/o/r":       "Bearer gho_file",
		}},
		{"GH_ENTERPRISE_TOKEN for GH_HOST", map[string]string{
			"GH_ENTERPRISE_TOKEN": "enterprise", "GH_HOST": "ghe.acme.test",
		}, map[string]string{
			"https://ghe.acme.test/api/v3/repos/o/r": "Bearer enterprise",
			"https://api.github.com/repos/o/r":       "Bearer gho_file",
		}},
		{"GH_CONFIG_DIR moves hosts.yml", map[string]string{"GH_CONFIG_DIR": t.TempDir()},
			map[string]string{"https://api.github.com/repos/o/r": "", "https://ghe.acme.test/api/v3/x": ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := onMachine(t, home, "linux", test.variables)
			for address, want := range test.want {
				if got := authorization(t, c, address); got != want {
					t.Errorf("%s: %q, want %q", address, got, want)
				}
			}
		})
	}
}
