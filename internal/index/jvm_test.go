package index

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

const central = "https://repo.maven.apache.org/maven2"

// machineMaven lists the Maven sources the machine's configuration yields, each with
// "?" when it is not trusted and its kind.
func machineMaven(c *Config) []string {
	var out []string
	for _, s := range c.Sources(Maven) {
		u := s.URL
		if !s.Trusted {
			u += "?"
		}
		out = append(out, fmt.Sprintf("%s %d", u, s.Kind))
	}
	return out
}

// The repositories of the active profiles of settings.xml are asked beside Central:
// those <activeProfiles> lists, else those active by default. An inactive profile,
// a profile activated by a property, <pluginRepositories> and a repository a mirror
// serves are not; a profile repository with the id central replaces Central. The
// installation's settings (MAVEN_HOME) are merged under the user's.
//
// Verifies: REQ-SUP-015, REQ-SUP-064
func TestMavenSettingsActiveProfileRepositories(t *testing.T) {
	home, maven := t.TempDir(), t.TempDir()
	put(t, filepath.Join(home, ".m2", "settings.xml"), `<settings>
  <mirrors><mirror><id>m</id><url>https://mirror.corp/snapshots</url><mirrorOf>snapshots</mirrorOf></mirror></mirrors>
  <profiles>
    <profile><id>corp</id>
      <repositories>
        <repository><id>corp</id><url>https://corp.example/maven</url></repository>
        <repository><id>snapshots</id><url>https://snapshots.example/maven</url></repository>
        <repository><id>c</id><url>https://repo1.maven.org/maven2</url></repository>
      </repositories>
      <pluginRepositories><pluginRepository><id>p</id><url>https://plugins.example/maven</url></pluginRepository></pluginRepositories>
    </profile>
    <profile><id>idle</id><repositories><repository><id>idle</id><url>https://idle.example/maven</url></repository></repositories></profile>
    <profile><id>default</id><activation><activeByDefault>true</activeByDefault></activation>
      <repositories><repository><id>d</id><url>https://default.example/maven</url></repository></repositories></profile>
    <profile><id>prop</id><activation><property><name>env</name></property></activation>
      <repositories><repository><id>p</id><url>https://property.example/maven</url></repository></repositories></profile>
  </profiles>
</settings>`)
	put(t, filepath.Join(maven, "conf", "settings.xml"), `<settings>
  <profiles><profile><id>global</id><repositories>
    <repository><id>central</id><url>https://central.corp/maven2</url></repository>
  </repositories></profile></profiles>
  <activeProfiles><activeProfile>corp</activeProfile><activeProfile>global</activeProfile><activeProfile>missing</activeProfile></activeProfiles>
</settings>`)

	c := discoverOn(home, "linux", map[string]string{"MAVEN_HOME": maven})
	want := []string{"https://mirror.corp/snapshots 2", "https://corp.example/maven 2", "https://central.corp/maven2 0"}
	if got := machineMaven(c); !slices.Equal(got, want) {
		t.Errorf("listed profiles: %v, want %v", got, want)
	}
	if got := order(c, Maven, "g:a", ""); !slices.Equal(got, []string{"https://mirror.corp/snapshots", "https://corp.example/maven", "https://central.corp/maven2"}) {
		t.Errorf("asked %v", got)
	}

	// Without the installation's <activeProfiles>, the profile active by default is.
	c = discoverOn(home, "linux", nil)
	want = []string{"https://mirror.corp/snapshots 2", "https://default.example/maven 2"}
	if got := machineMaven(c); !slices.Equal(got, want) {
		t.Errorf("by default: %v, want %v", got, want)
	}
	if got := order(c, Maven, "g:a", ""); got[len(got)-1] != central {
		t.Errorf("Central is not asked last: %v", got)
	}
}

