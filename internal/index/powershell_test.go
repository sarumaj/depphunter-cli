package index

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// cSpell: ignore PSResourceGet Clixml odata

// psResourceRepositories is a PSResourceRepository.xml as Register-PSResourceRepository
// writes it.
const psResourceRepositories = `<?xml version="1.0" encoding="utf-8"?>
<configuration>
  <Repository Name="PSGallery" Url="https://www.powershellgallery.com/api/v2" APIVersion="V2" Priority="50" Trusted="false" />
  <Repository Name="MicrosoftArtifactRegistry" Url="https://mcr.microsoft.com/" APIVersion="ContainerRegistry" Priority="40" Trusted="true" />
  <Repository Name="Local" Url="file:///C:/modules" APIVersion="Local" Priority="10" Trusted="true" />
  <Repository Name="Corp" Uri="https://nuget.corp.test/v3/index.json" APIVersion="V3" Priority="20" Trusted="true" />
  <Repository Name="Beta" Url="https://beta.corp.test/api/v2" APIVersion="V2" Priority="20" Trusted="true" />
</configuration>`

// psRepositories is a PSRepositories.xml as PowerShellGet 2 serializes it.
const psRepositories = `<Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04">
  <Obj RefId="0">
    <TN RefId="0"><T>System.Collections.Specialized.OrderedDictionary</T><T>System.Object</T></TN>
    <DCT>
      <En>
        <S N="Key">Legacy</S>
        <Obj N="Value" RefId="1">
          <TN RefId="1"><T>Microsoft.PowerShell.Commands.PSRepository</T><T>System.Management.Automation.PSCustomObject</T><T>System.Object</T></TN>
          <MS>
            <S N="Name">Legacy</S>
            <S N="SourceLocation">https://legacy.corp.test/nuget</S>
            <B N="Trusted">true</B>
            <S N="PackageManagementProvider">NuGet</S>
          </MS>
        </Obj>
      </En>
      <En>
        <S N="Key">Share</S>
        <Obj N="Value" RefId="2">
          <TNRef RefId="1" />
          <MS>
            <S N="Name">Share</S>
            <S N="SourceLocation">\\server\share</S>
          </MS>
        </Obj>
      </En>
    </DCT>
  </Obj>
</Objs>`

// PSResourceGet's repositories (by priority, then name) and PowerShellGet 2's
// are asked in order, the Gallery only when one of them is it; a local
// repository and a container registry are not asked. An install that names a
// repository is asked of that one alone.
//
// Verifies: REQ-SUP-078, REQ-SUP-064
func TestPowerShellRepositories(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".local", "share", "PSResourceGet", "PSResourceRepository.xml"), psResourceRepositories)
	put(t, filepath.Join(home, ".cache", "powershell", "PowerShellGet", "PSRepositories.xml"), psRepositories)
	config := discoverOn(home, "linux", nil)
	gallery := "https://www.powershellgallery.com/api/v2"
	want := []string{"https://beta.corp.test/api/v2", "https://nuget.corp.test/v3/index.json", gallery, "https://legacy.corp.test/nuget"}
	if got := order(config, PowerShell, "Pester", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for registry, want := range map[string][]string{
		"corp":      {"https://nuget.corp.test/v3/index.json"},
		"Legacy":    {"https://legacy.corp.test/nuget"},
		"PSGallery": {gallery},
		"Unknown":   nil,
	} {
		if got := order(config, PowerShell, "Pester", registry); !reflect.DeepEqual(got, want) {
			t.Errorf("-Repository %s: %v, want %v", registry, got, want)
		}
	}
	// Without the Gallery registered, it is not asked, even by name.
	put(t, filepath.Join(home, ".local", "share", "PSResourceGet", "PSResourceRepository.xml"),
		`<configuration><Repository Name="Corp" Url="https://corp.test/api/v2" APIVersion="V2" Priority="50" Trusted="true" /></configuration>`)
	config = discoverOn(home, "linux", map[string]string{"XDG_CACHE_HOME": t.TempDir()})
	if got := order(config, PowerShell, "Pester", ""); !reflect.DeepEqual(got, []string{"https://corp.test/api/v2"}) {
		t.Errorf("unregistered Gallery: %v", got)
	}
	if got := order(config, PowerShell, "Pester", "PSGallery"); got != nil {
		t.Errorf("unregistered Gallery by name: %v", got)
	}
	// Windows keeps both below LOCALAPPDATA; nothing registered means the Gallery.
	local := t.TempDir()
	put(t, filepath.Join(local, "PSResourceGet", "PSResourceRepository.xml"), psResourceRepositories)
	config = discoverOn(t.TempDir(), "windows", map[string]string{"LOCALAPPDATA": local})
	if got := order(config, PowerShell, "Pester", ""); !reflect.DeepEqual(got, want[:3]) {
		t.Errorf("windows: %v", got)
	}
	if got := order(discoverOn(t.TempDir(), "linux", nil), PowerShell, "Pester", ""); !reflect.DeepEqual(got, []string{gallery}) {
		t.Errorf("nothing registered: %v", got)
	}
}

