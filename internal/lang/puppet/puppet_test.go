package puppet

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// The fixture is a control repository: a Puppetfile (Forge modules pinned,
// :latest and without a version; git modules by tag, commit, branch and
// ref; a :local one), r10k's modules/ beside it (installed stdlib and apt, and
// an apache that must not be read), manifests/site.pp with nodes,
// site-modules/ with role and profile (classes, a function in Puppet and
// one in Ruby, a type alias, a plan, a custom type, a template and a file),
// and dist/widget, a module with a metadata.json and a .fixtures.yml whose
// spec/fixtures/modules must not be read. legacy/ holds a Free Pascal .pp and
// a metadata.json that is not a module's.
var (
	stdlibPinned = lang.Target{Ecosystem: ecoForge, Package: "puppetlabs-stdlib", Version: "9.4.1", Pinned: true}
	stdlibRange  = lang.Target{Ecosystem: ecoForge, Package: "puppetlabs-stdlib", Version: ">= 4.13.1 < 10.0.0", Floating: true}
	apache       = lang.Target{Ecosystem: ecoForge, Package: "github.com/puppetlabs/puppetlabs-apache", Version: "v12.0.0", Origin: "https://github.com/puppetlabs/puppetlabs-apache.git"}
	ntp          = lang.Target{Ecosystem: ecoForge, Package: "github.com/puppetlabs/puppetlabs-ntp", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://github.com/puppetlabs/puppetlabs-ntp"}
	firewall     = lang.Target{Ecosystem: ecoForge, Package: "github.com/puppetlabs/puppetlabs-firewall", Version: "main", Floating: true, Origin: "https://github.com/puppetlabs/puppetlabs-firewall"}
	mysql        = lang.Target{Ecosystem: ecoForge, Package: "github.com/puppetlabs/puppetlabs-mysql", Version: "abc1234", Pinned: true, Origin: "git@github.com:puppetlabs/puppetlabs-mysql.git"}
	concat       = lang.Target{Ecosystem: ecoForge, Package: "puppetlabs-concat", Floating: true}
	inifile      = lang.Target{Ecosystem: ecoForge, Package: "puppetlabs-inifile", Floating: true}
	archive      = lang.Target{Ecosystem: ecoForge, Package: "puppet-archive", Version: "1.2.3", Pinned: true}
)

func local(p string) lang.Target { return lang.Target{Local: p} }

// Verifies: REQ-PUPPET-001, REQ-PUPPET-002, REQ-PUPPET-004, REQ-PUPPET-006, REQ-PUPPET-010
func TestManifestImports(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["site-modules/profile/manifests/web.pp"], map[string]lang.Target{
		"Stdlib::Port":                         stdlibPinned, // the Puppetfile above the file
		"Profile::Port":                        local("site-modules/profile/types/port.pp"),
		"inherits profile::params":             local("site-modules/profile/manifests/params.pp"),
		"include apache":                       apache,
		"include apache::mod::ssl":             apache,
		"class { 'apache::mod::php': }":        apache,
		"apache::vhost":                        apache,
		"Class['profile::base']":               local("site-modules/profile/manifests/base.pp"),
		"epp('profile/motd.epp')":              local("site-modules/profile/templates/motd.epp"),
		"puppet:///modules/profile/banner.txt": local("site-modules/profile/files/banner.txt"),
		"profile::greet()":                     local("site-modules/profile/functions/greet.pp"),
		"profile::shout()":                     local("site-modules/profile/lib/puppet/functions/profile/shout.rb"),
		"merge()":                              stdlibPinned, // stdlib's unnamespaced function
		"ini_setting":                          inifile,      // a well-known module's type
		"concat::fragment":                     concat,
		"Firewall":                             firewall, // resource defaults
		"mysql_user":                           mysql,    // named by its module's prefix
		"nagios_host":                          {},       // nothing declares a nagios module
		"profile_thing":                        local("site-modules/profile/lib/puppet/type/profile_thing.rb"),
	})
	langtest.CheckImports(t, res["site-modules/role/manifests/webserver.pp"], map[string]lang.Target{
		"include profile::base": local("site-modules/profile/manifests/base.pp"),
		"include profile::web":  local("site-modules/profile/manifests/web.pp"),
		"include profile::db":   local("site-modules/profile/manifests/db.pp"),
		"contain profile::app":  local("site-modules/profile/manifests/app.pp"),
	})
	langtest.CheckImports(t, res["site-modules/profile/manifests/db.pp"], map[string]lang.Target{
		"class { 'mysql::server': }": mysql,
		"include ntp":                ntp,
	})
	langtest.CheckImports(t, res["manifests/site.pp"], map[string]lang.Target{
		"include role::webserver":   local("site-modules/role/manifests/webserver.pp"),
		"include role::base":        local("site-modules/role"), // no file: the module
		"include stdlib":            stdlibPinned,
		"include apt":               {Ecosystem: ecoForge, Package: "puppetlabs-apt", Version: "9.1.0"}, // installed, not declared
		"include docker":            {Ecosystem: ecoForge, Package: "puppetlabs-docker", Unresolved: true},
		"include unknownmod::thing": {Ecosystem: ecoForge, Package: "unknownmod", Unresolved: true},
	})
	// A module with its own metadata.json: its dependencies come first.
	langtest.CheckImports(t, res["dist/widget/manifests/init.pp"], map[string]lang.Target{
		"Stdlib::Absolutepath":        stdlibRange,
		"include widget::config":      local("dist/widget/manifests/config.pp"),
		"archive":                     archive,
		"file_line":                   stdlibRange,
		"stdlib::ensure_packages()":   stdlibRange,
		"template('widget/conf.erb')": local("dist/widget/templates/conf.erb"),
		"file('widget/data.txt')":     local("dist/widget/files/data.txt"),
	})
	langtest.CheckImports(t, res["dist/widget/manifests/config.pp"], map[string]lang.Target{
		"widget::instance": local("dist/widget/manifests/instance.pp"),
	})
	for p, r := range res {
		if strings.HasPrefix(p, "modules/") || strings.Contains(p, "spec/fixtures/modules/") || p == "legacy/metadata.json" {
			t.Errorf("%s: claimed", p)
		}
		if p == "legacy/widgets.pp" && (len(r.Imports) > 0 || len(r.Symbols) > 0) {
			t.Errorf("Pascal read as Puppet: %+v", r)
		}
	}
}