// basicFeed is a Maven repository that answers only with the credential given.
func basicFeed(t *testing.T, user, pass string, bodies map[string]string) (*httptest.Server, *int32) {
	t.Helper()
	var refused int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != user || p != pass {
			atomic.AddInt32(&refused, 1)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &refused
}

// The repository of an active profile is asked, with the <server> of the same id's
// credential; an artifact it lacks is still found on Central.
//
// Verifies: REQ-SUP-015, REQ-AUTH-003
func TestMavenProfileRepositoryWithServerCredentials(t *testing.T) {
	pub := newFeed(t, map[string]string{"/com/google/guava/guava/33.0/guava-33.0.pom": fmt.Sprintf(pomBody, "failureaccess")})
	asPublic(t, Maven, pub)
	nexus, refused := basicFeed(t, "deploy", "s3cr3t", map[string]string{
		"/repository/corp/com/acme/billing/1.0/billing-1.0.pom": fmt.Sprintf(pomBody, "acme-core"),
	})
	home := t.TempDir()
	put(t, filepath.Join(home, ".m2", "settings.xml"), `<settings>
  <servers><server><id>corp</id><username>deploy</username><password>s3cr3t</password></server></servers>
  <profiles>
    <profile><id>corp</id><repositories><repository><id>corp</id><url>`+nexus.URL+`/repository/corp</url></repository></repositories></profile>
  </profiles>
  <activeProfiles><activeProfile>corp</activeProfile></activeProfiles>
</settings>`)
	d := NewDiscoverer(env(nil), home)
	c := NewClient(d.Config(), t.TempDir(), time.Hour, 5*time.Second, auth.Read(home, env(nil)), nil)
	d.Discover(nil)
	got, l := ask(t, c, lang.Target{Ecosystem: Maven, Package: "com.acme:billing", Version: "1.0"})
	if !slices.Equal(got, []string{"g:acme-core"}) || l.Index != nexus.URL+"/repository/corp" {
		t.Errorf("billing: %v from %s, want acme-core from the profile repository", got, l.Index)
	}
	if n := atomic.LoadInt32(refused); n != 0 {
		t.Errorf("%d requests went without the server's credential", n)
	}
	if got, l := ask(t, c, lang.Target{Ecosystem: Maven, Package: "com.google.guava:guava", Version: "33.0"}); len(got) != 1 || l.Index != pub.URL {
		t.Errorf("guava: %v from %s, want Central after the profile repository", got, l.Index)
	}

	// Inactive, the profile's repository is not asked at all.
	put(t, filepath.Join(home, ".m2", "settings.xml"), `<settings><profiles>
    <profile><id>corp</id><repositories><repository><id>corp</id><url>`+nexus.URL+`/repository/corp</url></repository></repositories></profile>
  </profiles></settings>`)
	if got := order(Discover(nil, env(nil), home), Maven, "com.acme:billing", ""); !slices.Equal(got, []string{pub.URL}) {
		t.Errorf("inactive profile: asked %v", got)
	}
}

// An sbt build's resolvers are the repository's, asked beside Central and never
// fetched from; project/*.sbt (sbt's plugins), publishTo and Ivy-patterned
// resolvers are not read.
//
// Verifies: REQ-SUP-015, REQ-SUP-063
func TestSbtResolvers(t *testing.T) {
	files := write(t, map[string]string{
		"build.sbt": `ThisBuild / scalaVersion := "3.3.1"
resolvers += "corp releases" at "https://nexus.corp/releases"
resolvers ++= Seq(
  "snapshots" at "https://nexus.corp/snapshots",
  Resolver.url("plain", url("https://plain.corp/maven")),
  Resolver.url("ivy", url("https://ivy.corp/"))(Resolver.ivyStylePatterns),
  MavenRepository("mr", "https://mr.corp/maven"),
  "central" at "https://repo1.maven.org/maven2/"
)
publishTo := Some("publish" at "https://publish.corp/releases")
lazy val core = project.settings(resolvers += "inner" at "https://inner.corp/maven", publishTo := Some("p" at "https://p.corp/"))
`,
		"project/plugins.sbt": `resolvers += "plugins" at "https://plugins.corp/maven"`,
	})
	c := Discover(files, env(nil), "")
	want := []string{
		"https://nexus.corp/releases? 2", "https://nexus.corp/snapshots? 2", "https://plain.corp/maven? 2",
		"https://mr.corp/maven? 2", "https://inner.corp/maven? 2",
	}
	if got := machineMaven(c); !slices.Equal(got, want) {
		t.Errorf("sources %v, want %v", got, want)
	}
}

// ~/.sbt/repositories is this machine's: each Maven entry is asked beside Central;
// with -Dsbt.override.build.repos=true its list is all that is asked, in order, and
// Central only because the list names maven-central.
//
// Verifies: REQ-SUP-015, REQ-SUP-063, REQ-SUP-064
func TestSbtRepositoriesFile(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".sbt", "repositories"), `[scala]
  version: 2.13.12
[repositories]
  local
  # a comment
  corp-maven: https://nexus.corp/maven-public/
  corp-ivy: https://nexus.corp/ivy/, [organization]/[module]/[revision]/[type]s/[artifact].[ext]
  corp-compat: https://nexus.corp/compat/, [organization]/[module]/[revision]/[artifact].[ext], mavenCompatible
  boot: https://boot.corp/maven/, bootOnly
  maven-central
`)
	c := discoverOn(home, "linux", nil)
	if got, want := machineMaven(c), []string{"https://nexus.corp/maven-public 2", "https://nexus.corp/compat 2"}; !slices.Equal(got, want) {
		t.Errorf("sources %v, want %v", got, want)
	}

	// The override, from SBT_OPTS, and the file moved by -Dsbt.repository.config.
	moved := filepath.Join(t.TempDir(), "repos")
	put(t, moved, "[repositories]\n  local\n  corp: https://proxy.corp/maven\n  maven-central\n")
	c = discoverOn(home, "linux", map[string]string{
		"SBT_OPTS": "-Xmx2g -Dsbt.override.build.repos=true -Dsbt.repository.config=" + moved,
	})
	if got, want := order(c, Maven, "g:a", ""), []string{"https://proxy.corp/maven", central}; !slices.Equal(got, want) {
		t.Errorf("override: asked %v, want %v", got, want)
	}
	put(t, moved, "[repositories]\n  corp: https://proxy.corp/maven\n")
	c = discoverOn(home, "linux", map[string]string{"JAVA_OPTS": "-Dsbt.override.build.repos=true -Dsbt.repository.config=" + moved})
	if got, want := order(c, Maven, "g:a", ""), []string{"https://proxy.corp/maven"}; !slices.Equal(got, want) {
		t.Errorf("override without maven-central: asked %v, want %v", got, want)
	}
}

