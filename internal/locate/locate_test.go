package locate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// tree makes each directory under base, with a file where a path ends in one.
func tree(t *testing.T, base string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(base, filepath.FromSlash(p))
		if filepath.Ext(p) == ".txt" {
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte("yaml\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// Verifies: REQ-SRV-018
func TestFolder(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	tree(t, root,
		"node_modules/lodash", "web/node_modules/@scope/tool", "node_modules/@scope/tool",
		"vendor/monolog/monolog", "apps/api/deps/jason", "vendor/github.com/pkg/errors",
		".venv/lib/python3.12/site-packages/yaml", ".venv/lib/python3.12/site-packages/PyYAML-6.0.1.dist-info/top_level.txt",
		".venv/lib/python3.12/site-packages/typing_extensions-4.12.2.dist-info",
		"vendor/bundle/ruby/3.3.0/gems/rake-13.2.1", "libs/local",
	)
	tree(t, home,
		"go/pkg/mod/github.com/!burnt!sushi/toml@v1.4.0", ".cargo/registry/src/index.crates.io-6f17d22bba15001f/serde-1.0.210",
		".m2/repository/com/google/guava/guava/33.3.1-jre", ".nuget/packages/newtonsoft.json/13.0.3", ".pub-cache/hosted/pub.dev/http-1.2.2",
	)
	machine := userconf.Machine{Home: home, GOOS: runtime.GOOS, Environment: func(key string) string {
		if key == "GOENV" {
			return "off"
		}
		return ""
	}}
	// file:///C:/src/lib on Windows.
	fileURL := "file://" + filepath.ToSlash(filepath.Join(root, "libs", "local"))
	if runtime.GOOS == "windows" {
		fileURL = "file:///" + filepath.ToSlash(filepath.Join(root, "libs", "local"))
	}
	for _, c := range []struct {
		p    Package
		want string
	}{
		{Package{Ecosystem: "npm", Name: "lodash", Near: []string{"web/src"}}, filepath.Join(root, "node_modules", "lodash")},
		{Package{Ecosystem: "npm", Name: "@scope/tool", Near: []string{"web/src"}}, filepath.Join(root, "web", "node_modules", "@scope", "tool")},
		{Package{Ecosystem: "npm", Name: "@scope/tool"}, filepath.Join(root, "node_modules", "@scope", "tool")},
		{Package{Ecosystem: "npm", Name: "missing"}, ""},
		{Package{Ecosystem: "npm", Name: "../../etc"}, ""},
		{Package{Ecosystem: "composer", Name: "monolog/monolog"}, filepath.Join(root, "vendor", "monolog", "monolog")},
		{Package{Ecosystem: "hex", Name: "jason", Near: []string{"apps/api/lib"}}, filepath.Join(root, "apps", "api", "deps", "jason")},
		{Package{Ecosystem: "go", Name: "github.com/pkg/errors", Version: "v0.9.1"}, filepath.Join(root, "vendor", "github.com", "pkg", "errors")},
		{Package{Ecosystem: "go", Name: "github.com/BurntSushi/toml", Version: "v1.4.0"}, filepath.Join(home, "go", "pkg", "mod", "github.com", "!burnt!sushi", "toml@v1.4.0")},
		{Package{Ecosystem: "crates", Name: "serde", Version: "1.0.210"}, filepath.Join(home, ".cargo", "registry", "src", "index.crates.io-6f17d22bba15001f", "serde-1.0.210")},
		{Package{Ecosystem: "pypi", Name: "PyYAML"}, filepath.Join(root, ".venv", "lib", "python3.12", "site-packages", "yaml")},
		{Package{Ecosystem: "pypi", Name: "typing-extensions"}, filepath.Join(root, ".venv", "lib", "python3.12", "site-packages", "typing_extensions-4.12.2.dist-info")},
		{Package{Ecosystem: "maven", Name: "com.google.guava:guava", Version: "33.3.1-jre"}, filepath.Join(home, ".m2", "repository", "com", "google", "guava", "guava", "33.3.1-jre")},
		{Package{Ecosystem: "nuget", Name: "Newtonsoft.Json", Version: "13.0.3"}, filepath.Join(home, ".nuget", "packages", "newtonsoft.json", "13.0.3")},
		{Package{Ecosystem: "rubygems", Name: "rake", Version: "13.2.1"}, filepath.Join(root, "vendor", "bundle", "ruby", "3.3.0", "gems", "rake-13.2.1")},
		{Package{Ecosystem: "pub", Name: "http", Version: "1.2.2"}, filepath.Join(home, ".pub-cache", "hosted", "pub.dev", "http-1.2.2")},
		{Package{Ecosystem: "pypi", Name: "local", Origin: fileURL}, filepath.Join(root, "libs", "local")},
		{Package{Ecosystem: "hex", Name: "local", Origin: "path:libs/local"}, filepath.Join(root, "libs", "local")},
		{Package{Ecosystem: "pypi", Name: "remote", Origin: "git+https://github.com/acme/remote.git"}, ""},
		{Package{Ecosystem: "oci", Name: "alpine"}, ""},
	} {
		if got := Folder(root, machine, c.p); got != c.want {
			t.Errorf("Folder(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}