// odataEntry is one version of a module as the Gallery's v2 API answers it.
func odataEntryXML(name, version, dependencies string, prerelease bool) string {
	return fmt.Sprintf(`<entry><id>https://g/api/v2/Packages(Id='%[1]s',Version='%[2]s')</id><title type="text">%[1]s</title>
<m:properties><d:Id>%[1]s</d:Id><d:Version>%[2]s</d:Version><d:NormalizedVersion>%[2]s</d:NormalizedVersion>
<d:Dependencies>%[3]s</d:Dependencies><d:IsPrerelease m:type="Edm.Boolean">%[4]v</d:IsPrerelease></m:properties></entry>`,
		name, version, dependencies, prerelease)
}

func odataFeed(next string, entries ...string) string {
	link := ""
	if next != "" {
		link = `<link rel="next" href="` + next + `" />`
	}
	return `<?xml version="1.0" encoding="utf-8"?><feed xml:base="https://g/api/v2" xmlns="http://www.w3.org/2005/Atom"
 xmlns:d="http://schemas.microsoft.com/ado/2007/08/dataservices" xmlns:m="http://schemas.microsoft.com/ado/2007/08/dataservices/metadata">
<title type="text">FindPackagesById</title>` + strings.Join(entries, "") + link + `</feed>`
}

// A module's dependencies are the Dependencies property of its entry in the v2
// OData API: one version through Packages(Id,Version), else the newest release
// FindPackagesById lists (through its pages) that the requirement admits -
// ModuleVersion's minimum or a NuGet range. A NuGet v3 feed is read as NuGet's.
//
// Verifies: REQ-SUP-078
func TestPowerShellGalleryModules(t *testing.T) {
	recorded := &recorder{}
	bodies := map[string]string{}
	gallery := files(recorded, bodies)
	t.Cleanup(gallery.Close)
	for path, body := range map[string]string{
		"/api/v2/FindPackagesById()?id='Az.Storage'": odataFeed(gallery.URL+"/api/v2/FindPackagesById()?id='Az.Storage'&amp;$skip=2",
			odataEntryXML("Az.Storage", "5.9.0", "Az.Accounts:[2.12.1, ):|Az.Accounts:[2.12.1, ):net6.0", false),
			odataEntryXML("Az.Storage", "6.1.0", "Az.Accounts:[2.19.0, ):", false)),
		"/api/v2/FindPackagesById()?id='Az.Storage'&$skip=2": odataFeed("",
			odataEntryXML("Az.Storage", "7.0.0-preview", "Az.Accounts:[3.0.0, ):", true),
			odataEntryXML("Az.Storage", "6.10.0", "Az.Accounts:[2.19.0, ):|PSReadLine:2.3.4:", false)),
		"/api/v2/Packages(Id='Pester',Version='5.5.0')": odataEntryXML("Pester", "5.5.0", "", false),
		"/api/v2/Packages(Id='Az',Version='11.0.0')": `<?xml version="1.0"?><entry xmlns="http://www.w3.org/2005/Atom"
 xmlns:d="http://schemas.microsoft.com/ado/2007/08/dataservices" xmlns:m="http://schemas.microsoft.com/ado/2007/08/dataservices/metadata">` +
			strings.TrimPrefix(odataEntryXML("Az", "11.0.0", "Az.Accounts:[2.13.2]:|Az.Storage:[6.0, 7.0):", false), "<entry>"),
	} {
		bodies[path] = body
	}
	c := publicClient(t, PowerShell, gallery.URL+"/api/v2", nil)
	for _, test := range []struct {
		target lang.Target
		want   []string
	}{
		{lang.Target{Ecosystem: PowerShell, Package: "Az.Storage"}, []string{"Az.Accounts [2.19.0, )", "PSReadLine 2.3.4"}},
		{lang.Target{Ecosystem: PowerShell, Package: "Az.Storage", Version: "[5.0,6.0)"}, []string{"Az.Accounts [2.12.1, )"}},
		// ModuleVersion is a minimum.
		{lang.Target{Ecosystem: PowerShell, Package: "Az.Storage", Version: "6.2"}, []string{"Az.Accounts [2.19.0, )", "PSReadLine 2.3.4"}},
		{lang.Target{Ecosystem: PowerShell, Package: "Pester", Version: "5.5.0", Pinned: true}, []string{}},
		{lang.Target{Ecosystem: PowerShell, Package: "Az", Version: "[11.0.0]"}, []string{"Az.Accounts [2.13.2]", "Az.Storage [6.0, 7.0)"}},
	} {
		_, lookup := ask(t, c, test.target)
		if got := dependencyNames(c.Dependencies(test.target)); lookup.Answer != trace.FromIndex || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s %s: %v %+v, want %v", test.target.Package, test.target.Version, got, lookup, test.want)
		}
	}
	// "[2.13.2]" is one version, a range is not.
	for _, d := range c.Dependencies(lang.Target{Ecosystem: PowerShell, Package: "Az", Version: "[11.0.0]"}) {
		if d.Pinned != (d.Package == "Az.Accounts") {
			t.Errorf("%s %s pinned %v", d.Package, d.Version, d.Pinned)
		}
	}
	if _, lookup := ask(t, c, lang.Target{Ecosystem: PowerShell, Package: "Az.Storage", Version: "[9.0,)"}); lookup.Answer != trace.NoAnswer {
		t.Errorf("a range nothing admits: %+v", lookup)
	}
}

