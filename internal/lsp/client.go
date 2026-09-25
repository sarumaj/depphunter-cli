// Package lsp finds symbol-level references by asking language servers (gopls,
// typescript-language-server, pyright, …) where each definition is used.
package lsp

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"time"

	"github.com/sourcegraph/jsonrpc2"
)

// client is a JSON-RPC connection to a language server process over stdio, framed
// with LSP's Content-Length headers.
//
// Implements: REQ-LSP-002
type client struct {
	cmd  *exec.Cmd
	conn *jsonrpc2.Conn
}

// stdio joins the server's stdout and stdin into the one stream jsonrpc2 expects.
type stdio struct {
	io.ReadCloser
	io.WriteCloser
}

func (s stdio) Close() error {
	s.WriteCloser.Close()
	return s.ReadCloser.Close()
}

func start(ctx context.Context, dir string, argv []string) (*client, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// Implements: REQ-DIST-016
	stream := jsonrpc2.NewBufferedStream(stdio{stdout, stdin}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(context.Background(), stream, jsonrpc2.HandlerWithError(answer))
	return &client{cmd: cmd, conn: conn}, nil
}

// answer replies to the server's own requests with neutral results: servers ask for
// settings (workspace/configuration), progress tokens and capability registration.
// Notifications (diagnostics, progress, logs) are ignored.
func answer(_ context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
	if req.Method == "workspace/configuration" && req.Params != nil {
		var p struct {
			Items []json.RawMessage `json:"items"`
		}
		json.Unmarshal(*req.Params, &p)
		return make([]any, len(p.Items)), nil // one null (defaults) per requested section
	}
	return nil, nil
}

// call sends a request and decodes its result into out (which may be nil).
func (c *client) call(ctx context.Context, method string, params, out any) error {
	return c.conn.Call(ctx, method, params, out)
}

func (c *client) notify(method string, params any) error {
	return c.conn.Notify(context.Background(), method, params)
}

// shutdown asks the server to exit and makes sure it does.
func (c *client) shutdown(ctx context.Context) {
	c.conn.Call(ctx, "shutdown", nil, nil)
	c.conn.Notify(ctx, "exit", nil)
	c.conn.Close()
	done := make(chan struct{})
	go func() { c.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		c.cmd.Process.Kill()
		<-done
	case <-time.After(2 * time.Second):
		c.cmd.Process.Kill()
		<-done
	}
}
