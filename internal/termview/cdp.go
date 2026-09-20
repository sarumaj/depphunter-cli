// Package termview shows the web interface inside the terminal: it drives a headless
// Chromium over the DevTools protocol, paints the frames it streams back with the
// terminal's graphics protocol, and forwards the terminal's keys and mouse to the page.
//
// The map is one WebGL canvas (three.js), so a text browser has nothing to render;
// a real engine has to draw the pixels and something has to carry them to the
// terminal. That something is this package.
package termview

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// The protocol speaks JSON over a pair of pipes (--remote-debugging-pipe): the browser
// reads commands from file descriptor 3 and writes replies and events to 4, each
// message terminated by a NUL byte. The pipes avoid opening a debugging port that
// anything on the machine could connect to.
const (
	pipeReadFD  = 3
	pipeWriteFD = 4
	msgEnd      = 0
)

// message is a DevTools command, reply or event. A reply carries the id of the command
// it answers; an event carries a method and the session it happened in.
type message struct {
	ID        int             `json:"id,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *protocolError  `json:"error,omitempty"`
}

type protocolError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func (e *protocolError) Error() string {
	if e.Data != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Data)
	}
	return e.Message
}

// client is a DevTools connection: commands in, replies and events out.
type client struct {
	in       io.WriteCloser // commands to the browser
	out      io.ReadCloser  // replies and events from it
	onEvent  func(sessionID, method string, params json.RawMessage)
	mu       sync.Mutex
	lastID   int
	pending  map[int]chan message
	closed   bool
	readDone chan struct{}
	readErr  error
}

// newClient starts the read loop over the two pipe ends. onEvent runs on that loop, so
// it must not block: the screencast handler only hands the frame on.
func newClient(out io.ReadCloser, in io.WriteCloser, onEvent func(sessionID, method string, params json.RawMessage)) *client {
	c := &client{in: in, out: out, onEvent: onEvent, pending: map[int]chan message{}, readDone: make(chan struct{})}
	go c.read()
	return c
}

func (c *client) read() {
	defer close(c.readDone)
	r := bufio.NewReader(c.out)
	for {
		raw, err := r.ReadBytes(msgEnd)
		if len(raw) > 1 {
			var m message
			if jsonErr := json.Unmarshal(raw[:len(raw)-1], &m); jsonErr == nil {
				c.dispatch(m)
			}
		}
		if err != nil {
			c.fail(err)
			return
		}
	}
}

func (c *client) dispatch(m message) {
	if m.ID == 0 {
		if c.onEvent != nil && m.Method != "" {
			c.onEvent(m.SessionID, m.Method, m.Params)
		}
		return
	}
	c.mu.Lock()
	ch, ok := c.pending[m.ID]
	delete(c.pending, m.ID)
	c.mu.Unlock()
	if ok {
		ch <- m
	}
}

// fail wakes everyone waiting for a reply once the browser is gone.
func (c *client) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.readErr == nil {
		c.readErr = err
	}
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
}

// call sends a command and waits for its reply. sessionID selects an attached target;
// "" addresses the browser itself. result may be nil when the reply is not needed.
func (c *client) call(ctx context.Context, sessionID, method string, params, result any) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("devtools connection closed")
	}
	c.lastID++
	id := c.lastID
	ch := make(chan message, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	cmd := struct {
		ID        int    `json:"id"`
		SessionID string `json:"sessionId,omitempty"`
		Method    string `json:"method"`
		Params    any    `json:"params,omitempty"`
	}{ID: id, SessionID: sessionID, Method: method, Params: params}
	body, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	if _, err := c.in.Write(append(body, msgEnd)); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return ctx.Err()
	case m, ok := <-ch:
		if !ok {
			c.mu.Lock()
			err := c.readErr
			c.mu.Unlock()
			return fmt.Errorf("%s: browser disconnected: %w", method, err)
		}
		if m.Error != nil {
			return fmt.Errorf("%s: %w", method, m.Error)
		}
		if result != nil && len(m.Result) > 0 {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	}
}

func (c *client) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()
	c.in.Close()
	c.out.Close()
}

// pipes wires the two extra file descriptors a --remote-debugging-pipe browser expects
// onto cmd, and returns the ends this process keeps.
func pipes(cmd *exec.Cmd) (browserOut io.ReadCloser, browserIn io.WriteCloser, err error) {
	toBrowserR, toBrowserW, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	fromBrowserR, fromBrowserW, err := os.Pipe()
	if err != nil {
		toBrowserR.Close()
		toBrowserW.Close()
		return nil, nil, err
	}
	// ExtraFiles[0] becomes descriptor 3 in the child, ExtraFiles[1] descriptor 4.
	cmd.ExtraFiles = append(cmd.ExtraFiles, toBrowserR, fromBrowserW)
	return fromBrowserR, toBrowserW, nil
}