// A NuGet v3 feed registered as a repository is read by the NuGet client; a
// module the organization owns is not named to the Gallery.
//
// Verifies: REQ-SUP-078, REQ-SUP-038
func TestPowerShellFeedsAndPrivateModules(t *testing.T) {
	feed := newNuGetStub(t, "", "", map[string]string{
		"Acme.Tools": "Acme.Core",
	})
	gallery := &recorder{}
	galleryServer := files(gallery, nil)
	t.Cleanup(galleryServer.Close)
	withPublic(t, PowerShell, galleryServer.URL+"/api/v2")
	config := New()
	config.Add(PowerShell, Source{URL: feed.index(), Kind: Listed, Trusted: true, Origin: OriginMachine, powershellName: "Corp"})
	config.Add(PowerShell, Source{URL: galleryServer.URL + "/api/v2", Kind: Listed, Trusted: true, Origin: OriginMachine, powershellName: "PSGallery"})
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, nil, scope.New([]string{"psgallery:Acme.*"}))
	if got, lookup := ask(t, c, lang.Target{Ecosystem: PowerShell, Package: "Acme.Tools", Version: "1.0.0", Pinned: true}); !reflect.DeepEqual(got, []string{"Acme.Core"}) {
		t.Errorf("v3 feed: %v %+v", got, lookup)
	}
	if _, lookup := ask(t, c, lang.Target{Ecosystem: PowerShell, Package: "Acme.Missing", Version: "1.0.0", Pinned: true}); lookup.Answer != trace.NoAnswer {
		t.Errorf("missing: %+v", lookup)
	}
	if asked := gallery.take(); len(asked) != 0 {
		t.Errorf("the Gallery was asked about the organization's modules: %v", asked)
	}
}

// Verifies: REQ-SUP-078
func TestPowerShellRequirements(t *testing.T) {
	for _, test := range []struct {
		requirement, version string
		want                 bool
	}{
		{"", "1.0", true}, {"1.2", "1.10", true}, {"1.2", "1.1.9", false},
		{"[1.0,2.0)", "1.9.9", true}, {"[1.0,2.0)", "2.0", false}, {"(1.0,)", "1.0", false},
		{"(,2.0]", "2.0", true}, {"[1.5]", "1.5.0", true}, {"[1.5]", "1.6", false},
	} {
		if got := powershellAdmits(test.requirement, test.version); got != test.want {
			t.Errorf("%q admits %s: %v", test.requirement, test.version, got)
		}
	}
	for requirement, want := range map[string]string{"[1.2.3]": "1.2.3", "1.2.3": "1.2.3", "[1.0, )": "", "": "", "(1.0,2.0)": ""} {
		if got := powershellExact(requirement); got != want {
			t.Errorf("exact %q: %q", requirement, got)
		}
	}
}
