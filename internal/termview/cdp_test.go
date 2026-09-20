package termview

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeBrowser answers commands the way the browser does: JSON messages separated by a
// NUL byte, replies carrying the id they answer.
type fakeBrowser struct {
	t       *testing.T
	in      io.Reader // what the client sends
	out     io.Writer // what the client reads
	events  chan string
	mu      sync.Mutex
	methods []string
}

func newFakeBrowser(t *testing.T) (*client, *fakeBrowser, func()) {
	t.Helper()
	toBrowser, fromClient := io.Pipe()
	toClient, fromBrowser := io.Pipe()
	f := &fakeBrowser{t: t, in: toBrowser, out: fromBrowser}
	events := make(chan string, 8)
	c := newClient(toClient, fromClient, func(sessionID, method string, params json.RawMessage) {
		events <- method + " " + sessionID + " " + string(params)
	})
	f.events = events
	go f.serve()
	return c, f, func() { c.close(); fromBrowser.Close() }
}

func (f *fakeBrowser) serve() {
	r := bufio.NewReader(f.in)
	for {
		raw, err := r.ReadBytes(msgEnd)
		if len(raw) > 1 {
			var m struct {
				ID        int    `json:"id"`
				SessionID string `json:"sessionId"`
				Method    string `json:"method"`
			}
			json.Unmarshal(raw[:len(raw)-1], &m)
			f.mu.Lock()
			f.methods = append(f.methods, m.Method)
			f.mu.Unlock()
			switch m.Method {
			case "Page.broken":
				f.reply(`{"id":` + strconv.Itoa(m.ID) + `,"error":{"code":-32000,"message":"no such thing"}}`)
			default:
				f.reply(`{"id":` + strconv.Itoa(m.ID) + `,"result":{"sessionId":"S1","echo":"` + m.SessionID + `"}}`)
			}
		}
		if err != nil {
			return
		}
	}
}

func (f *fakeBrowser) reply(s string) {
	f.out.Write(append([]byte(s), msgEnd))
}

func (f *fakeBrowser) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.methods...)
}

func TestClientCall(t *testing.T) {
	c, f, done := newFakeBrowser(t)
	defer done()
	ctx := context.Background()

	var res struct {
		SessionID string `json:"sessionId"`
		Echo      string `json:"echo"`
	}
	if err := c.call(ctx, "S9", "Target.attachToTarget", map[string]any{"flatten": true}, &res); err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "S1" || res.Echo != "S9" {
		t.Errorf("result %+v: the session did not travel with the command", res)
	}
	if got := f.seen(); len(got) != 1 || got[0] != "Target.attachToTarget" {
		t.Errorf("the browser saw %v", got)
	}
}

func TestClientCallReportsProtocolErrors(t *testing.T) {
	c, _, done := newFakeBrowser(t)
	defer done()
	err := c.call(context.Background(), "", "Page.broken", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no such thing") {
		t.Errorf("error %v, want the browser's own message", err)
	}
}

func TestClientDeliversEvents(t *testing.T) {
	c, f, done := newFakeBrowser(t)
	defer done()
	f.reply(`{"method":"Page.screencastFrame","sessionId":"S1","params":{"data":"AA","sessionId":3}}`)
	select {
	case got := <-f.events:
		if !strings.HasPrefix(got, "Page.screencastFrame S1 ") {
			t.Errorf("event %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event arrived")
	}
	_ = c
}

// discard stands in for the pipe to a browser that never reads: the commands go
// nowhere, which is what these tests are about.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
func (discard) Close() error                { return nil }

func TestClientWakesWaitersWhenTheBrowserGoesAway(t *testing.T) {
	toClient, fromBrowser := io.Pipe()
	c := newClient(toClient, discard{}, nil)
	defer c.close()

	errs := make(chan error, 1)
	go func() { errs <- c.call(context.Background(), "", "Page.enable", nil, nil) }()
	time.Sleep(50 * time.Millisecond)
	fromBrowser.Close() // the browser exits with a command in flight

	select {
	case err := <-errs:
		if err == nil || !strings.Contains(err.Error(), "disconnected") {
			t.Errorf("error %v, want a disconnect", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the call never returned")
	}
}

func TestClientCallHonoursContext(t *testing.T) {
	toClient, _ := io.Pipe()
	c := newClient(toClient, discard{}, nil)
	defer c.close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := c.call(ctx, "", "Page.enable", nil, nil); err == nil {
		t.Error("a command with no reply returned without an error")
	}
}
