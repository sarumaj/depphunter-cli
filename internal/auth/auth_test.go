package auth

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// Verifies: REQ-AUTH-003, REQ-AUTH-009
func TestMavenServerCredentialsFindTheirHost(t *testing.T) {
	t.Setenv("NEXUS_PASSWORD", "from-the-environment")
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readMavenSettings([]byte(`<?xml version="1.0"?>
<settings>
  <servers>
    <server><id>nexus</id><username>build</username><password>${env.NEXUS_PASSWORD}</password></server>
    <server><id>internal</id><username>dev</username><password>s3cRet</password></server>
    <server><id>encrypted</id><username>dev</username><password>{aGVsbG8=}</password></server>
    <server><id>orphan</id><username>dev</username><password>x</password></server>
  </servers>
  <mirrors>
    <mirror><id>nexus</id><url>https://nexus.corp/repository/maven-group</url></mirror>
  </mirrors>
  <profiles>
    <profile><repositories>
      <repository><id>internal</id><url>https://artifactory.corp/artifactory/libs</url></repository>
    </repositories></profile>
  </profiles>
</settings>`))

	// A mirror and a profile repository both name a host a server can belong to.
	if got := c.basic["nexus.corp"]; got != "build:from-the-environment" {
		t.Errorf("nexus.corp: %q - the environment reference was not resolved", got)
	}
	if got := c.basic["artifactory.corp"]; got != "dev:s3cRet" {
		t.Errorf("artifactory.corp: %q", got)
	}
	// An encrypted password needs the master password to be of any use, and a server
	// nothing points at has no host to be sent to. Neither is a credential.
	if len(c.basic) != 2 {
		t.Errorf("kept %v", c.basic)
	}
}

// Verifies: REQ-AUTH-004, REQ-AUTH-009, REQ-AUTH-010
func TestNuGetFeedCredentialsFindTheirHost(t *testing.T) {
	t.Setenv("AZ_PAT", "a-token")
	c := &Store{bearer: map[string]string{}, basic: map[string]string{}}
	c.readNuGetConfig([]byte(`<?xml version="1.0"?>
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
</configuration>`))

	// A source key with a space is escaped into the element name; the value may come
	// from the environment.
	if got := c.basic["pkgs.dev.azure.com"]; got != "build:a-token" {
		t.Errorf("pkgs.dev.azure.com: %q", got)
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
	req, _ := http.NewRequest(http.MethodGet, "https://index.internal/simple/requests/", nil)
	c.Apply(req)
	// cSpell: disable-next-line
	if got := req.Header.Get("Authorization"); got != "Basic dXNlcjpwYXNz" {
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
	c.readMavenSettings([]byte(`<settings>
  <servers><server><id>old</id><username>u</username><password>p</password></server></servers>
  <mirrors><mirror><id>old</id><url>http://legacy.corp/maven</url></mirror></mirrors>
</settings>`))
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
		req, _ := http.NewRequest(http.MethodGet, "https://mirror1.corp/x", nil)
		c.Apply(req)
	}
	<-done
}
