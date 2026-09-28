package analyze

import (
	"context"
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// notedPlugin is a fakePlugin whose resolver has something to say to --explain.
type notedPlugin struct{ fakePlugin }

func (p notedPlugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	r := &notedResolver{fakeResolver: fakeResolver(p.fakePlugin)}
	r.Note("b.lock", trace.NoteFlat, "flat")
	r.Note("a.lock", trace.NoteUnread, "unread")
	r.Note("b.lock", trace.NoteFlat, "flat") // said twice, kept once
	return r, nil
}

type notedResolver struct {
	fakeResolver
	lang.NoteList
}

// A resolver's notes reach the report under the plugin's name, once each and in
// order, whether or not the walk ran.
//
// Verifies: REQ-TRC-017
func TestResolverNotesReachTheReport(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "x\n"})
	for _, depth := range []int{0, 1} {
		report := trace.New(depth, false, nil, nil)
		if _, _, err := Run(context.Background(), root, Options{
			Plugins: []lang.Plugin{notedPlugin{}}, ResolveDepth: depth, Trace: report,
		}); err != nil {
			t.Fatal(err)
		}
		want := []trace.Note{
			{Plugin: "fake", File: "a.lock", Code: trace.NoteUnread, Message: "unread"},
			{Plugin: "fake", File: "b.lock", Code: trace.NoteFlat, Message: "flat"},
		}
		if !reflect.DeepEqual(report.Notes, want) {
			t.Errorf("depth %d: notes %+v, want %+v", depth, report.Notes, want)
		}
	}
}

// End to end through a real plugin: a Bun project with only bun.lockb says so.
//
// Verifies: REQ-TRC-017, REQ-JS-017
func TestBunLockbIsNoted(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{
		"package.json": `{"dependencies": {"react": "^18"}}`,
		"bun.lockb":    "\x00binary",
		"index.js":     "import React from 'react'\n",
	})
	report := trace.New(0, false, nil, nil)
	if _, _, err := Run(context.Background(), root, Options{
		Plugins: []lang.Plugin{javascript.Plugin{}}, Trace: report,
	}); err != nil {
		t.Fatal(err)
	}
	if len(report.Notes) != 1 || report.Notes[0].File != "bun.lockb" || report.Notes[0].Code != trace.NoteUnread ||
		report.Notes[0].Plugin != (javascript.Plugin{}).Name() {
		t.Errorf("notes: %+v", report.Notes)
	}
}
