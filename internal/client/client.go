// Package client sends one relay call as one POST.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"fs-relay/internal/wire"
)

// Client talks to one fs-relay server for one namespace.
type Client struct {
	URL   string
	NS    string
	Token string
	HTTP  *http.Client

	once   sync.Once
	evicts chan wire.Change
	rel    chan struct{}
	mu     sync.Mutex
	id     string
}

func (c *Client) setID(id string) {
	if id == "" {
		return
	}
	c.mu.Lock()
	c.id = id
	c.mu.Unlock()
}

func (c *Client) clientID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.id
}

// Events receives cache evictions from the join stream.
func (c *Client) Events() <-chan wire.Change {
	c.ensure()
	return c.evicts
}

func (c *Client) ensure() {
	c.once.Do(func() {
		c.evicts = make(chan wire.Change, 32)
	})
}

// Releases returns the current release broadcast. A release closes it and
// installs a new one, so every waiter wakes and the reader never blocks.
func (c *Client) Releases() <-chan struct{} {
	c.ensure()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rel == nil {
		c.rel = make(chan struct{})
	}
	return c.rel
}

func (c *Client) notifyRelease() {
	c.ensure()
	c.mu.Lock()
	ch := c.rel
	c.rel = make(chan struct{})
	c.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

// Call posts method and returns the result. A filesystem failure is *wire.Error.
func (c *Client) Call(ctx context.Context, method string, p wire.Params) (wire.Result, error) {
	if p.Namespace == "" {
		p.Namespace = c.NS
	}
	if p.Client == "" {
		p.Client = c.clientID()
	}
	body, err := wire.MarshalRequest(method, p)
	if err != nil {
		return wire.Result{}, err
	}
	endpoint := strings.TrimRight(c.URL, "/") + "/fs/" + url.PathEscape(p.Namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return wire.Result{}, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", wire.ContentType)
	authorize(req.Header, c.Token)
	resp, err := c.http().Do(req)
	if err != nil {
		return wire.Result{}, fmt.Errorf("post %s: %w", method, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, wire.MaxBody))
	if err != nil {
		return wire.Result{}, fmt.Errorf("read %s: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		return wire.Result{}, statusErr("post "+method, resp.StatusCode, raw)
	}
	res, err := wire.DecodeResponse(raw)
	if err != nil {
		return wire.Result{}, err
	}
	return res, nil
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func authorize(h http.Header, token string) {
	h.Set(wire.TokenHeader, wire.TokenScheme+" "+token)
}

// rejected is a token the server will keep refusing. Retrying does not change it.
type rejected struct{ Why string }

func (e *rejected) Error() string {
	return fmt.Sprintf("the server refused the token (%s). Stopping. If it restarted without -token, it printed a new one", e.Why)
}

// Refused reports a token rejection. Another try will get the same answer.
func Refused(err error) bool {
	var rej *rejected
	return errors.As(err, &rej)
}

// missing is a namespace the server does not have. Retrying does not change it.
type missing struct{ why string }

func (e *missing) Error() string { return e.why }

// Missing reports a missing namespace. Another try will get the same answer.
func Missing(err error) bool {
	var m *missing
	return errors.As(err, &m)
}

func statusErr(op string, status int, body []byte) error {
	if status == http.StatusUnauthorized {
		why := tokenWhy(body)
		if why == "" {
			why = "unauthorized"
		}
		return &rejected{Why: why}
	}
	if status == http.StatusNotFound {
		why := strings.TrimSpace(string(body))
		if why == "" {
			why = "namespace does not exist"
		}
		return &missing{why: why}
	}
	return fmt.Errorf("%s: http %d", op, status)
}

func tokenWhy(body []byte) string {
	var msg struct {
		Error wire.Error `json:"error"`
	}
	if json.Unmarshal(body, &msg) != nil || msg.Error.Message == "" {
		return ""
	}
	return msg.Error.Message
}
