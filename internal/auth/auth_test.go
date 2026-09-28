package auth

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// Verifies: REQ-AUTH-003, REQ-AUTH-009, REQ-AUTH-010
func TestMavenServerCredentialsFindTheirHost(t *testing.T) {
	// cSpell: words COQLCE
	t.Setenv("NEXUS_PASSWORD", "from-the-environment")
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readMavenSettings([][]byte{[]byte(`<?xml version="1.0"?>
<settings>
  <servers>
    <server><id>nexus</id><username>build</username><password>${env.NEXUS_PASSWORD}</password></server>
    <server><id>internal</id><username>dev</username><password>s3cRet</password></server>
    <server><id>encrypted</id><username>dev</username><password>{aGVsbG8=}</password></server>
    <server><id>sealed</id><username>dev</username><password>Rotated 2026-01 {COQLCE6DU6GtcS5P=}</password></server>
    <server><id>orphan</id><username>dev</username><password>x</password></server>
  </servers>
  <mirrors>
    <mirror><id>nexus</id><url>https://nexus.corp/repository/maven-group</url></mirror>
    <mirror><id>encrypted</id><url>https://vault.corp/maven</url></mirror>
    <mirror><id>sealed</id><url>https://sealed.corp/maven</url></mirror>
  </mirrors>
  <profiles>
    <profile><repositories>
      <repository><id>internal</id><url>https://artifactory.corp/artifactory/libs</url></repository>
    </repositories></profile>
  </profiles>
</settings>`)}, nil)

	// A mirror and a profile repository both name a host a server can belong to.
	if got := c.basic["nexus.corp"]; got != "build:from-the-environment" {
		t.Errorf("nexus.corp: %q - the environment reference was not resolved", got)
	}
	if got := c.basic["artifactory.corp"]; got != "dev:s3cRet" {
		t.Errorf("artifactory.corp: %q", got)
	}
	// An encrypted password needs the master password to be of any use, even when a
	// mirror names its server, and a server nothing points at has no host to be sent
	// to. Neither is a credential.
	for _, host := range []string{"vault.corp", "sealed.corp"} {
		if got, ok := c.basic[host]; ok {
			t.Errorf("%s: sent the encrypted password %q", host, got)
		}
	}
	if len(c.basic) != 2 {
		t.Errorf("kept %v", c.basic)
	}
}

// Verifies: REQ-AUTH-004, REQ-AUTH-009, REQ-AUTH-010
func TestNuGetFeedCredentialsFindTheirHost(t *testing.T) {
	t.Setenv("AZ_PAT", "a-token")
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readNuGetConfigs([][]byte{[]byte(`<?xml version="1.0"?>
<configuration>
  <packageSources>
    <add key="Company Feed" value="https://pkgs.dev.azure.com/acme/_packaging/feed/nuget/v3/index.json" />
    <add key="proget" value="https://proget.corp/nuget/Default/v3/index.json" />
    <add key="nuget.org" value="https://api.nuget.org/v3/index.json" />
  </packageSources>
  <packageSourceCredentials>
    <Company_x0020_Feed>
      <add key="Username" value="build" />
      <add key="ClearTextPassword" value="%AZ_PAT%" />
    </Company_x0020_Feed>
    <proget>
      <add key="Username" value="dev" />
      <add key="Password" value="encrypted-and-useless-here" />
    </proget>
  </packageSourceCredentials>
</configuration>`)}, nil)

	// A source key with a space is escaped into the element name; the value may come
	// from the environment. On pkgs.dev.azure.com, which every Azure DevOps
	// organization shares, it serves the organization's path only.
	if got := authorization(t, c, "https://pkgs.dev.azure.com/acme/_packaging/0f3a/nuget/v3/flat2/x/index.json"); got != basicHeader("build:a-token") {
		t.Errorf("pkgs.dev.azure.com/acme: %q", got)
	}
	if got := authorization(t, c, "https://pkgs.dev.azure.com/other/_packaging/feed/nuget/v3/index.json"); got != "" {
		t.Errorf("another organization got acme's credential: %q", got)
	}
	// A Windows-encrypted password cannot be decrypted here, so it is left alone
	// rather than sent as itself.
	if _, ok := c.basic["proget.corp"]; ok {
		t.Errorf("kept an encrypted password: %v", c.basic)
	}
}

