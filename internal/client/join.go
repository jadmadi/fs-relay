package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fs-relay/internal/wire"
)

const rejoinWait = 100 * time.Millisecond

// Listen holds the join stream until ctx is cancelled. The first drop rejoins
// at once. A further drop waits, then rejoins with the same client uuid.
func (c *Client) Listen(ctx context.Context) {
	c.ensure()
	var id string
	drops := 0
	for ctx.Err() == nil {
		if drops > 1 {
			timer := time.NewTimer(rejoinWait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		next, err := c.joinOnce(ctx, id)
		if next != "" {
			id = next
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			drops++
		}
	}
}

func (c *Client) joinOnce(ctx context.Context, id string) (string, error) {
	body, err := wire.MarshalRequest(wire.Join, wire.Params{Namespace: c.NS, Client: id})
	if err != nil {
		return id, err
	}
	endpoint := strings.TrimRight(c.URL, "/") + "/fs/" + url.PathEscape(c.NS)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return id, err
	}
	req.Header.Set("Content-Type", wire.ContentType)
	resp, err := c.http().Do(req)
	if err != nil {
		return id, fmt.Errorf("join: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return id, fmt.Errorf("join: http %d", resp.StatusCode)
	}
	return c.readJoin(ctx, resp.Body, id)
}

func (c *Client) readJoin(ctx context.Context, body io.Reader, id string) (string, error) {
	for {
		frame, err := wire.ReadFrame(body)
		if err != nil {
			return id, joinErr(err)
		}
		line, err := wire.DecodePub(frame)
		if err != nil {
			return id, err
		}
		if line.Client != "" {
			id = line.Client
		}
		if line.Evict == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return id, ctx.Err()
		case c.evicts <- *line.Evict:
		}
	}
}

func joinErr(err error) error {
	if err == nil {
		return io.EOF
	}
	return err
}
