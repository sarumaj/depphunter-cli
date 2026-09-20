package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testAssets() *assets {
	return newAssets(fstest.MapFS{
		"index.html": {Data: []byte("<!-- a note -->\n<p>hi</p>")},
		"app.js":     {Data: []byte("// a note\nexport const a = 1; /* and another */\n" + longComment())},
		"style.css":  {Data: []byte("/* a note */\n.x { color: red; }")},
		"hand.glb":   {Data: []byte("glTF binary bytes")},
	})
}

func longComment() string {
	s := ""
	for i := 0; i < 400; i++ {
		s += "// a line of documentation that is not worth sending to a browser\n"
	}
	return s
}

func fetch(t *testing.T, a *assets, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	for k, v := range header {
		r.Header[k] = v
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func TestAssetsMinifies(t *testing.T) {
	w := fetch(t, testAssets(), "/app.js", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.String()
	if want := "export const a = 1;"; !strings.Contains(body, want) {
		t.Errorf("body lost the code: %q", body[:min(80, len(body))])
	}
	if strings.Contains(body, "a note") || strings.Contains(body, "documentation") {
		t.Error("comments were served")
	}
	if got := w.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Errorf("content type %q", got)
	}
}

func TestAssetsCompresses(t *testing.T) {
	a := testAssets()
	w := fetch(t, a, "/app.js", http.Header{"Accept-Encoding": {"gzip"}})
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("encoding %q", got)
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	plain := fetch(t, a, "/app.js", nil)
	if string(body) != plain.Body.String() {
		t.Error("compressed and plain bodies differ")
	}
	if w.Body.Len() >= plain.Body.Len() {
		t.Errorf("gzip did not help: %d vs %d", w.Body.Len(), plain.Body.Len())
	}
	if got := w.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary %q", got)
	}
}

func TestAssetsRevalidates(t *testing.T) {
	a := testAssets()
	first := fetch(t, a, "/style.css", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	again := fetch(t, a, "/style.css", http.Header{"If-None-Match": {etag}})
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Errorf("re-request: status %d, %d bytes", again.Code, again.Body.Len())
	}
	weak := fetch(t, a, "/style.css", http.Header{"If-None-Match": {`W/` + etag}})
	if weak.Code != http.StatusNotModified {
		t.Errorf("a weakened tag was not matched: %d", weak.Code)
	}
	stale := fetch(t, a, "/style.css", http.Header{"If-None-Match": {`"nonsense"`}})
	if stale.Code != http.StatusOK {
		t.Errorf("a stale tag should be served: %d", stale.Code)
	}
}

func TestAssetsServesIndexAndBinaries(t *testing.T) {
	a := testAssets()
	for _, tc := range []struct{ path, ctype, body string }{
		{"/", "text/html; charset=utf-8", "<p>hi</p>"},
		{"/index.html", "text/html; charset=utf-8", "<p>hi</p>"},
		{"/hand.glb", "model/gltf-binary", "glTF binary bytes"},
	} {
		w := fetch(t, a, tc.path, nil)
		if w.Code != http.StatusOK {
			t.Errorf("%s: status %d", tc.path, w.Code)
			continue
		}
		if got := w.Header().Get("Content-Type"); got != tc.ctype {
			t.Errorf("%s: content type %q, want %q", tc.path, got, tc.ctype)
		}
		if !strings.Contains(w.Body.String(), tc.body) {
			t.Errorf("%s: body %q", tc.path, w.Body.String())
		}
	}
}

func TestAssetsRejectsWhatIsNotThere(t *testing.T) {
	a := testAssets()
	for _, p := range []string{"/nope.js", "/../server.go", "//etc/passwd"} {
		if w := fetch(t, a, p, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", p, w.Code)
		}
	}
}