// Verifies: REQ-AUTH-011
func TestCredentialsGoOnlyToTheHostTheyWereWrittenFor(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{"nexus.corp": "u:p"}}
	ours, _ := http.NewRequest(http.MethodGet, "https://nexus.corp/repository/x", nil)
	c.Apply(ours)
	if ours.Header.Get("Authorization") == "" {
		t.Error("the host it was written for got nothing")
	}
	theirs, _ := http.NewRequest(http.MethodGet, "https://registry.npmjs.org/react", nil)
	c.Apply(theirs)
	if got := theirs.Header.Get("Authorization"); got != "" {
		t.Errorf("a company password was sent to the public registry: %q", got)
	}
}

// A netrc is what git and curl read, and therefore what a Go proxy, a pip mirror and
// a link into a private repository are most often reached with.
//
// Verifies: REQ-AUTH-002
func TestNetrcCredentials(t *testing.T) {
	home := t.TempDir()
	netrc := "machine index.internal login user password pass\nmachine other.internal login u2 password p2\n"
	if err := os.WriteFile(filepath.Join(home, ".netrc"), []byte(netrc), 0o600); err != nil {
		t.Fatal(err)
	}
	c := Read(home, nil)
	request, _ := http.NewRequest(http.MethodGet, "https://index.internal/simple/requests/", nil)
	c.Apply(request)
	// cSpell: disable-next-line
	if got := request.Header.Get("Authorization"); got != "Basic dXNlcjpwYXNz" {
		t.Errorf("got %q", got)
	}
	other, _ := http.NewRequest(http.MethodGet, "https://elsewhere.internal/x", nil)
	c.Apply(other)
	if got := other.Header.Get("Authorization"); got != "" {
		t.Errorf("credentials were sent to another host: %q", got)
	}
}

func TestNoCredentialsInTheClear(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{"nexus.corp": "u:p", "localhost": "l:p"}}
	// An address from the repository - a README's link - asking for plain http.
	link, _ := http.NewRequest(http.MethodGet, "http://nexus.corp/repository/x", nil)
	c.Apply(link)
	if got := link.Header.Get("Authorization"); got != "" {
		t.Errorf("a password was sent over plain http: %q", got)
	}
	// A registry on this machine is reached over http and nothing leaves it.
	local, _ := http.NewRequest(http.MethodGet, "http://localhost:4873/react", nil)
	c.Apply(local)
	if local.Header.Get("Authorization") == "" {
		t.Error("the local registry got nothing")
	}

	// A company feed this machine's own configuration names over http keeps working.
	c.readMavenSettings([][]byte{[]byte(`<settings>
  <servers><server><id>old</id><username>u</username><password>p</password></server></servers>
  <mirrors><mirror><id>old</id><url>http://legacy.corp/maven</url></mirror></mirrors>
</settings>`)}, nil)
	legacy, _ := http.NewRequest(http.MethodGet, "http://legacy.corp/maven/x.pom", nil)
	c.Apply(legacy)
	if legacy.Header.Get("Authorization") == "" {
		t.Error("a feed configured over http got nothing")
	}
}

// Index discovery files URL credentials on every --watch analysis while the link
// checker of the one before may still be sending requests; run with -race.
func TestFromURLWhileApplying(t *testing.T) {
	c := Read("", nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 200 {
			c.FromURL(fmt.Sprintf("https://u:p@mirror%d.corp/simple", i), true)
		}
	}()
	for range 200 {
		request, _ := http.NewRequest(http.MethodGet, "https://mirror1.corp/x", nil)
		c.Apply(request)
	}
	<-done
}

