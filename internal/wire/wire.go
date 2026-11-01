// Package wire is the FlatBuffers contract shared by the server and the mount.
// It does not speak HTTP, FUSE, or the filesystem.
package wire

import (
	"errors"
	"fmt"
	"io"
	"time"
)

// ContentType is the only body type the server accepts.
const ContentType = "application/x-flatbuffers"

// TokenHeader carries the shared secret. TokenScheme is the word in front of it.
const (
	TokenHeader = "Authorization"
	TokenScheme = "Bearer"
)

// Limits from SPEC: a namespace is bounded, and one FUSE write is one POST.
const (
	MaxNamespace = 64
	MaxWrite     = 1 << 20
	MaxBody      = MaxWrite + 64*1024
	// MaxCopy bounds one copyfilerange. Written is a uint32, so the count fits.
	MaxCopy = 1 << 30
	// MaxReads and MaxPrefetch bound one reads call: a window of small files.
	MaxReads    = 32
	MaxPrefetch = 64 << 10
	// Heartbeat is how often a live stream is touched so a quiet peer is noticed.
	Heartbeat = 2 * time.Second
	// MissedHeartbeats is how many intervals a mount waits before the stream is stalled.
	MissedHeartbeats = 3
	// Stall is MissedHeartbeats intervals with nothing received.
	Stall = Heartbeat * MissedHeartbeats
)

// Method names are the go-fuse methods, lowercased.
const (
	Lookup        = "lookup"
	Getattr       = "getattr"
	Setattr       = "setattr"
	Access        = "access"
	Statfs        = "statfs"
	Mkdir         = "mkdir"
	Create        = "create"
	Open          = "open"
	Read          = "read"
	Reads         = "reads"
	Readdirs      = "readdirs"
	Write         = "write"
	Flush         = "flush"
	Fsync         = "fsync"
	Release       = "release"
	Opendir       = "opendir"
	Readdir       = "readdir"
	Releasedir    = "releasedir"
	Unlink        = "unlink"
	Rmdir         = "rmdir"
	Rename        = "rename"
	Symlink       = "symlink"
	Readlink      = "readlink"
	Join          = "join"
	CopyFileRange = "copyfilerange"
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
	ID     uint64
	Method string
	Params Params
}

// Params carries the kernel arguments plus the namespace.
// A setattr to zero still works because Valid says which fields count.
type Params struct {
	Namespace string
	Inode     uint64
	NewInode  uint64
	Name      string
	NewName   string
	Fh        uint64
	Mode      uint32
	Flags     uint32
	Valid     uint32
	Mask      uint32
	Offset    int64
	Size      uint64
	Atime     int64
	Mtime     int64
	UID       uint32
	GID       uint32
	Target    string
	Data      []byte
	Inodes    []uint64
	Client    string
	FhOut     uint64
	OffsetOut int64
}

// Blob is one whole small file returned by reads.
type Blob struct {
	Inode uint64
	Data  []byte
}

// Change is one name another mount must drop from its kernel cache.
type Change struct {
	Parent uint64
	Name   string
}

// Result is the success body. Empty strings and empty byte vectors are absent.
// Numeric fields are present, including zero.
type Result struct {
	Attr     *Attr
	Entries  []Dirent
	Fh       uint64
	Data     []byte
	Target   string
	Stat     *FSStat
	Written  uint32
	Blobs    []Blob
	Listings []Listing
}

// Listing is one directory returned by readdirs.
type Listing struct {
	Inode   uint64
	Entries []Dirent
}

// Attr is the protocol inode's attributes. Ino is the server's number, not st_ino.
type Attr struct {
	Ino   uint64
	Mode  uint32
	Size  uint64
	Nlink uint32
	Uid   uint32
	Gid   uint32
	Atime int64
	Mtime int64
	Ctime int64
}

// Dirent is one name from readdir, with the attributes a lookup would return.
// Ino is assigned on this listing if it was new.
type Dirent struct {
	Name string
	Attr
}

// FSStat is the namespace filesystem.
type FSStat struct {
	Blocks  uint64
	Bfree   uint64
	Bavail  uint64
	Files   uint64
	Ffree   uint64
	Bsize   uint32
	NameLen uint32
	Frsize  uint32
}

