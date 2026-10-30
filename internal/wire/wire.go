// Package wire is the MsgPack contract shared by the server and the mount.
// It does not speak HTTP, FUSE, or the filesystem.
package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

// ContentType is the only body type the server accepts.
const ContentType = "application/msgpack"

// Limits from SPEC: a namespace is bounded, and one FUSE write is one POST.
const (
	MaxNamespace = 64
	MaxWrite     = 1 << 20
	MaxBody      = MaxWrite + 64*1024
	// MaxReads and MaxPrefetch bound one reads call: a window of small files.
	MaxReads    = 32
	MaxPrefetch = 64 << 10
)

// Method names are the go-fuse methods, lowercased.
const (
	Lookup     = "lookup"
	Getattr    = "getattr"
	Setattr    = "setattr"
	Access     = "access"
	Statfs     = "statfs"
	Mkdir      = "mkdir"
	Create     = "create"
	Open       = "open"
	Read       = "read"
	Reads      = "reads"
	Write      = "write"
	Flush      = "flush"
	Fsync      = "fsync"
	Release    = "release"
	Opendir    = "opendir"
	Readdir    = "readdir"
	Releasedir = "releasedir"
	Unlink     = "unlink"
	Rmdir      = "rmdir"
	Rename     = "rename"
	Symlink    = "symlink"
	Readlink   = "readlink"
	Join       = "join"
)

// Setattr bits match Linux FUSE FATTR_* so the mount can pass Valid through.
const (
	SetMode     uint32 = 1 << 0
	SetUID      uint32 = 1 << 1
	SetGID      uint32 = 1 << 2
	SetSize     uint32 = 1 << 3
	SetAtime    uint32 = 1 << 4
	SetMtime    uint32 = 1 << 5
	SetFH       uint32 = 1 << 6
	SetAtimeNow uint32 = 1 << 7
	SetMtimeNow uint32 = 1 << 8
)

// Protocol errors. A filesystem failure is a syscall.Errno, not one of these.
var (
	ErrParams  = errors.New("bad params")
	ErrUnknown = errors.New("unknown method")
)

// Request is one call. ID is set on the websocket, where many calls share a socket.
// A POST omits it; the HTTP exchange is the correlation there.
type Request struct {
	ID     uint64 `msgpack:"id,omitempty"`
	Method string `msgpack:"method"`
	Params Params `msgpack:"params"`
}

// Params carries the kernel arguments plus the namespace.
// Zero values are omitted; a setattr to zero still works because Valid says which fields count.
type Params struct {
	Namespace string   `msgpack:"namespace"`
	Inode     uint64   `msgpack:"inode,omitempty"`
	NewInode  uint64   `msgpack:"new_inode,omitempty"`
	Name      string   `msgpack:"name,omitempty"`
	NewName   string   `msgpack:"new_name,omitempty"`
	Fh        uint64   `msgpack:"fh,omitempty"`
	Mode      uint32   `msgpack:"mode,omitempty"`
	Flags     uint32   `msgpack:"flags,omitempty"`
	Valid     uint32   `msgpack:"valid,omitempty"`
	Mask      uint32   `msgpack:"mask,omitempty"`
	Offset    int64    `msgpack:"offset,omitempty"`
	Size      uint64   `msgpack:"size,omitempty"`
	Atime     int64    `msgpack:"atime,omitempty"`
	Mtime     int64    `msgpack:"mtime,omitempty"`
	UID       uint32   `msgpack:"uid,omitempty"`
	GID       uint32   `msgpack:"gid,omitempty"`
	Target    string   `msgpack:"target,omitempty"`
	Data      []byte   `msgpack:"data,omitempty"`
	Inodes    []uint64 `msgpack:"inodes,omitempty"`
	Client    string   `msgpack:"client,omitempty"`
}

// Blob is one whole small file returned by reads.
type Blob struct {
	Inode uint64 `msgpack:"ino"`
	Data  []byte `msgpack:"data"`
}

// Change is one name another mount must drop from its kernel cache.
type Change struct {
	Parent uint64 `msgpack:"parent" json:"parent"`
	Name   string `msgpack:"name" json:"name"`
}

// Result is the success body. Unused fields are omitted.
type Result struct {
	Attr    *Attr    `msgpack:"attr,omitempty"`
	Entries []Dirent `msgpack:"entries,omitempty"`
	Fh      uint64   `msgpack:"fh,omitempty"`
	Data    []byte   `msgpack:"data,omitempty"`
	Target  string   `msgpack:"target,omitempty"`
	Stat    *FSStat  `msgpack:"statfs,omitempty"`
	Written uint32   `msgpack:"written,omitempty"`
	Blobs   []Blob   `msgpack:"blobs,omitempty"`
}

// Attr is the protocol inode's attributes. Ino is the server's number, not st_ino.
type Attr struct {
	Ino   uint64 `msgpack:"ino"`
	Mode  uint32 `msgpack:"mode"`
	Size  uint64 `msgpack:"size"`
	Nlink uint32 `msgpack:"nlink"`
	Uid   uint32 `msgpack:"uid"`
	Gid   uint32 `msgpack:"gid"`
	Atime int64  `msgpack:"atime"`
	Mtime int64  `msgpack:"mtime"`
	Ctime int64  `msgpack:"ctime"`
}

// Dirent is one name from readdir, with the attributes a lookup would return.
// Ino is assigned on this listing if it was new.
type Dirent struct {
	Name string `msgpack:"name"`
	Attr `msgpack:",inline"`
}

