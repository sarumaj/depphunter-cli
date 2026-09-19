// Package lsp finds symbol-level references by asking language servers (gopls,
// typescript-language-server, pyright, …) where each definition is used.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// client speaks JSON-RPC 2.0 over a language server's stdin/stdout.
type client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	writeM sync.Mutex
	nextID atomic.Int64

	mu      sync.Mutex
	pending map[int64]chan response
	closed  chan struct{}
	err     error // why the connection ended
}

type response struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	response
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
	c := &client{cmd: cmd, stdin: stdin, pending: map[int64]chan response{}, closed: make(chan struct{})}
	go c.read(bufio.NewReaderSize(stdout, 1<<16))
	return c, nil
}

// read dispatches responses to their callers and answers the server's own requests.
func (c *client) read(r *bufio.Reader) {
	err := func() error {
		for {
			body, err := readFrame(r)
			if err != nil {
				return err
			}
			var m message
			if err := json.Unmarshal(body, &m); err != nil {
				continue
			}
			switch {
			case m.Method != "" && len(m.ID) > 0:
				c.answer(m)
			case m.Method != "": // notification: diagnostics, progress, logs
			default:
				id, _ := strconv.ParseInt(string(m.ID), 10, 64)
				c.mu.Lock()
				ch := c.pending[id]
				delete(c.pending, id)
				c.mu.Unlock()
				if ch != nil {
					ch <- m.response
				}
			}
		}
	}()
	c.mu.Lock()
	c.err = err
	close(c.closed)
	c.mu.Unlock()
}

// answer replies to server-to-client requests with neutral results: servers ask for
// settings (workspace/configuration), progress tokens and capability registration.
func (c *client) answer(m message) {
	var result any
	if m.Method == "workspace/configuration" {
		var p struct {
			Items []json.RawMessage `json:"items"`
		}
		json.Unmarshal(m.Params, &p)
		result = make([]any, len(p.Items)) // one null (defaults) per requested section
	}
	c.send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": result})
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(k, "Content-Length") {
			length, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if length < 0 {
		return nil, errors.New("lsp: frame without Content-Length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}

func (c *client) send(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeM.Lock()
	defer c.writeM.Unlock()
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.stdin.Write(body)
	return err
}

func (c *client) notify(method string, params any) error {
	return c.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// call sends a request and decodes its result into out (which may be nil).
func (c *client) call(ctx context.Context, method string, params, out any) error {
	id := c.nextID.Add(1)
	ch := make(chan response, 1)
	c.mu.Lock()
	select {
	case <-c.closed:
		c.mu.Unlock()
		return fmt.Errorf("lsp: server exited: %v", c.err)
	default:
	}
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return err
	}
	select {
	case r := <-ch:
		if r.Error != nil {
			return fmt.Errorf("lsp: %s: %s", method, r.Error.Message)
		}
		if out != nil && len(r.Result) > 0 {
			return json.Unmarshal(r.Result, out)
		}
		return nil
	case <-c.closed:
		return fmt.Errorf("lsp: server exited during %s: %v", method, c.err)
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.notify("$/cancelRequest", map[string]any{"id": id})
		return ctx.Err()
	}
}

// shutdown asks the server to exit and makes sure it does.
func (c *client) shutdown(ctx context.Context) {
	c.call(ctx, "shutdown", nil, nil)
	c.notify("exit", nil)
	c.stdin.Close()
	select {
	case <-c.closed:
	case <-ctx.Done():
	}
	c.cmd.Process.Kill()
	c.cmd.Wait()
}