// Verifies: REQ-PUPPET-005
func TestManifests(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["Puppetfile"], map[string]lang.Target{
		"puppetlabs-stdlib":  stdlibPinned,
		"puppetlabs/concat":  concat,
		"puppetlabs-inifile": inifile,
		"apache":             apache,
		"ntp":                ntp,
		"firewall":           firewall,
		"mysql":              mysql,
	})
	langtest.CheckImports(t, res["dist/widget/metadata.json"], map[string]lang.Target{
		"puppetlabs-stdlib": stdlibRange,
		"puppet-archive":    archive,
	})
	langtest.CheckSymbols(t, res["dist/widget/metadata.json"], map[string]string{"acme-widget": "module"})
	langtest.CheckImports(t, res["dist/widget/.fixtures.yml"], map[string]lang.Target{
		"forge_modules:archive": archive,
		"forge_modules:inifile": inifile,
		// A repository of a module metadata.json declares is that module.
		"repositories:stdlib":       {Ecosystem: ecoForge, Package: "puppetlabs-stdlib", Floating: true, Origin: "https://github.com/puppetlabs/puppetlabs-stdlib.git"},
		"repositories:yumrepo_core": {Ecosystem: ecoForge, Package: "github.com/puppetlabs/puppetlabs-yumrepo_core", Version: "main", Floating: true, Origin: "https://github.com/puppetlabs/puppetlabs-yumrepo_core.git"},
		"repositories:facts":        {Ecosystem: ecoForge, Package: "github.com/puppetlabs/puppetlabs-facts", Version: "v1.4.0", Origin: "https://github.com/puppetlabs/puppetlabs-facts.git"},
	})
}

