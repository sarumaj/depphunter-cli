package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------- Terraform registries

// Terraform and OpenTofu keep API tokens for private registries (HCP Terraform, a
// company's own) in their CLI configuration - credentials "host" { token = "..." } in
// ~/.terraformrc, ~/.tofurc or the file TF_CLI_CONFIG_FILE names - in the
// credentials.tfrc.json that `terraform login` writes, and in TF_TOKEN_<host>
// variables. A host the configuration names in a credentials or host block is one
// this machine knows (see TerraformHost), with or without a token.

var (
	tfBlock = regexp.MustCompile(`(?m)^\s*(credentials|host)\s+"([^"]+)"\s*\{`)
	tfToken = regexp.MustCompile(`^\s*token\s*=\s*"([^"]*)"`)
)

// readTerraform reads the CLI configuration files and the tokens they and the
// environment hold.
//
// Implements: REQ-AUTH-015
func (c *Store) readTerraform(home string, env func(string) string) {
	var files []string
	if f := env("TF_CLI_CONFIG_FILE"); f != "" {
		files = append(files, f)
	} else {
		files = append(files, filepath.Join(home, ".terraformrc"), filepath.Join(home, ".tofurc"))
	}
	for _, name := range files {
		if data, err := os.ReadFile(name); err == nil {
			c.readTerraformRC(data)
		}
	}
	for _, dir := range []string{".terraform.d", filepath.Join(".config", "opentofu")} {
		if data, err := os.ReadFile(filepath.Join(home, dir, "credentials.tfrc.json")); err == nil {
			var doc struct {
				Credentials map[string]struct{ Token string } `json:"credentials"`
			}
			if json.Unmarshal(data, &doc) == nil {
				for host, cred := range doc.Credentials {
					c.terraformHost(host, cred.Token)
				}
			}
		}
	}
	// The variable for a host is its name with "." as "_" and "-" as "__": there is
	// no listing them, so the hosts known so far and HCP Terraform's are looked up.
	hosts := []string{"app.terraform.io"}
	for h := range c.terraform {
		hosts = append(hosts, h)
	}
	for _, h := range hosts {
		v := "TF_TOKEN_" + strings.ReplaceAll(strings.ReplaceAll(h, "-", "__"), ".", "_")
		if token := env(v); token != "" {
			c.terraformHost(h, token)
		}
	}
}

// readTerraformRC reads the credentials and host blocks of a CLI configuration.
func (c *Store) readTerraformRC(data []byte) {
	src := string(data)
	for _, m := range tfBlock.FindAllStringSubmatchIndex(src, -1) {
		kind, host := src[m[2]:m[3]], src[m[4]:m[5]]
		token := ""
		if kind == "credentials" {
			body := src[m[1]:]
			if end := strings.IndexByte(body, '}'); end >= 0 {
				body = body[:end]
			}
			for _, line := range strings.Split(body, "\n") {
				if t := tfToken.FindStringSubmatch(line); t != nil {
					token = t[1]
				}
			}
		}
		c.terraformHost(host, token)
	}
}

func (c *Store) terraformHost(host, token string) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return
	}
	if c.terraform == nil {
		c.terraform = map[string]bool{}
	}
	c.terraform[host] = true
	if token != "" {
		c.bearer[host] = token
	}
}

// TerraformHost reports whether this machine's Terraform or OpenTofu configuration
// names a registry host, with or without a token for it.
//
// Implements: REQ-AUTH-015, REQ-SUP-050
func (c *Store) TerraformHost(host string) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.terraform[strings.ToLower(host)]
}