// COURSIER_REPOSITORIES replaces the default repositories with its list, in order:
// local and Ivy repositories are not read, and Central is asked only when listed.
//
// Verifies: REQ-SUP-015, REQ-SUP-063
func TestCoursierRepositories(t *testing.T) {
	c := discoverOn(t.TempDir(), "linux", map[string]string{
		"COURSIER_REPOSITORIES": "ivy2Local|https://nexus.corp/maven|central|ivy:https://ivy.corp/[organisation]/[module]|sonatype:releases|jitpack",
	})
	want := []string{"https://nexus.corp/maven", central, "https://oss.sonatype.org/content/repositories/releases", "https://jitpack.io"}
	if got := order(c, Maven, "g:a", ""); !slices.Equal(got, want) {
		t.Errorf("asked %v, want %v", got, want)
	}
	c = discoverOn(t.TempDir(), "linux", map[string]string{"COURSIER_REPOSITORIES": "https://nexus.corp/maven"})
	if got := order(c, Maven, "g:a", ""); !slices.Equal(got, []string{"https://nexus.corp/maven"}) {
		t.Errorf("without central: asked %v", got)
	}
}

// The init scripts of Gradle's user home (GRADLE_USER_HOME) name this machine's
// repositories, asked beside Central; those serving Gradle's plugins are not read.
//
// Verifies: REQ-SUP-015, REQ-SUP-064
func TestGradleInitScripts(t *testing.T) {
	home, gradle := t.TempDir(), t.TempDir()
	put(t, filepath.Join(home, ".gradle", "init.d", "home.gradle"), `allprojects { repositories { maven { url "https://home.corp/maven" } } }`)
	put(t, filepath.Join(gradle, "init.gradle.kts"), `allprojects { repositories { maven("https://first.corp/maven") } }`)
	put(t, filepath.Join(gradle, "init.d", "b.gradle"), `settingsEvaluated { settings -> settings.pluginManagement { repositories { maven { url "https://plugins.corp/" } } } }
allprojects {
    repositories {
        mavenCentral()
        maven {
            name = "corp"
            url = uri("https://nexus.corp/repository/maven")
            credentials(PasswordCredentials)
        }
    }
}`)
	put(t, filepath.Join(gradle, "init.d", "a.gradle.kts"), `allprojects { repositories { maven { url = uri("https://a.corp/maven") } } }`)
	put(t, filepath.Join(gradle, "init.d", "notes.txt"), `maven { url "https://notes.corp/" }`)
	c := discoverOn(home, "linux", map[string]string{"GRADLE_USER_HOME": gradle})
	want := []string{"https://first.corp/maven 2", "https://a.corp/maven 2", "https://nexus.corp/repository/maven 2"}
	if got := machineMaven(c); !slices.Equal(got, want) {
		t.Errorf("sources %v, want %v", got, want)
	}
	if got := machineMaven(discoverOn(home, "linux", nil)); !slices.Equal(got, []string{"https://home.corp/maven 2"}) {
		t.Errorf("~/.gradle: %v", got)
	}
}

