package auth

import (
	"path/filepath"
	"testing"
)

// The installation's settings.xml (MAVEN_HOME) is read under the user's: a server of
// either finds a repository of either, the user's defining an id first. A server
// named like a repository of the Clojure CLI's user deps.edn goes to that host, as
// tools.deps sends it.
//
// Verifies: REQ-AUTH-003, REQ-AUTH-020
func TestMavenSettingsFilesAndClojureRepositories(t *testing.T) {
	home, maven, clj := t.TempDir(), t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(home, ".m2", "settings.xml"), `<settings>
  <servers>
    <server><id>corp</id><username>user</username><password>mine</password></server>
    <server><id>clj</id><username>clj</username><password>pw</password></server>
  </servers>
  <profiles><profile><id>p</id><repositories><repository><id>corp</id><url>https://nexus.corp/maven</url></repository></repositories></profile></profiles>
</settings>`)
	writeFile(t, filepath.Join(maven, "conf", "settings.xml"), `<settings>
  <servers>
    <server><id>corp</id><username>shared</username><password>theirs</password></server>
    <server><id>global</id><username>ci</username><password>g</password></server>
  </servers>
  <mirrors><mirror><id>global</id><url>https://mirror.corp/maven</url><mirrorOf>central</mirrorOf></mirror></mirrors>
</settings>`)
	writeFile(t, filepath.Join(clj, "deps.edn"), `{:mvn/repos {"clj" {:url "https://clj.corp/maven"}}}`)
	c := onMachine(t, home, "linux", map[string]string{"MAVEN_HOME": maven, "CLJ_CONFIG": clj})
	for host, want := range map[string]string{
		"nexus.corp": "user:mine", "mirror.corp": "ci:g", "clj.corp": "clj:pw",
	} {
		if got := c.basic[host]; got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
}

// sbt's credential files, SBT_CREDENTIALS among them, give a credential per host.
//
// Verifies: REQ-AUTH-021
func TestSbtCredentials(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".sbt", ".credentials"), "realm=Sonatype Nexus Repository Manager\nhost=nexus.corp\nuser=sbt\npassword=s3cr3t\n")
	writeFile(t, filepath.Join(home, ".ivy2", ".credentials"), "# old\nrealm = Artifactory Realm\nhost = Artifactory.Corp\nuser = ivy\npassword = p:w\n")
	env := filepath.Join(t.TempDir(), "ci.credentials")
	writeFile(t, env, "host=ci.corp\nuser=ci\npassword=token\n")
	bad := filepath.Join(t.TempDir(), "bad.credentials")
	writeFile(t, bad, "host=https://url.corp/maven\nuser=u\npassword=p\n")
	c := onMachine(t, home, "linux", map[string]string{"SBT_CREDENTIALS": env})
	for host, want := range map[string]string{"nexus.corp": "sbt:s3cr3t", "artifactory.corp": "ivy:p:w", "ci.corp": "ci:token"} {
		if got := c.basic[host]; got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
	c = onMachine(t, t.TempDir(), "linux", map[string]string{"SBT_CREDENTIALS": bad})
	if len(c.basic) != 0 {
		t.Errorf("a URL is no host: %v", c.basic)
	}
}

// Coursier's credentials come from COURSIER_CREDENTIALS, inline or naming a file,
// else from credentials.properties in its configuration directory.
//
// Verifies: REQ-AUTH-021, REQ-AUTH-020
func TestCoursierCredentials(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(xdg, "coursier", "credentials.properties"), `corp.host=nexus.corp
corp.username=cs
corp.password=pw
corp.realm=Sonatype Nexus Repository Manager
other.host=other.corp
other.password=no-user
`)
	c := onMachine(t, home, "linux", map[string]string{"XDG_CONFIG_HOME": xdg})
	if got := c.basic["nexus.corp"]; got != "cs:pw" {
		t.Errorf("file: %q", got)
	}
	if _, ok := c.basic["other.corp"]; ok {
		t.Error("an entry without a user name was kept")
	}

	c = onMachine(t, home, "linux", map[string]string{
		"XDG_CONFIG_HOME":      xdg,
		"COURSIER_CREDENTIALS": "\n  artifacts.corp(tha realm) alex:my-pass\n  plain.corp bob:b:c\n",
	})
	for host, want := range map[string]string{"artifacts.corp": "alex:my-pass", "plain.corp": "bob:b:c", "nexus.corp": ""} {
		if got := c.basic[host]; got != want {
			t.Errorf("inline %s: %q, want %q", host, got, want)
		}
	}

	file := filepath.Join(t.TempDir(), "creds.properties")
	writeFile(t, file, "x.host=file.corp\nx.username=f\nx.password=p\n")
	for _, v := range []string{file, "file://" + filepath.ToSlash(file)} {
		c = onMachine(t, home, "linux", map[string]string{"COURSIER_CREDENTIALS": v})
		if got := c.basic["file.corp"]; got != "f:p" {
			t.Errorf("%s: %q", v, got)
		}
	}

	// macOS keeps the configuration under Application Support.
	writeFile(t, filepath.Join(home, "Library", "Application Support", "Coursier", "credentials.properties"), "m.host=mac.corp\nm.username=m\nm.password=p\n")
	if got := onMachine(t, home, "darwin", nil).basic["mac.corp"]; got != "m:p" {
		t.Errorf("darwin: %q", got)
	}
}

// A repository of an init script in Gradle's user home that asks for
// PasswordCredentials takes <name>Username and <name>Password from gradle.properties
// there, else from ORG_GRADLE_PROJECT_ variables.
//
// Verifies: REQ-AUTH-021
func TestGradleInitScriptCredentials(t *testing.T) {
	home, gradle := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(gradle, "init.d", "repos.gradle.kts"), `allprojects {
    repositories {
        maven {
            name = "corp"
            url = uri("https://nexus.corp/repository/maven")
            credentials(PasswordCredentials::class)
        }
        maven { name "ci"; url "https://ci.corp/maven"; credentials(PasswordCredentials) }
        maven { name = "open"; url = uri("https://open.corp/maven") }
    }
}`)
	writeFile(t, filepath.Join(gradle, "gradle.properties"), "corpUsername=gradle\ncorpPassword=s3cr3t\nopenUsername=o\nopenPassword=p\n")
	c := onMachine(t, home, "linux", map[string]string{
		"GRADLE_USER_HOME":                  gradle,
		"ORG_GRADLE_PROJECT_ciUsername":     "ci",
		"ORG_GRADLE_PROJECT_ciPassword":     "token",
		"ORG_GRADLE_PROJECT_corpPassword":   "not-this",
		"ORG_GRADLE_PROJECT_corpUsername":   "nor-this",
		"ORG_GRADLE_PROJECT_unusedUsername": "x",
	})
	for host, want := range map[string]string{"nexus.corp": "gradle:s3cr3t", "ci.corp": "ci:token", "open.corp": ""} {
		if got := c.basic[host]; got != want {
			t.Errorf("%s: %q, want %q", host, got, want)
		}
	}
}
