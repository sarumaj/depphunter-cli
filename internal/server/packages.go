package server

import (
	"encoding/json"
	"net/http"
	"path"
	"runtime"

	"github.com/sarumaj/depphunter-cli/internal/editor"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/locate"
)

// pkg is the package node id names, or nil.
func (sn *snapshot) pkg(id string) *graph.Node {
	for n := range sn.g.Of(graph.KindPackage) {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// folder is where a package is installed on this machine, looked for first beside
// the files that use it.
func (s *Server) folder(sn *snapshot, n *graph.Node) string {
	var near []string
	seen := map[string]bool{}
	for _, e := range sn.g.Edges {
		if e.To != n.ID {
			continue
		}
		if file := graph.FileOf(e.From); file != "" && !seen[path.Dir(file)] {
			seen[path.Dir(file)] = true
			near = append(near, path.Dir(file))
		}
	}
	return locate.Folder(s.root, s.machine, locate.Package{
		Ecosystem: graph.EcosystemOf(n.Parent), Name: n.Name, Version: n.Version, Origin: n.Origin, Near: near,
	})
}

// handleLocate answers where a package is installed: {"folder": "<absolute path>"},
// or 404 when it is nowhere this machine keeps one.
//
// Implements: REQ-SRV-018
func (s *Server) handleLocate(w http.ResponseWriter, r *http.Request) {
	sn := s.current()
	n := sn.pkg(r.URL.Query().Get("id"))
	if n == nil {
		http.NotFound(w, r)
		return
	}
	folder := s.folder(sn, n)
	if folder == "" {
		http.Error(w, n.Name+" is not installed anywhere depphunter looks", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"folder": folder})
}

// handleBrowse opens what a package node points at: {"id": ..., "to": "page" |
// "repository" | "folder"}. Only what the graph says, or the folder the server found
// itself, is ever opened: the request names a package, never an address.
//
// A stream that opens links (?opens=links) is handed it as a "browse" event, with
// the package's name, and the answer is 202: the map in the extension's tab is in a frame that can open neither
// a window nor a folder. Without one, a folder is shown in the system's file manager
// - on the machine the server runs on, which is why a server listening beyond it
// refuses - and a link is left to the page, which can open it itself (501).
//
// Implements: REQ-SRV-019
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	type browseRequest struct {
		ID string `json:"id"`
		To string `json:"to"`
	}
	request, ok := decodeBody[browseRequest](w, r, 4096)
	if !ok {
		return
	}
	sn := s.current()
	n := sn.pkg(request.ID)
	if n == nil {
		http.NotFound(w, r)
		return
	}
	target := map[string]string{"name": n.Name}
	switch request.To {
	case "page":
		target["url"] = n.Page
	case "repository":
		target["url"] = n.Repository
	case "folder":
		target["folder"] = s.folder(sn, n)
	default:
		http.Error(w, "to must be page, repository or folder", http.StatusBadRequest)
		return
	}
	if target["url"] == "" && target["folder"] == "" {
		http.NotFound(w, r)
		return
	}
	if s.handTo(s.linkers, event{name: "browse", data: mustJSON(target)}) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if target["folder"] == "" || s.allowedHosts == nil {
		http.Error(w, "nothing here opens it", http.StatusNotImplemented)
		return
	}
	command := reveal(runtime.GOOS, target["folder"])
	if err := command.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	go command.Wait() // reap the file manager's launcher
	w.WriteHeader(http.StatusNoContent)
}

// reveal is editor.Reveal, which tests replace.
var reveal = editor.Reveal
