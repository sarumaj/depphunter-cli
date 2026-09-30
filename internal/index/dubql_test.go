package index

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// dub asks DUB_REGISTRY's registries, then the registryUrls of its settings files
// (the user's before the system's), then code.dlang.org; only HTTP registries are
// recorded, all of them this machine's. skipRegistry switches the public index off
// (standard), the settings' registries too (configured) or everything (all).
//
// Verifies: REQ-SUP-060, REQ-SUP-064
func TestDiscoverDubRegistries(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, ".dub", "settings.json"),
		`{"registryUrls": ["https://dub.corp.test/", "file:///opt/packages", "mvn+https://maven.corp.test", "https://code.dlang.org/"]}`)
	system := filepath.Join(userconf.SystemRoot, "etc", "dub", "settings.json")
	put(t, system, `{"registryUrls": ["https://system.corp.test"], "skipRegistry": "none"}`)
	t.Cleanup(func() { os.RemoveAll(filepath.Join(userconf.SystemRoot, "etc", "dub")) })
	variables := map[string]string{"DUB_REGISTRY": "https://env.corp.test;https://code.dlang.org"}
	if got, want := order(discoverOn(home, "linux", variables), Dub, "vibe-d", ""),
		[]string{"https://env.corp.test", "https://dub.corp.test", "https://system.corp.test", "https://code.dlang.org"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order: got %v, want %v", got, want)
	}
	// DUB_HOME, else DPATH/dub, moves the user's settings.
	moved := t.TempDir()
	put(t, filepath.Join(moved, "dub", "settings.json"), `{"registryUrls": ["https://moved.corp.test"]}`)
	for _, v := range []map[string]string{{"DUB_HOME": filepath.Join(moved, "dub")}, {"DPATH": moved}} {
		if got, want := order(discoverOn(home, "linux", v), Dub, "vibe-d", ""),
			[]string{"https://moved.corp.test", "https://system.corp.test", "https://code.dlang.org"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %v, want %v", v, got, want)
		}
	}
	// The user's skipRegistry wins over the system's.
	for skip, want := range map[string][]string{
		"none":       {"https://env.corp.test", "https://dub.corp.test", "https://system.corp.test", "https://code.dlang.org"},
		"standard":   {"https://env.corp.test", "https://dub.corp.test", "https://system.corp.test"},
		"configured": {"https://env.corp.test"},
		"all":        nil,
	} {
		put(t, filepath.Join(home, ".dub", "settings.json"), `{"registryUrls": ["https://dub.corp.test"], "skipRegistry": "`+skip+`"}`)
		if got := order(discoverOn(home, "linux", variables), Dub, "vibe-d", ""); !reflect.DeepEqual(got, want) {
			t.Errorf("skipRegistry %s: got %v, want %v", skip, got, want)
		}
	}
	// A repository's dub.settings.json names registries of its own: untrusted, after
	// this machine's.
	put(t, filepath.Join(home, ".dub", "settings.json"), `{"registryUrls": ["https://dub.corp.test"]}`)
	c := discoverOn(home, "linux", nil)
	c.project(write(t, map[string]string{"dub.settings.json": `{"registryUrls": ["https://repo.corp.test/"]}`}))
	if got, want := order(c, Dub, "vibe-d", ""),
		[]string{"https://dub.corp.test", "https://system.corp.test", "https://repo.corp.test?", "https://code.dlang.org"}; !reflect.DeepEqual(got, want) {
		t.Errorf("repository: got %v, want %v", got, want)
	}
}

