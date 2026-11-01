package client

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"fs-relay/internal/wire"

	"github.com/coder/websocket"
)

func TestRepliesFollowIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		first, err := readCall(conn)
		if err != nil {
			t.Error(err)
			return
		}
		second, err := readCall(conn)
		if err != nil {
			t.Error(err)
			return
		}
		if err := writeReply(conn, second.ID, wire.Result{Written: mark(second.Method)}); err != nil {
			t.Error(err)
			return
		}
		if err := writeReply(conn, first.ID, wire.Result{Written: mark(first.Method)}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	sock := mustDial(t, srv.URL)

	var wg sync.WaitGroup
	wg.Add(2)
	var first, second wire.Result
	go func() {
		defer wg.Done()
		res, err := sock.Call(context.Background(), wire.Write, wire.Params{})
		if err != nil {
			t.Error(err)
		}
		first = res
	}()
	go func() {
		defer wg.Done()
		res, err := sock.Call(context.Background(), wire.Read, wire.Params{})
		if err != nil {
			t.Error(err)
		}
		second = res
	}()
	wg.Wait()
	if first.Written != 1 || second.Written != 2 {
		t.Fatalf("write %d read %d", first.Written, second.Written)
	}
}

func TestPendingThenResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		req, err := readCall(conn)
		if err != nil {
			t.Error(err)
			return
		}
		b, err := wire.MarshalPending(req.ID, "ticket")
		if err != nil {
			t.Error(err)
			return
		}
		if err := conn.Write(context.Background(), websocket.MessageBinary, b); err != nil {
			t.Error(err)
			return
		}
		if err := writeReply(conn, req.ID, wire.Result{Written: 7}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	sock := mustDial(t, srv.URL)
	res, err := sock.Call(context.Background(), wire.Write, wire.Params{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Written != 7 {
		t.Fatalf("written %d", res.Written)
	}
}

func mustDial(t *testing.T, base string) *Socket {
	t.Helper()
	sock, err := Dial(context.Background(), base, "demo", "testtoken")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sock.Close() })
	return sock
}

func mark(method string) uint32 {
	if method == wire.Write {
		return 1
	}
	return 2
}

func readCall(conn *websocket.Conn) (wire.Request, error) {
	_, data, err := conn.Read(context.Background())
	if err != nil {
		return wire.Request{}, err
	}
	return wire.DecodeRequest(bytes.NewReader(data))
}

func writeReply(conn *websocket.Conn, id uint64, res wire.Result) error {
	b, err := wire.MarshalReply(id, res)
	if err != nil {
		return err
	}
	return conn.Write(context.Background(), websocket.MessageBinary, b)
}
