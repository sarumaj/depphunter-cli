package index

import (
	"net/http"
	"testing"
)

func TestMavenServerCredentialsFindTheirHost(t *testing.T) {
	t.Setenv("NEXUS_PASSWORD", "from-the-environment")
	c := &credentials{bearer: map[string]string{}, basic: map[string]string{}}
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

func TestNuGetFeedCredentialsFindTheirHost(t *testing.T) {
	t.Setenv("AZ_PAT", "a-token")
	c := &credentials{bearer: map[string]string{}, basic: map[string]string{}}
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

func TestCredentialsGoOnlyToTheHostTheyWereWrittenFor(t *testing.T) {
	c := &credentials{bearer: map[string]string{}, basic: map[string]string{"nexus.corp": "u:p"}}
	ours, _ := http.NewRequest(http.MethodGet, "https://nexus.corp/repository/x", nil)
	c.apply(ours)
	if ours.Header.Get("Authorization") == "" {
		t.Error("the host it was written for got nothing")
	}
	theirs, _ := http.NewRequest(http.MethodGet, "https://registry.npmjs.org/react", nil)
	c.apply(theirs)
	if got := theirs.Header.Get("Authorization"); got != "" {
		t.Errorf("a company password was sent to the public registry: %q", got)
	}
}
