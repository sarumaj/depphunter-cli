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
	command *exec.Cmd
	conn    *jsonrpc2.Conn
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

func start(ctx context.Context, directory string, argv []string) (*client, error) {
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = directory
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	// Implements: REQ-DIST-016
	stream := jsonrpc2.NewBufferedStream(stdio{stdout, stdin}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(context.Background(), stream, jsonrpc2.HandlerWithError(answer))
	return &client{command: command, conn: conn}, nil
}

// answer replies to the server's own requests with neutral results: servers ask for
// settings (workspace/configuration), progress tokens and capability registration.
// Notifications (diagnostics, progress, logs) are ignored.
func answer(_ context.Context, _ *jsonrpc2.Conn, request *jsonrpc2.Request) (any, error) {
	if request.Method == "workspace/configuration" && request.Params != nil {
		var p struct {
			Items []json.RawMessage `json:"items"`
		}
		json.Unmarshal(*request.Params, &p)
		return make([]any, len(p.Items)), nil // one null (defaults) per requested section
	}
	return nil, nil
}

// call sends a request and decodes its result into out (which may be nil).
func (c *client) call(ctx context.Context, method string, parameters, out any) error {
	return c.conn.Call(ctx, method, parameters, out)
}

func (c *client) notify(method string, parameters any) error {
	return c.conn.Notify(context.Background(), method, parameters)
}

// shutdown asks the server to exit and makes sure it does.
func (c *client) shutdown(ctx context.Context) {
	c.conn.Call(ctx, "shutdown", nil, nil)
	c.conn.Notify(ctx, "exit", nil)
	c.conn.Close()
	done := make(chan struct{})
	go func() { c.command.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		c.command.Process.Kill()
		<-done
	case <-time.After(2 * time.Second):
		c.command.Process.Kill()
		<-done
	}
}
