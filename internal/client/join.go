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

const (
	rejoinWait  = 100 * time.Millisecond
	rejoinReset = time.Second
)

// Listen holds the join stream until ctx is cancelled. The first try of an
// outage runs at once. A further try waits. A session that stays up clears that wait.
// A refused token is returned and is not retried.
func (c *Client) Listen(ctx context.Context) error {
	c.ensure()
	var id string
	drops := 0
	for ctx.Err() == nil {
		if drops > 0 && !waitRejoin(ctx) {
			return nil
		}
		started := time.Now()
		next, up, err := c.joinOnce(ctx, id)
		if next != "" {
			id = next
		}
		if ctx.Err() != nil {
			return nil
		}
		if Refused(err) {
			return err
		}
		if err != nil && up {
			logLost(c.URL)
		}
		drops = nextDrops(drops, up, time.Since(started))
	}
	return nil
}

func waitRejoin(ctx context.Context) bool {
	timer := time.NewTimer(rejoinWait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextDrops(drops int, established bool, alive time.Duration) int {
	if established && alive >= rejoinReset {
		return 0
	}
	return drops + 1
}

func (c *Client) joinOnce(ctx context.Context, id string) (string, bool, error) {
	body, err := wire.MarshalRequest(wire.Join, wire.Params{Namespace: c.NS, Client: id})
	if err != nil {
		return id, false, err
	}
	endpoint := strings.TrimRight(c.URL, "/") + "/fs/" + url.PathEscape(c.NS)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return id, false, err
	}
	req.Header.Set("Content-Type", wire.ContentType)
	authorize(req.Header, c.Token)
	resp, err := c.http().Do(req)
	if err != nil {
		return id, false, fmt.Errorf("join: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return id, false, statusErr("join", resp.StatusCode, raw)
	}
	return c.readJoin(ctx, resp.Body, id)
}

type joinFrame struct {
	line wire.Pub
	err  error
}

func (c *Client) readJoin(ctx context.Context, body io.Reader, id string) (string, bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	frames := make(chan joinFrame, 1)
	go readFrames(ctx, body, frames)
	stall := time.NewTimer(wire.Stall)
	defer stall.Stop()
	rejoin := id != ""
	up := false
	for {
		select {
		case <-ctx.Done():
			return id, up, ctx.Err()
		case <-stall.C:
			return id, up, fmt.Errorf("join: stalled")
		case fr := <-frames:
			if fr.err != nil {
				return id, up, joinErr(fr.err)
			}
			stall.Reset(wire.Stall)
			id, up = c.saw(fr.line, id, up, rejoin)
			if fr.line.Evict == nil {
				continue
			}
			select {
			case <-ctx.Done():
				return id, up, ctx.Err()
			case c.evicts <- *fr.line.Evict:
			}
		}
	}
}

func readFrames(ctx context.Context, body io.Reader, out chan<- joinFrame) {
	for {
		frame, err := wire.ReadFrame(body)
		got := joinFrame{err: err}
		if err == nil {
			got.line, got.err = wire.DecodePub(frame)
		}
		select {
		case <-ctx.Done():
			return
		case out <- got:
		}
		if got.err != nil {
			return
		}
	}
}

func (c *Client) saw(line wire.Pub, id string, up, rejoin bool) (string, bool) {
	if line.Client == "" {
		return id, up
	}
	id = line.Client
	if up {
		return id, true
	}
	if rejoin {
		logReconnected(c.URL, id)
		return id, true
	}
	logConnected(c.URL, id)
	return id, true
}

func joinErr(err error) error {
	if err == nil {
		return io.EOF
	}
	return err
}
