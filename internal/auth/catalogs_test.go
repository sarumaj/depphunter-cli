package auth

import (
	"path/filepath"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// BUF_TOKEN's entries each go to their host, and a token without a host to
// buf.build alone; `buf registry login`'s netrc entry is the token for its host,
// sent as a Bearer token rather than the netrc's Basic pair.
//
// Verifies: REQ-AUTH-031
func TestBufTokens(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".netrc"), "machine buf.corp.example login me password netrc-token\n")
	c := Read(home, environment(map[string]string{"BUF_TOKEN": "a-token@buf.build, b-token@BUF.Other.example"}))
	for host, want := range map[string]string{
		"buf.build": "a-token", "buf.other.example": "b-token", "buf.corp.example": "netrc-token", "buf.none.example": "",
	} {
		if got := c.BufToken(host); got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
	c = Read(home, environment(map[string]string{"BUF_TOKEN": "bare-token"}))
	if c.BufToken("buf.build") != "bare-token" || c.BufToken("buf.other.example") != "" {
		t.Error("a bare BUF_TOKEN reached another registry")
	}
	var none *Store
	if none.BufToken("buf.build") != "" {
		t.Error("a nil store had a token")
	}
}

// cue login's tokens go to the registry host they were stored for, before a
// Docker credential for the same host; the file is found through CUE_CONFIG_DIR,
// else cue's directory in the platform's configuration directory.
//
// Verifies: REQ-AUTH-032
func TestCUELogins(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".config", "cue", "logins.json"),
		`{"registries":{"registry.cue.works":{"access_token":"cue-token","token_type":"Bearer","refresh_token":"r"},"other.example":{"access_token":"x","token_type":"mac"}}}`)
	writeFile(t, filepath.Join(home, ".docker", "config.json"), `{"auths":{"registry.cue.works":{"auth":"dXNlcjpwYXNz"}}}`)
	c := onMachine(t, home, "linux", nil)
	if got := authorization(t, c, "https://registry.cue.works/v2/"); got != "Bearer cue-token" {
		t.Errorf("registry.cue.works: %q", got)
	}
	if got := authorization(t, c, "https://other.example/v2/"); got != "" {
		t.Errorf("a token of another type was sent: %q", got)
	}
	moved := t.TempDir()
	writeFile(t, filepath.Join(moved, "logins.json"), `{"registries":{"cue.corp.example":{"access_token":"corp-token"}}}`)
	c = onMachine(t, home, "linux", map[string]string{"CUE_CONFIG_DIR": moved})
	if got := authorization(t, c, "https://cue.corp.example/v2/"); got != "Bearer corp-token" {
		t.Errorf("CUE_CONFIG_DIR: %q", got)
	}
}

// r10k's authorization_token is the whole header value, sent to its Forge's paths
// only; without a baseurl it is for the public Forge.
//
// Verifies: REQ-AUTH-033
func TestR10KForgeToken(t *testing.T) {
	saved := userconf.SystemRoot
	userconf.SystemRoot = t.TempDir()
	t.Cleanup(func() { userconf.SystemRoot = saved })
	writeFile(t, filepath.Join(userconf.SystemRoot, "etc", "puppetlabs", "r10k", "r10k.yaml"),
		"forge:\n  baseurl: https://forge.corp.example/api\n  authorization_token: 'Bearer secret'\n")
	c := Read(t.TempDir(), nil)
	if got := authorization(t, c, "https://forge.corp.example/api/v3/modules/corp-base"); got != "Bearer secret" {
		t.Errorf("forge: %q", got)
	}
	if got := authorization(t, c, "https://forge.corp.example/other"); got != "" {
		t.Errorf("another path: %q", got)
	}
	writeFile(t, filepath.Join(userconf.SystemRoot, "etc", "puppetlabs", "r10k", "r10k.yaml"), "forge:\n  authorization_token: plain-token\n")
	c = Read(t.TempDir(), nil)
	if got := authorization(t, c, "https://forgeapi.puppet.com/v3/modules/corp-base"); got != "Bearer plain-token" {
		t.Errorf("public forge: %q", got)
	}
}
