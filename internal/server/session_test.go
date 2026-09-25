package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// send performs one state-changing request, with the header the guard insists on.
func send(t *testing.T, c *http.Client, method, url, body string) int {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set(requestHeader, "1")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

// stream reads the event names and payloads off an open /api/events connection.
func stream(t *testing.T, c *http.Client, base string) func() string {
	t.Helper()
	res, err := c.Get(base + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	lines := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(res.Body)
		name := ""
		for sc.Scan() {
			line := sc.Text()
			if rest, ok := strings.CutPrefix(line, "event: "); ok {
				name = rest
			}
			if rest, ok := strings.CutPrefix(line, "data: "); ok {
				lines <- name + " " + rest
			}
		}
	}()
	return func() string {
		select {
		case l := <-lines:
			return l
		case <-time.After(3 * time.Second):
			t.Fatal("no event")
			return ""
		}
	}
}

// Verifies: REQ-SRV-010, REQ-SRV-012
func TestSelectionIsSharedAndSaysWhoMadeIt(t *testing.T) {
	_, url, base := start(t)
	c := login(t, url)
	next := stream(t, c, base)
	if e := next(); !strings.HasPrefix(e, `hello {"version":1,`) {
		t.Fatalf("hello: %s", e)
	}

	if code := send(t, c, http.MethodPost, base+"/api/selection", `{"id":"f:a.go","origin":"panel"}`); code != http.StatusNoContent {
		t.Fatalf("select: %d", code)
	}
	// The announcement names the client that made the change, so that client can
	// tell its own selection coming back from somebody else's.
	if e := next(); e != `selection {"id":"f:a.go","origin":"panel"}` {
		t.Fatalf("selection event: %s", e)
	}
	// Selecting the same thing again is not news.
	if code := send(t, c, http.MethodPost, base+"/api/selection", `{"id":"f:a.go","origin":"page"}`); code != http.StatusNoContent {
		t.Fatalf("re-select: %d", code)
	}

	code, body := get(t, c, base+"/api/session", nil)
	var session Session
	if code != http.StatusOK || json.Unmarshal([]byte(body), &session) != nil {
		t.Fatalf("session: %d %s", code, body)
	}
	if session.Selected != "f:a.go" {
		t.Errorf("selected %q", session.Selected)
	}
	if session.Backpack == nil {
		t.Error("an empty backpack should still be a list, not null")
	}
}

// Verifies: REQ-SRV-009, REQ-SRV-011, REQ-EXP-014
func TestBackpackRelayAndExports(t *testing.T) {
	_, url, base := start(t)
	c := login(t, url)
	next := stream(t, c, base)
	next() // hello

	items := `{"origin":"page","items":[
		{"id":"GO-1","severity":"low","title":"a small thing","where":"a.go","line":3,"caughtAt":1700000000000},
		{"id":"GO-2","severity":"critical","title":"a large thing","where":"pkg","fixed":true}]}`
	if code := send(t, c, http.MethodPut, base+"/api/backpack", items); code != http.StatusNoContent {
		t.Fatalf("put: %d", code)
	}
	if e := next(); e != `backpack {"count":2,"origin":"page"}` {
		t.Fatalf("backpack event: %s", e)
	}

	// An item with no id is not one anything could be done with later.
	if code := send(t, c, http.MethodPut, base+"/api/backpack", `{"items":[{"title":"nameless"}]}`); code != http.StatusBadRequest {
		t.Errorf("item without an id: %d", code)
	}

	code, body := get(t, c, base+"/api/backpack?format=md", nil)
	if code != http.StatusOK {
		t.Fatalf("markdown: %d %s", code, body)
	}
	// Worst first, whatever order they were caught in, and the fixed one is ticked.
	first := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "- ") {
			first = line
			break
		}
	}
	if first != "- [x] **critical** a large thing - `pkg` (GO-2)" {
		t.Errorf("first item was %q; want the critical one, ticked:\n%s", first, body)
	}
	if !strings.Contains(body, "`a.go:3`") {
		t.Errorf("markdown lost the line number:\n%s", body)
	}

	code, body = get(t, c, base+"/api/backpack?format=csv", nil)
	if code != http.StatusOK || !strings.HasPrefix(body, "id,severity,title,where,line,caught,fixed\n") {
		t.Fatalf("csv: %d %s", code, body)
	}
	if !strings.Contains(body, "2023-11-14T22:13:20Z") {
		t.Errorf("csv lost when it was caught:\n%s", body)
	}

	if code, body = get(t, c, base+"/api/backpack?format=xml", nil); code != http.StatusBadRequest {
		t.Errorf("unknown format: %d %s", code, body)
	}
}

// Verifies: REQ-SRV-011
func TestBackpackIsCappedAtWhatTheBrowserHolds(t *testing.T) {
	_, url, base := start(t)
	c := login(t, url)
	var items []PackItem
	for i := range maxPack + 50 {
		items = append(items, PackItem{ID: "F-" + string(rune('a'+i%26)) + strings.Repeat("x", i%7), Severity: "low"})
	}
	body, _ := json.Marshal(map[string]any{"items": items})
	if code := send(t, c, http.MethodPut, base+"/api/backpack", string(body)); code != http.StatusNoContent {
		t.Fatalf("put: %d", code)
	}
	code, out := get(t, c, base+"/api/session", nil)
	var session Session
	if code != http.StatusOK || json.Unmarshal([]byte(out), &session) != nil {
		t.Fatalf("session: %d", code)
	}
	if len(session.Backpack) != maxPack {
		t.Errorf("kept %d items, want the same %d the page holds", len(session.Backpack), maxPack)
	}
}

func TestSessionWritesNeedTheRequestHeader(t *testing.T) {
	_, url, base := start(t)
	c := login(t, url)
	for _, call := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/selection", `{"id":"f:a.go"}`},
		{http.MethodPut, "/api/backpack", `{"items":[]}`},
	} {
		req, _ := http.NewRequest(call.method, base+call.path, strings.NewReader(call.body))
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s without the header: %d", call.method, call.path, res.StatusCode)
		}
	}
}