// The Clojure CLI's user deps.edn (CLJ_CONFIG, $XDG_CONFIG_HOME/clojure,
// ~/.clojure) and the :user profile of Leiningen's profiles.clj (LEIN_HOME,
// ~/.lein) are this machine's repositories; a "central" entry replaces Central.
//
// Verifies: REQ-SUP-015, REQ-SUP-064
func TestClojureUserRepositories(t *testing.T) {
	home, xdg, clj := t.TempDir(), t.TempDir(), t.TempDir()
	put(t, filepath.Join(home, ".clojure", "deps.edn"), `{:mvn/repos {"home" {:url "https://home.corp/maven"}}}`)
	put(t, filepath.Join(xdg, "clojure", "deps.edn"), `{:mvn/repos {"xdg" {:url "https://xdg.corp/maven"}}}`)
	put(t, filepath.Join(clj, "deps.edn"), `{:mvn/repos {"central" {:url "https://central.corp/maven2"} "clojars" {:url "https://repo.clojars.org/"} "corp" {:url "https://nexus.corp/maven"}}}`)
	put(t, filepath.Join(home, ".lein", "profiles.clj"), `{:user {:plugins [[lein-ancient "1.0.0"]]
        :repositories [["lein-corp" {:url "https://lein.corp/maven"}] ["plain" "https://plain.corp/maven"]]}
 :other {:repositories [["other" "https://other.corp/maven"]]}}`)

	for _, tc := range []struct {
		vars map[string]string
		want []string
	}{
		{nil, []string{"https://home.corp/maven 2", "https://lein.corp/maven 2", "https://plain.corp/maven 2"}},
		{map[string]string{"XDG_CONFIG_HOME": xdg}, []string{"https://xdg.corp/maven 2", "https://lein.corp/maven 2", "https://plain.corp/maven 2"}},
		{map[string]string{"CLJ_CONFIG": clj, "XDG_CONFIG_HOME": xdg, "LEIN_HOME": t.TempDir()},
			[]string{"https://central.corp/maven2 0", "https://nexus.corp/maven 2"}},
	} {
		c := discoverOn(home, "linux", tc.vars)
		if got := machineMaven(c); !slices.Equal(got, tc.want) {
			t.Errorf("%v: %v, want %v", tc.vars, got, tc.want)
		}
		for _, u := range order(c, Maven, "g:a", "") {
			if strings.HasSuffix(u, "?") {
				t.Errorf("%v: %s is not trusted", tc.vars, u)
			}
		}
	}
}