// FSStat is the namespace filesystem.
type FSStat struct {
	Blocks  uint64 `msgpack:"blocks"`
	Bfree   uint64 `msgpack:"bfree"`
	Bavail  uint64 `msgpack:"bavail"`
	Files   uint64 `msgpack:"files"`
	Ffree   uint64 `msgpack:"ffree"`
	Bsize   uint32 `msgpack:"bsize"`
	NameLen uint32 `msgpack:"namelen"`
	Frsize  uint32 `msgpack:"frsize"`
}

// Error is the failure body. Code is a Linux errno.
type Error struct {
	Code    uint32 `msgpack:"code"`
	Message string `msgpack:"message"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// CheckNS accepts a namespace of [A-Za-z0-9_-], at most MaxNamespace bytes.
func CheckNS(ns string) error {
	if ns == "" || len(ns) > MaxNamespace {
		return ErrParams
	}
	for i := 0; i < len(ns); i++ {
		c := ns[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			continue
		}
		return ErrParams
	}
	return nil
}

// MarshalRequest encodes one POST. The body has no id.
func MarshalRequest(method string, p Params) ([]byte, error) {
	return marshalCall(0, method, p)
}

// MarshalCall encodes one websocket call. id is the caller's correlation number.
func MarshalCall(id uint64, method string, p Params) ([]byte, error) {
	if id == 0 {
		return nil, fmt.Errorf("marshal %s: %w", method, ErrParams)
	}
	return marshalCall(id, method, p)
}

func marshalCall(id uint64, method string, p Params) ([]byte, error) {
	b, err := msgpack.Marshal(&Request{ID: id, Method: method, Params: p})
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", method, err)
	}
	return b, nil
}

// DecodeRequest reads one call.
func DecodeRequest(r io.Reader) (Request, error) {
	var req Request
	if err := msgpack.NewDecoder(r).Decode(&req); err != nil {
		return Request{}, err
	}
	return req, nil
}

type okBody struct {
	Result Result `msgpack:"result"`
}

type errBody struct {
	Error Error `msgpack:"error"`
}

// MarshalResult encodes a success envelope.
func MarshalResult(res Result) ([]byte, error) {
	return msgpack.Marshal(&okBody{Result: res})
}

// MarshalCallError encodes a filesystem failure envelope.
func MarshalCallError(e Error) ([]byte, error) {
	return msgpack.Marshal(&errBody{Error: e})
}

type envelope struct {
	Result *Result `msgpack:"result"`
	Error  *Error  `msgpack:"error"`
}

// DecodeResponse returns the result, or an *Error when the envelope failed.
func DecodeResponse(b []byte) (Result, error) {
	var env envelope
	if err := msgpack.Unmarshal(b, &env); err != nil {
		return Result{}, fmt.Errorf("response: %w", err)
	}
	if env.Error != nil {
		return Result{}, env.Error
	}
	if env.Result == nil {
		return Result{}, fmt.Errorf("response: empty")
	}
	return *env.Result, nil
}

// Reply is one websocket message for a call id.
// A pending ticket is not the result: the same id is answered again when the call finishes.
type Reply struct {
	ID      uint64
	Result  *Result
	Err     *Error
	Fault   string
	Pending string
	Evict   *Change
	Release string
}

type replyBody struct {
	ID      uint64  `msgpack:"id"`
	Result  *Result `msgpack:"result,omitempty"`
	Error   *Error  `msgpack:"error,omitempty"`
	Fault   string  `msgpack:"fault,omitempty"`
	Pending string  `msgpack:"pending,omitempty"`
	Evict   *Change `msgpack:"evict,omitempty"`
	Release string  `msgpack:"release,omitempty"`
}

// MarshalReply encodes a successful websocket reply.
func MarshalReply(id uint64, res Result) ([]byte, error) {
	return msgpack.Marshal(&replyBody{ID: id, Result: &res})
}

// MarshalReplyError encodes a filesystem failure on the websocket.
func MarshalReplyError(id uint64, e Error) ([]byte, error) {
	return msgpack.Marshal(&replyBody{ID: id, Error: &e})
}

// MarshalFault encodes a failure that is not an errno. The mount treats it as a transport error.
func MarshalFault(id uint64, message string) ([]byte, error) {
	return msgpack.Marshal(&replyBody{ID: id, Fault: message})
}

// MarshalPending encodes the lock-wait notice for a call that has not finished.
func MarshalPending(id uint64, ticket string) ([]byte, error) {
	return msgpack.Marshal(&replyBody{ID: id, Pending: ticket})
}

// DecodeReply reads one websocket reply.
func DecodeReply(b []byte) (Reply, error) {
	var raw replyBody
	if err := msgpack.Unmarshal(b, &raw); err != nil {
		return Reply{}, fmt.Errorf("reply: %w", err)
	}
	return Reply{ID: raw.ID, Result: raw.Result, Err: raw.Error, Fault: raw.Fault, Pending: raw.Pending, Evict: raw.Evict, Release: raw.Release}, nil
}

// Pub is one JSON line on a join stream.
type Pub struct {
	Client  string  `json:"client,omitempty"`
	Evict   *Change `json:"evict,omitempty"`
	Release string  `json:"release,omitempty"`
}

// MarshalPub encodes one join line, including the newline.
func MarshalPub(p Pub) ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// DecodePub reads one join line. The newline is optional.
func DecodePub(line []byte) (Pub, error) {
	var p Pub
	if err := json.Unmarshal(bytes.TrimSpace(line), &p); err != nil {
		return Pub{}, fmt.Errorf("pub: %w", err)
	}
	return p, nil
}

// MarshalPush encodes one websocket push. It has no call id.
func MarshalPush(c Change) ([]byte, error) {
	return msgpack.Marshal(&replyBody{Evict: &c})
}

// MarshalRelease encodes a lock-release push with no call id.
func MarshalRelease(id string) ([]byte, error) {
	return msgpack.Marshal(&replyBody{Release: id})
}