// Verifies: REQ-PUPPET-003
func TestSymbols(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	for file, want := range map[string]map[string]string{
		"manifests/site.pp":                            {"web01.example.com": "node", `^web\d+$`: "node", "default": "node"},
		"site-modules/profile/manifests/web.pp":        {"profile::web": "class"},
		"site-modules/profile/functions/greet.pp":      {"profile::greet": "function"},
		"site-modules/profile/types/port.pp":           {"Profile::Port": "type"},
		"site-modules/profile/plans/deploy.pp":         {"profile::deploy": "plan"},
		"dist/widget/manifests/instance.pp":            {"widget::instance": "define"},
		"site-modules/profile/manifests/app/config.pp": {"profile::app::config": "class"},
	} {
		langtest.CheckSymbols(t, res[file], want)
	}
}

// --resolve-depth follows what r10k installed: the metadata.json in
// modules/.
//
// Verifies: REQ-PUPPET-007
func TestInstalledDependencies(t *testing.T) {
	root := "testdata/repo"
	r := newResolver(root, langtest.Files(t, root))
	apt := lang.Target{Ecosystem: ecoForge, Package: "puppetlabs-apt", Version: "9.1.0"}
	want := []lang.Target{{Ecosystem: ecoForge, Package: "puppetlabs-stdlib", Version: ">= 9.0.0 < 10.0.0", Floating: true}}
	if got := r.Dependencies(apt); !reflect.DeepEqual(got, want) || !r.Installed(apt) {
		t.Errorf("apt depends on %+v", got)
	}
	if got := r.Dependencies(stdlibPinned); len(got) != 0 || !r.Installed(stdlibPinned) {
		t.Errorf("stdlib depends on %+v", got)
	}
	if r.Installed(apache) {
		t.Error("apache is not installed with a metadata.json")
	}
}

// Verifies: REQ-PUPPET-002
func TestLexer(t *testing.T) {
	src := `$a = "x ${b('y')} \" include no::dq"
$c = 'it''s \' include no::sq'
$d = @(EOT:json/L)
  include no::heredoc
  |-EOT
include yes::one
$e = $f / 2 # include no::comment
if $g =~ /include no::regex/ { include yes::two }
`
	ex := extractSource([]byte(src))
	var got []string
	for _, im := range ex.Imports {
		got = append(got, im.Module+"@"+string(rune('0'+im.Line)))
	}
	if want := []string{"yes::one@6", "yes::two@8"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Every prefix of the fixtures, and pathological inputs, are read in time
// without a panic.
//
// Verifies: REQ-PUPPET-009
func TestTruncated(t *testing.T) {
	var srcs [][]byte
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if b, err := os.ReadFile(p); err == nil {
				srcs = append(srcs, b)
			}
		}
		return nil
	})
	for _, src := range srcs {
		for i := 0; i <= len(src); i++ {
			extractSource(src[:i])
			readPuppetfile(src[:i])
			readMetadata(src[:i])
			readFixtures(src[:i])
		}
	}
	for _, unit := range []string{"{", "[", "(", "\"${", "\"${'", "@(E)\n", "/", "include ", "a::b { ", "class { ", "node ", "Class[", "x::y(", "$", "mod 'a', ", ":git => ", "\"$", "/*", "a { 'x': "} {
		src := []byte(strings.Repeat(unit, 200_000/len(unit)))
		start := time.Now()
		extractSource(src)
		readPuppetfile(src)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%q x %d: %v", unit, 200_000/len(unit), d)
		}
	}
}

// Verifies: REQ-PUPPET-008
func TestEcosystems(t *testing.T) {
	if e := (Plugin{}).Ecosystems(); len(e) != 1 || e[0].ID != ecoForge || e[0].Std {
		t.Errorf("%+v", e)
	}
}
