package index

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// nugetStub is a v3 NuGet feed serving one version (1.0.0) of each package given,
// with the dependency given, under /v3/index.json. With a user it answers only
// requests carrying that Basic credential. It remembers what was asked of it and
// how many requests it refused.
type nugetStub struct {
	*httptest.Server
	mu      sync.Mutex
	asked   []string
	refused int
}

func newNuGetStub(t *testing.T, user, pass string, packages map[string]string) *nugetStub {
	t.Helper()
	f := &nugetStub{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.asked = append(f.asked, r.URL.Path)
		if u, p, _ := r.BasicAuth(); user != "" && (u != user || p != pass) {
			f.refused++
			f.mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Unlock()
		if r.URL.Path == "/v3/index.json" {
			fmt.Fprintf(w, `{"resources":[{"@id":"http://%s/flat","@type":"PackageBaseAddress/3.0.0"}]}`, r.Host)
			return
		}
		for id, dependency := range packages {
			id = strings.ToLower(id)
			switch r.URL.Path {
			case "/flat/" + id + "/index.json":
				fmt.Fprint(w, `{"versions":["1.0.0"]}`)
				return
			case "/flat/" + id + "/1.0.0/" + id + ".nuspec":
				fmt.Fprintf(w, `<package><metadata><dependencies><dependency id="%s" version="1.0.0"/></dependencies></metadata></package>`, dependency)
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *nugetStub) index() string { return f.URL + "/v3/index.json" }

// askedFor reports whether any request named the package (lower-cased).
func (f *nugetStub) askedFor(packageName string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.asked {
		if strings.Contains(p, strings.ToLower(packageName)) {
			return true
		}
	}
	return false
}

func (f *nugetStub) requests() (asked, refused int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked), f.refused
}

// basicOf is the user:password the store sends with a GET of u, "" for none.
func basicOf(s *auth.Store, u string) string {
	request, _ := http.NewRequest(http.MethodGet, u, nil)
	s.Apply(request)
	if user, pass, ok := request.BasicAuth(); ok {
		return user + ":" + pass
	}
	return ""
}

// userNuGet writes the user's NuGet.Config under home.
func userNuGet(t *testing.T, home, body string) {
	t.Helper()
	put(t, filepath.Join(home, ".nuget", "NuGet", "NuGet.Config"), "<configuration>"+body+"</configuration>")
}

// nugetEnvironment is the environment of a test that sets NuGetPackageSourceCredentials_
// variables with t.Setenv: those are listed by the process environment, and only
// they are answered.
func nugetEnvironment(t *testing.T, variables map[string]string) func(string) string {
	t.Helper()
	for k, v := range variables {
		t.Setenv(k, v)
	}
	return func(k string) string { return variables[k] }
}

// packageSourceMapping sends a package its patterns cover to the mapped feed alone:
// Contoso.* is never named to nuget.org, even when the private feed lacks it; a
// more specific pattern wins, an exact id over any prefix; a package no pattern
// covers is asked as without a mapping.
//
// Verifies: REQ-SUP-065
func TestNuGetSourceMapping(t *testing.T) {
	private := newNuGetStub(t, "", "", map[string]string{"Contoso.Billing": "Contoso.Core"})
	org := newNuGetStub(t, "", "", map[string]string{"Newtonsoft.Json": "System.Runtime", "Contoso.Public.Sdk": "Contoso.Abstractions"})
	was := public[NuGet]
	public[NuGet] = org.index()
	t.Cleanup(func() { public[NuGet] = was })

	home := t.TempDir()
	userNuGet(t, home, `<packageSources>
    <add key="nuget.org" value="https://api.nuget.org/v3/index.json"/>
    <add key="contoso" value="`+private.index()+`"/>
  </packageSources>
  <packageSourceMapping>
    <packageSource key="contoso"><package pattern="Contoso.*"/><package pattern="Fabrikam.Tool"/></packageSource>
    <packageSource key="nuget.org"><package pattern="Contoso.Public.*"/><package pattern="Fabrikam.*"/></packageSource>
  </packageSourceMapping>`)
	c := newClient(t, Discover(nil, environment(nil), home))

	got, l := ask(t, c, lang.Target{Ecosystem: NuGet, Package: "Contoso.Billing", Version: "1.0.0"})
	if !slices.Equal(got, []string{"Contoso.Core"}) || l.Index != private.index() {
		t.Errorf("Contoso.Billing: %v from %s, want the mapped feed's answer", got, l.Index)
	}
	if got, _ := ask(t, c, lang.Target{Ecosystem: NuGet, Package: "Contoso.Missing", Version: "1.0.0"}); len(got) != 0 {
		t.Errorf("Contoso.Missing: %v", got)
	}
	if org.askedFor("contoso.billing") || org.askedFor("contoso.missing") {
		t.Error("a package mapped to the private feed was named to nuget.org")
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: NuGet, Package: "Contoso.Public.Sdk", Version: "1.0.0"}); len(got) != 1 || l.Index != org.index() {
		t.Errorf("Contoso.Public.Sdk: %v from %s, want nuget.org's (the longer pattern)", got, l.Index)
	}
	if private.askedFor("contoso.public") {
		t.Error("the shorter pattern's feed was asked for Contoso.Public.Sdk")
	}
	// Unmapped: both, the feed first.
	if got, l := ask(t, c, lang.Target{Ecosystem: NuGet, Package: "Newtonsoft.Json", Version: "1.0.0"}); len(got) != 1 || l.Index != org.index() {
		t.Errorf("Newtonsoft.Json: %v from %s", got, l.Index)
	}
	if !private.askedFor("newtonsoft") {
		t.Error("an unmapped package was not asked as without a mapping")
	}

	config := Discover(nil, environment(nil), home)
	for packageName, want := range map[string][]string{
		"Fabrikam.Tool":    {private.index()}, // exact id over Fabrikam.*
		"fabrikam.tool":    {private.index()}, // ids compare without case
		"Fabrikam.Other":   {org.index()},
		"Contoso.Anything": {private.index()},
		"Other.Package":    {private.index(), org.index()},
	} {
		if got := order(config, NuGet, packageName, ""); !slices.Equal(got, want) {
			t.Errorf("%s: asked %v, want %v", packageName, got, want)
		}
	}

	// Mapped to a disabled source only: asked of nothing, not of nuget.org.
	userNuGet(t, home, `<packageSources><add key="contoso" value="`+private.index()+`"/></packageSources>
  <disabledPackageSources><add key="contoso" value="true"/></disabledPackageSources>
  <packageSourceMapping><packageSource key="contoso"><package pattern="Contoso.*"/></packageSource></packageSourceMapping>`)
	if got := order(Discover(nil, environment(nil), home), NuGet, "Contoso.Billing", ""); len(got) != 0 {
		t.Errorf("mapped to a disabled source: asked %v", got)
	}
}

// A disabled source is not asked, from whichever file disables it; a disabled
// nuget.org switches the public default off, and a closer file can enable a source
// again.
//
// Verifies: REQ-SUP-065
func TestNuGetDisabledSources(t *testing.T) {
	on := newNuGetStub(t, "", "", map[string]string{"Acme.Tools": "Acme.Core"})
	off := newNuGetStub(t, "", "", map[string]string{"Acme.Tools": "Acme.Other"})
	home := t.TempDir()
	userNuGet(t, home, `<packageSources><add key="on" value="`+on.index()+`"/><add key="off" value="`+off.index()+`"/></packageSources>
  <disabledPackageSources><add key="Off" value="true"/><add key="nuget.org" value="true"/></disabledPackageSources>`)
	c := newClient(t, Discover(nil, environment(nil), home))
	if got, l := ask(t, c, lang.Target{Ecosystem: NuGet, Package: "Acme.Tools", Version: "1.0.0"}); !slices.Equal(got, []string{"Acme.Core"}) || l.Index != on.index() {
		t.Errorf("Acme.Tools: %v from %s", got, l.Index)
	}
	if n, _ := off.requests(); n != 0 {
		t.Errorf("the disabled feed was asked %d times", n)
	}
	if got := order(Discover(nil, environment(nil), home), NuGet, "Newtonsoft.Json", ""); !slices.Equal(got, []string{on.index()}) {
		t.Errorf("nuget.org disabled: asked %v", got)
	}
	// The repository's nuget.config enables it again (value false), and disables the other.
	repository := write(t, map[string]string{"nuget.config": `<configuration><disabledPackageSources>
    <add key="off" value="false"/><add key="on" value="true"/></disabledPackageSources></configuration>`})
	if got := order(Discover(repository, environment(nil), home), NuGet, "Acme.Tools", ""); !slices.Equal(got, []string{off.index()}) {
		t.Errorf("re-enabled by the repository: asked %v", got)
	}
}

// NuGet's configuration files are one stack: the machine-wide files, the user's,
// then the repository's, a closer <clear/> dropping every source farther away -
// this machine's feeds included - and a closer file's key replacing a farther
// one's URL. What the repository says is forgotten with it.
//
// Verifies: REQ-SUP-065, REQ-SUP-064
func TestNuGetConfigLayers(t *testing.T) {
	home := t.TempDir()
	userNuGet(t, home, `<packageSources><add key="corp" value="https://nuget.corp/v3/index.json"/></packageSources>`)
	put(t, filepath.Join(home, "etc", "NuGet", "Config", "vendor.config"),
		`<configuration><packageSources><add key="vendor" value="https://vendor.example/v3/index.json"/>`+
			`<add key="corp" value="https://old.corp/v3/index.json"/></packageSources></configuration>`)
	d := NewDiscoverer(environment(map[string]string{"NUGET_COMMON_APPLICATION_DATA": filepath.Join(home, "etc")}), home)
	const org = "https://api.nuget.org/v3/index.json"
	c := d.Discover(nil)
	// The machine-wide file's sources come first; the user's corp URL wins its key.
	if got := order(c, NuGet, "Acme.Tools", ""); !slices.Equal(got, []string{"https://vendor.example/v3/index.json", "https://nuget.corp/v3/index.json", org}) {
		t.Errorf("machine layers: %v", got)
	}

	repository := write(t, map[string]string{
		"nuget.config":     `<configuration><packageSources><clear/><add key="repo" value="https://repo.feed/v3/index.json"/></packageSources></configuration>`,
		"src/nuget.config": `<configuration><packageSources><add key="corp" value="https://nuget.corp/v3/index.json"/></packageSources></configuration>`,
	})
	c = d.Discover(repository)
	// The root file clears the machine's sources and nuget.org; the deeper file,
	// closer still, names corp again - with this machine's URL, but from the
	// repository, so it is the repository's source now.
	if got := order(c, NuGet, "Acme.Tools", ""); !slices.Equal(got, []string{"https://repo.feed/v3/index.json?", "https://nuget.corp/v3/index.json?"}) {
		t.Errorf("repository <clear/>: %v", got)
	}
	if c.off[NuGet] != OriginProject {
		t.Errorf("nuget.org switched off by %q, want the repository", c.off[NuGet])
	}
	c = d.Discover(nil)
	if got := order(c, NuGet, "Acme.Tools", ""); !slices.Equal(got, []string{"https://vendor.example/v3/index.json", "https://nuget.corp/v3/index.json", org}) {
		t.Errorf("after the repository's files went: %v", got)
	}

	// On Windows the machine-wide files are under %ProgramFiles(x86)%.
	programFiles := t.TempDir()
	put(t, filepath.Join(programFiles, "NuGet", "Config", "Microsoft.VisualStudio.Offline.config"),
		`<configuration><packageSources><add key="offline" value="https://offline.example/v3/index.json"/></packageSources></configuration>`)
	w := discoverOn(t.TempDir(), "windows", map[string]string{"ProgramFiles(x86)": programFiles})
	if got := order(w, NuGet, "Acme.Tools", ""); !slices.Equal(got, []string{"https://offline.example/v3/index.json", org}) {
		t.Errorf("Windows machine-wide: %v", got)
	}
}

// A NuGetPackageSourceCredentials_<source> variable serves the source of that name
// in this machine's configuration - found without regard to case - and nothing
// else; VSS_NUGET_EXTERNAL_FEED_ENDPOINTS serves its endpoint.
//
// Verifies: REQ-AUTH-022
func TestNuGetEnvironmentCredentials(t *testing.T) {
	corp := newNuGetStub(t, "ci", "s3cr3t", map[string]string{"Acme.Tools": "Acme.Core"})
	other := newNuGetStub(t, "", "", map[string]string{"Acme.Other": "Acme.Core"})
	vss := newNuGetStub(t, "vsts", "pat-123", map[string]string{"Acme.Vss": "Acme.Core"})
	home := t.TempDir()
	userNuGet(t, home, `<packageSources><add key="Corp Feed" value="`+corp.index()+`"/><add key="other" value="`+other.index()+`"/>`+
		`<add key="vss" value="`+vss.index()+`"/></packageSources>`)
	e := nugetEnvironment(t, map[string]string{
		"NUGETPACKAGESOURCECREDENTIALS_corp feed": "Username=ci;Password=s3cr3t;ValidAuthenticationTypes=basic",
		"VSS_NUGET_EXTERNAL_FEED_ENDPOINTS":       `{"endpointCredentials":[{"endpoint":"` + vss.index() + `","username":"vsts","password":"pat-123"}]}`,
	})
	store := auth.Read(home, e)
	d := NewDiscoverer(e, home)
	d.Config().Credentials(store)
	c := NewClient(d.Config(), t.TempDir(), time.Hour, 5*time.Second, store)
	d.Discover(nil)
	for packageName, feed := range map[string]*nugetStub{"Acme.Tools": corp, "Acme.Vss": vss} {
		if got, l := ask(t, c, lang.Target{Ecosystem: NuGet, Package: packageName, Version: "1.0.0"}); !slices.Equal(got, []string{"Acme.Core"}) || l.Index != feed.index() {
			t.Errorf("%s: %v from %s", packageName, got, l.Index)
		}
		if _, refused := feed.requests(); refused != 0 {
			t.Errorf("%s: %d requests went without the credential", packageName, refused)
		}
	}
	// The variable is for "Corp Feed" (on its host and port): the other feed gets
	// nothing.
	request, _ := http.NewRequest(http.MethodGet, other.index(), nil)
	store.Apply(request)
	if h := request.Header.Get("Authorization"); h != "" {
		t.Errorf("another source got %q", h)
	}
}

// A repository's feed gets a credential only when this machine supplies the secret
// - a %NAME% password in paket.dependencies or nuget.config, or a
// NuGetPackageSourceCredentials_ variable - and the feed is one this machine
// vouches for: on the host of a source this machine configures, or named with
// --trust-index. A password the repository writes out is never sent.
//
// Verifies: REQ-AUTH-023, REQ-AUTH-012
func TestRepositoryFeedCredentialsFromTheEnvironment(t *testing.T) {
	variables := map[string]string{
		"FEED_PAT":                               "from-env",
		"NuGetPackageSourceCredentials_repofeed": "Username=ci;Password=env-var",
	}
	paket := "source https://paket.corp/v3/index.json username: \"ci\" password: \"%FEED_PAT%\"\n" +
		"source https://literal.corp/v3/index.json username: \"ci\" password: \"written-out\"\n" +
		"source https://ntlm.corp/v3/index.json username: \"ci\" password: \"%FEED_PAT%\" authtype: \"ntlm\"\n" +
		"nuget Acme.Tools\n"
	nugetConfig := `<configuration><packageSources><add key="repofeed" value="https://repo.corp/v3/index.json"/>` +
		`<add key="refd" value="https://refd.corp/v3/index.json"/><add key="plain" value="https://plain.corp/v3/index.json"/></packageSources>
  <packageSourceCredentials>
    <refd><add key="Username" value="ci"/><add key="ClearTextPassword" value="%FEED_PAT%"/></refd>
    <plain><add key="Username" value="ci"/><add key="ClearTextPassword" value="written-out"/></plain>
  </packageSourceCredentials></configuration>`
	files := write(t, map[string]string{"paket.dependencies": paket, "nuget.config": nugetConfig})
	run := func(home string, trust ...string) *auth.Store {
		e := nugetEnvironment(t, variables)
		store := auth.Read(home, e)
		d := NewDiscoverer(e, home)
		d.Config().Credentials(store)
		d.Config().Trust(trust)
		d.Discover(files)
		return store
	}

	// Nobody vouches: nothing is lent.
	s := run(t.TempDir())
	for _, u := range []string{"https://paket.corp/x", "https://repo.corp/x", "https://refd.corp/x"} {
		if got := basicOf(s, u); got != "" {
			t.Errorf("unvouched %s got %q", u, got)
		}
	}

	// Vouched with --trust-index (URL or host): the environment's secrets are lent.
	s = run(t.TempDir(), "https://paket.corp/v3/index.json", "repo.corp", "refd.corp",
		"https://literal.corp/v3/index.json", "plain.corp", "ntlm.corp")
	for u, want := range map[string]string{
		"https://paket.corp/x":   "ci:from-env",
		"https://repo.corp/x":    "ci:env-var",
		"https://refd.corp/x":    "ci:from-env",
		"https://literal.corp/x": "",
		"https://plain.corp/x":   "",
		"https://ntlm.corp/x":    "",
	} {
		if got := basicOf(s, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}

	// On the host of a source this machine configures (without a credential of
	// its own), the Paket password is lent; a machine credential is kept.
	home := t.TempDir()
	userNuGet(t, home, `<packageSources><add key="corp" value="https://paket.corp/other/index.json"/>`+
		`<add key="repo" value="https://repo.corp/v3/index.json"/></packageSources>
  <packageSourceCredentials><repo><add key="Username" value="me"/><add key="ClearTextPassword" value="mine"/></repo></packageSourceCredentials>`)
	s = run(home)
	if got := basicOf(s, "https://paket.corp/x"); got != "ci:from-env" {
		t.Errorf("machine-configured host: %q", got)
	}
	if got := basicOf(s, "https://repo.corp/x"); got != "me:mine" {
		t.Errorf("the machine's own credential was replaced: %q", got)
	}
}

// A vouched repository feed is asked with the credential the environment lends it,
// end to end.
//
// Verifies: REQ-AUTH-023
func TestPaketFeedWithEnvironmentPassword(t *testing.T) {
	feed := newNuGetStub(t, "ci", "from-env", map[string]string{"Acme.Tools": "Acme.Core"})
	e := nugetEnvironment(t, map[string]string{"FEED_PAT": "from-env"})
	home := t.TempDir()
	store := auth.Read(home, e)
	d := NewDiscoverer(e, home)
	d.Config().Credentials(store)
	d.Config().Trust([]string{feed.index()})
	c := NewClient(d.Config(), t.TempDir(), time.Hour, 5*time.Second, store)
	d.Discover(write(t, map[string]string{"paket.dependencies": "source " + feed.index() + " username: \"ci\" password: \"%FEED_PAT%\"\nnuget Acme.Tools\n"}))
	if got, l := ask(t, c, lang.Target{Ecosystem: NuGet, Package: "Acme.Tools", Version: "1.0.0"}); !slices.Equal(got, []string{"Acme.Core"}) || l.Index != feed.index() {
		t.Errorf("Acme.Tools: %v from %s", got, l.Index)
	}
	if _, refused := feed.requests(); refused != 0 {
		t.Errorf("%d requests went without the credential", refused)
	}
}