// On Windows dub's settings are %APPDATA%\dub's and %ProgramData%\dub's; without
// %APPDATA% the user's fall back to ~/.dub.
//
// Verifies: REQ-SUP-064
func TestDiscoverDubOnWindows(t *testing.T) {
	home, appData, programData := t.TempDir(), t.TempDir(), t.TempDir()
	put(t, filepath.Join(appData, "dub", "settings.json"), `{"registryUrls": ["https://user.corp.test"]}`)
	put(t, filepath.Join(programData, "dub", "settings.json"), `{"registryUrls": ["https://machine.corp.test"]}`)
	put(t, filepath.Join(home, ".dub", "settings.json"), `{"registryUrls": ["https://home.corp.test"]}`)
	got := order(discoverOn(home, "windows", map[string]string{"APPDATA": appData, "ProgramData": programData}), Dub, "vibe-d", "")
	if want := []string{"https://user.corp.test", "https://machine.corp.test", "https://code.dlang.org"}; !reflect.DeepEqual(got, want) {
		t.Errorf("windows: got %v, want %v", got, want)
	}
	got = order(discoverOn(home, "windows", nil), Dub, "vibe-d", "")
	if want := []string{"https://home.corp.test", "https://code.dlang.org"}; !reflect.DeepEqual(got, want) {
		t.Errorf("windows without APPDATA: got %v, want %v", got, want)
	}
}

// A registry this machine's dub settings name is asked before code.dlang.org, and
// answers without code.dlang.org being asked.
//
// Verifies: REQ-SUP-060
func TestDubMachineRegistryIsAsked(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.URL.Path != "/dub/api/packages/acme-log/1.0.0/info" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"version": "1.0.0", "name": "acme-log", "dependencies": {"acme-core": "~>2.0"}}`))
	}))
	defer server.Close()
	home := t.TempDir()
	put(t, filepath.Join(home, ".dub", "settings.json"), `{"registryUrls": ["`+server.URL+`/dub/"]}`)
	c := NewClient(discoverOn(home, "linux", nil), t.TempDir(), time.Hour, 5*time.Second, auth.Read("", nil))
	want := []lang.Target{{Ecosystem: Dub, Package: "acme-core", Version: "~>2.0"}}
	if got := c.Dependencies(lang.Target{Ecosystem: Dub, Package: "acme-log", Version: "1.0.0", Pinned: true}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if want := []string{"/dub/api/packages/acme-log/1.0.0/info"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %v, want %v", asked, want)
	}
}

// A qlfile's dist lines add dists beside the Quicklisp dist and its ultralisp lines
// name the Ultralisp dist as the project's, all of them the repository's
// (untrusted); the dists installed in ~/quicklisp are this machine's. The Quicklisp
// dist itself is never recorded.
//
// Verifies: REQ-SUP-062, REQ-SUP-064
func TestDiscoverQuicklispDists(t *testing.T) {
	home := t.TempDir()
	put(t, filepath.Join(home, "quicklisp", "dists", "quicklisp", "distinfo.txt"),
		"name: quicklisp\nversion: 2024-10-12\ndistinfo-subscription-url: http://beta.quicklisp.org/dist/quicklisp.txt\n")
	put(t, filepath.Join(home, "quicklisp", "dists", "corp", "distinfo.txt"),
		"name: corp\nversion: 20240101\ndistinfo-subscription-url: https://dist.corp.test/corp.txt\nsystem-index-url: https://dist.corp.test/corp/20240101/systems.txt\n")
	c := discoverOn(home, "linux", nil)
	c.project(write(t, map[string]string{"qlfile": `# dists
