// Package client sends one relay call as one POST.
package client

import (
	"bytes"
	"context"
	"encoding/json"
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

func statusErr(op string, status int, body []byte) error {
	if status == http.StatusUnauthorized {
		if why := tokenWhy(body); why != "" {
			return fmt.Errorf("%s: %s", op, why)
		}
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