// Error is the failure body. Code is a Linux errno on a filesystem call,
// and the HTTP status when the request is refused before a call.
type Error struct {
	Code    uint32 `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// ErrTokenShort is a secret shorter than eight characters.
var ErrTokenShort = errors.New("token must be at least 8 characters")

// ErrTokenChars is a secret outside the URL-safe alphabet.
var ErrTokenChars = errors.New("token may contain only letters, digits, hyphens, and underscores")

// CheckToken accepts a secret of at least eight characters. The alphabet is
// letters, digits, hyphen, and underscore, so the value needs no encoding in
// a header. The comparison is case-sensitive.
func CheckToken(token string) error {
	if len(token) < 8 {
		return ErrTokenShort
	}
	if !urlSafe(token) {
		return ErrTokenChars
	}
	return nil
}

// CheckNS accepts a namespace of [A-Za-z0-9_-], at most MaxNamespace bytes.
func CheckNS(ns string) error {
	if ns == "" || len(ns) > MaxNamespace || !urlSafe(ns) {
		return ErrParams
	}
	return nil
}

func urlSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
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
	b, err := encodeFB(fbMsg{ID: id, Method: method, Params: &p})
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", method, err)
	}
	return b, nil
}

// DecodeRequest reads one call from a stream.
func DecodeRequest(r io.Reader) (Request, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxBody+1))
	if err != nil {
		return Request{}, err
	}
	return DecodeRequestBuffer(b)
}

// DecodeRequestBuffer decodes a call that is already in memory.
// The websocket reader has already copied the frame once.
func DecodeRequestBuffer(b []byte) (Request, error) {
	if len(b) > MaxBody {
		return Request{}, ErrParams
	}
	return decodeFBRequest(b)
}

// MarshalResult encodes a success envelope.
func MarshalResult(res Result) ([]byte, error) {
	return encodeFB(fbMsg{Result: &res})
}

// MarshalCallError encodes a filesystem failure envelope.
func MarshalCallError(e Error) ([]byte, error) {
	return encodeFB(fbMsg{Err: &e})
}

// DecodeResponse returns the result, or an *Error when the envelope failed.
func DecodeResponse(b []byte) (Result, error) {
	rp, err := decodeFBReply(b)
	if err != nil {
		return Result{}, fmt.Errorf("response: %w", err)
	}
	if rp.Err != nil {
		return Result{}, rp.Err
	}
	if rp.Result == nil {
		return Result{}, fmt.Errorf("response: empty")
	}
	return *rp.Result, nil
}

// Reply is one websocket message for a call id.
// A pending ticket is not the result: the same id is answered again when the call finishes.
// Client is set on the first message, which has no call id.
type Reply struct {
	ID      uint64
	Client  string
	Result  *Result
	Err     *Error
	Fault   string
	Pending string
	Evict   *Change
	Release string
}

// MarshalReply encodes a successful websocket reply.
func MarshalReply(id uint64, res Result) ([]byte, error) {
	return encodeFB(fbMsg{ID: id, Result: &res})
}

// MarshalReplyError encodes a filesystem failure on the websocket.
func MarshalReplyError(id uint64, e Error) ([]byte, error) {
	return encodeFB(fbMsg{ID: id, Err: &e})
}

// MarshalFault encodes a failure that is not an errno. The mount treats it as a transport error.
func MarshalFault(id uint64, message string) ([]byte, error) {
	return encodeFB(fbMsg{ID: id, Fault: message})
}

// MarshalPending encodes the lock-wait notice for a call that has not finished.
func MarshalPending(id uint64, ticket string) ([]byte, error) {
	return encodeFB(fbMsg{ID: id, Pending: ticket})
}

// DecodeReply reads one websocket reply.
func DecodeReply(b []byte) (Reply, error) {
	return decodeFBReply(b)
}

// Pub is one event on the join stream.
type Pub struct {
	Client  string
	Evict   *Change
	Release string
}

// MarshalPub encodes one join event. The stream frames it with WriteFrame.
func MarshalPub(p Pub) ([]byte, error) {
	return encodeFB(fbMsg{Client: p.Client, Evict: p.Evict, Release: p.Release})
}

// DecodePub reads one join event.
func DecodePub(b []byte) (Pub, error) {
	return decodeFBPub(b)
}

// MarshalPush encodes one websocket push. It has no call id.
func MarshalPush(c Change) ([]byte, error) {
	return encodeFB(fbMsg{Evict: &c})
}

// MarshalRelease encodes a lock-release push with no call id.
func MarshalRelease(id string) ([]byte, error) {
	return encodeFB(fbMsg{Release: id})
}