dist http://beta.quicklisp.org/dist/quicklisp.txt 2023-10-21
dist https://dist.shirakumo.org/shirakumo.txt
dist acme https://dist.acme.test/acme.txt  # named
ultralisp 40ants-doc
ultralisp :all
ql alexandria
git fork https://github.com/acme/fork.git
`}))
	if got, want := sources(c, Quicklisp), []string{
		"https://dist.corp.test/corp.txt", "https://dist.shirakumo.org/shirakumo.txt?", "https://dist.acme.test/acme.txt?",
		"40ants-doc https://dist.ultralisp.org?",
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("sources:\n got %v\nwant %v", got, want)
	}
	if got, want := order(c, Quicklisp, "cl-ppcre", ""), []string{
		"https://dist.corp.test/corp.txt", "https://dist.shirakumo.org/shirakumo.txt?", "https://dist.acme.test/acme.txt?",
		"https://beta.quicklisp.org/dist/quicklisp.txt",
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("order: got %v, want %v", got, want)
	}
	if got := order(c, Quicklisp, "40ants-doc", ""); !reflect.DeepEqual(got, []string{"https://dist.ultralisp.org?"}) {
		t.Errorf("ultralisp project: %v", got)
	}
}

// A dist that does not have a project says so and the next dist is asked; a dist
// whose distinfo cannot be read is asked once, not once per project, until the
// failure is old enough to try again.
//
// Verifies: REQ-SUP-062
func TestQuicklispDistFallbackAndFailureMemo(t *testing.T) {
	var corp, broken atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/corp.txt":
			corp.Add(1)
			w.Write([]byte("name: corp\nsystem-index-url: http://" + r.Host + "/corp/systems.txt\n"))
		case "/corp/systems.txt":
			w.Write([]byte("# project system-file system-name dependency1 ... dependencyN\nacme-log acme-log acme-log alexandria\n"))
		case "/quicklisp.txt":
			w.Write([]byte("name: quicklisp\nsystem-index-url: http://" + r.Host + "/ql/systems.txt\n"))
		case "/ql/systems.txt":
			w.Write([]byte("cl-ppcre cl-ppcre cl-ppcre\nalexandria alexandria alexandria\n"))
		case "/broken.txt":
			broken.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	config := New()
	config.Add(Quicklisp, Source{URL: server.URL + "/corp.txt", Kind: Additive, Trusted: true})
	config.Add(Quicklisp, Source{URL: server.URL + "/quicklisp.txt", Trusted: true})
	c := NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read("", nil))
	if got := c.Dependencies(lang.Target{Ecosystem: Quicklisp, Package: "acme-log"}); !reflect.DeepEqual(got, []lang.Target{{Ecosystem: Quicklisp, Package: "alexandria"}}) {
		t.Errorf("acme-log: %+v", got)
	}
	// Not in the corp dist: the Quicklisp dist answers.
	if got := c.Dependencies(lang.Target{Ecosystem: Quicklisp, Package: "cl-ppcre"}); got == nil || len(got) != 0 {
		t.Errorf("cl-ppcre: %+v, want an empty answer from the Quicklisp dist", got)
	}
	if index, _, ok := c.Located(Quicklisp, "cl-ppcre"); !ok || index != server.URL+"/quicklisp.txt" {
		t.Errorf("cl-ppcre located at %s", index)
	}
	if n := corp.Load(); n != 1 {
		t.Errorf("corp distinfo asked %d times, want once", n)
	}

	config = New()
	config.Add(Quicklisp, Source{URL: server.URL + "/broken.txt", Trusted: true})
	c = NewClient(config, t.TempDir(), time.Hour, 5*time.Second, auth.Read("", nil))
	for _, p := range []string{"a", "b", "c"} {
		if got := c.Dependencies(lang.Target{Ecosystem: Quicklisp, Package: p}); got != nil {
			t.Errorf("%s: %+v, want no answer", p, got)
		}
	}
	if n := broken.Load(); n != 1 {
		t.Errorf("broken distinfo asked %d times, want once", n)
	}
	// Once the failure is older than failRetry the dist is asked again.
	c.mu.Lock()
	f := c.qlFailed[server.URL+"/broken.txt"]
	f.at = f.at.Add(-failRetry)
	c.qlFailed[server.URL+"/broken.txt"] = f
	c.mu.Unlock()
	c.Dependencies(lang.Target{Ecosystem: Quicklisp, Package: "d"})
	if n := broken.Load(); n != 2 {
		t.Errorf("broken distinfo asked %d times after failRetry, want twice", n)
	}
}
