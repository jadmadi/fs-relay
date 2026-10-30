// Package client sends one relay call as one POST.
package client

import (
	"bytes"
	"context"
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
	URL  string
	NS   string
	HTTP *http.Client

	once   sync.Once
	evicts chan wire.Change
}

// Events receives cache evictions from the join stream.
func (c *Client) Events() <-chan wire.Change {
	c.ensure()
	return c.evicts
}

func (c *Client) ensure() {
	c.once.Do(func() { c.evicts = make(chan wire.Change, 32) })
}

// Call posts method and returns the result. A filesystem failure is *wire.Error.
func (c *Client) Call(ctx context.Context, method string, p wire.Params) (wire.Result, error) {
	if p.Namespace == "" {
		p.Namespace = c.NS
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
		return wire.Result{}, fmt.Errorf("post %s: http %d", method, resp.StatusCode)
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