// NuGet's files merge like NuGet merges them, so a user file's credential serves a
// machine-wide file's source; a NuGetPackageSourceCredentials_ variable wins over
// the file and serves only the source of its name; a disabled source, a variable
// for a source no file names, and a credential limited to other than basic give
// nothing.
//
// Verifies: REQ-AUTH-004, REQ-AUTH-022
func TestNuGetCredentialLayersAndVariables(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readNuGetConfigs([][]byte{
		[]byte(`<configuration>
  <packageSourceCredentials>
    <vendor><add key="Username" value="u"/><add key="ClearTextPassword" value="file"/></vendor>
    <corp><add key="Username" value="u"/><add key="ClearTextPassword" value="file"/></corp>
    <ntlm><add key="Username" value="u"/><add key="ClearTextPassword" value="p"/><add key="ValidAuthenticationTypes" value="negotiate"/></ntlm>
  </packageSourceCredentials>
  <disabledPackageSources><add key="off" value="true"/></disabledPackageSources>
</configuration>`),
		[]byte(`<configuration><packageSources>
  <add key="vendor" value="https://vendor.example/v3/index.json"/><add key="corp" value="https://corp.example:8443/v3/index.json"/>
  <add key="off" value="https://off.example/v3/index.json"/><add key="ntlm" value="https://ntlm.example/v3/index.json"/>
</packageSources></configuration>`),
	}, map[string]string{
		"corp":    "Username=ci;Password=env",
		"off":     "Username=ci;Password=env",
		"unnamed": "Username=ci;Password=env",
	})
	for u, want := range map[string]string{
		"https://vendor.example/x":    basicHeader("u:file"),
		"https://corp.example:8443/x": basicHeader("ci:env"),
		"https://corp.example/x":      "",
		"https://off.example/x":       "",
		"https://ntlm.example/x":      "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
	if len(c.basic) != 2 {
		t.Errorf("filed %v", c.basic)
	}
}

// VSS_NUGET_EXTERNAL_FEED_ENDPOINTS gives each endpoint its password: on
// pkgs.dev.azure.com for the organization's path only, elsewhere for the host.
//
// Verifies: REQ-AUTH-022
func TestVSSExternalFeedEndpoints(t *testing.T) {
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readVSSEndpoints(`{"endpointCredentials":[
  {"endpoint":"https://pkgs.dev.azure.com/acme/_packaging/feed/nuget/v3/index.json","password":"pat-1"},
  {"endpoint":"https://pkgs.dev.azure.com/beta/proj/_packaging/f/nuget/v3/index.json","username":"b","password":"pat-2"},
  {"endpoint":"https://nexus.corp/repository/nuget/index.json","username":"n","password":"pat-3"},
  {"endpoint":"https://nopass.corp/index.json","username":"n"}]}`)
	for u, want := range map[string]string{
		"https://pkgs.dev.azure.com/acme/_packaging/0f3a/nuget/v3/flat2/x/index.json": basicHeader(":pat-1"),
		"https://pkgs.dev.azure.com/beta/0b1c/_packaging/f/nuget/v3/flat2/":           basicHeader("b:pat-2"),
		"https://pkgs.dev.azure.com/evil/_packaging/feed/nuget/v3/index.json":         "",
		"https://pkgs.dev.azure.com/acmecorp/_packaging/feed/nuget/v3/index.json":     "",
		"https://nexus.corp/any/path":                                                 basicHeader("n:pat-3"),
		"https://nopass.corp/index.json":                                              "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
	c.readVSSEndpoints(`not json`)
	c.readVSSEndpoints(``)
}

// Lend gives a feed the credential only when its host has none of its own.
//
// Verifies: REQ-AUTH-023
func TestLendKeepsTheMachinesCredential(t *testing.T) {
	c := &Store{bearer: map[string]string{"token.corp": "t"}, basic: map[string]string{"mine.corp": "me:mine"}}
	c.Lend("https://mine.corp/v3/index.json", "ci", "lent")
	c.Lend("https://token.corp/v3/index.json", "ci", "lent")
	c.Lend("https://free.corp/v3/index.json", "ci", "lent")
	c.Lend("https://empty.corp/v3/index.json", "ci", "")
	c.Lend("https://pkgs.dev.azure.com/acme/_packaging/f/nuget/v3/index.json", "", "pat")
	for u, want := range map[string]string{
		"https://mine.corp/x":                     basicHeader("me:mine"),
		"https://token.corp/x":                    "Bearer t",
		"https://free.corp/x":                     basicHeader("ci:lent"),
		"https://empty.corp/x":                    "",
		"https://pkgs.dev.azure.com/acme/x":       basicHeader(":pat"),
		"https://pkgs.dev.azure.com/someone-else": "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
	var nilStore *Store
	nilStore.Lend("https://x.corp", "a", "b")
}
