package wire

import (
	"encoding/binary"
	"fmt"
)

// RawProtocol is the Upgrade token for the large-transfer connection.
const RawProtocol = "fs-relay-raw/1"

const (
	RawReqLen = 40
	RawRepLen = 16
	RawRead   = 1
	RawWrite  = 2
)

const (
	rawReqMagic = "FSRQ"
	rawRepMagic = "FSRP"
)

// RawRequest is one fixed header on the raw connection. A write is followed by Length bytes.
type RawRequest struct {
	Op     uint16
	Inode  uint64
	Fh     uint64
	Offset int64
	Length uint32
}

// RawReply is one fixed reply header. A successful read is followed by Length bytes.
type RawReply struct {
	Errno  uint32
	Length uint32
}

// RawSpare is pipe space left empty so a socket splice can finish.
// A pipe whose capacity equals the byte count returns EAGAIN with bytes still queued.
const RawSpare = 64 << 10

// RawLimits is the size window for the raw path. Min comes from the block size, Max from the pipe.
func RawLimits(blksize, pipeCap int) (min, max int) {
	max = pipeCap
	if max > MaxWrite {
		max = MaxWrite
	}
	if max-RawSpare >= 64<<10 {
		max -= RawSpare
	}
	min = clamp(32*blksize, 64<<10, max)
	return min, max
}

func clamp(v, lo, hi int) int {
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v
}

// PutRawRequest encodes one request header into b, which must be RawReqLen bytes.
func PutRawRequest(b []byte, r RawRequest) error {
	if len(b) < RawReqLen {
		return fmt.Errorf("raw request: %w", ErrParams)
	}
	copy(b[0:4], rawReqMagic)
	binary.LittleEndian.PutUint16(b[4:6], r.Op)
	binary.LittleEndian.PutUint16(b[6:8], 0)
	binary.LittleEndian.PutUint64(b[8:16], r.Inode)
	binary.LittleEndian.PutUint64(b[16:24], r.Fh)
	binary.LittleEndian.PutUint64(b[24:32], uint64(r.Offset))
	binary.LittleEndian.PutUint32(b[32:36], r.Length)
	binary.LittleEndian.PutUint32(b[36:40], 0)
	return nil
}

// ParseRawRequest decodes one request header. A bad magic, op, or reserved field is ErrParams.
func ParseRawRequest(b []byte) (RawRequest, error) {
	if len(b) < RawReqLen || string(b[0:4]) != rawReqMagic {
		return RawRequest{}, fmt.Errorf("raw request: %w", ErrParams)
	}
	if binary.LittleEndian.Uint16(b[6:8]) != 0 || binary.LittleEndian.Uint32(b[36:40]) != 0 {
		return RawRequest{}, fmt.Errorf("raw request: %w", ErrParams)
	}
	op := binary.LittleEndian.Uint16(b[4:6])
	if op != RawRead && op != RawWrite {
		return RawRequest{}, fmt.Errorf("raw request: %w", ErrParams)
	}
	n := binary.LittleEndian.Uint32(b[32:36])
	if n == 0 {
		return RawRequest{}, fmt.Errorf("raw request: %w", ErrParams)
	}
	return RawRequest{
		Op:     op,
		Inode:  binary.LittleEndian.Uint64(b[8:16]),
		Fh:     binary.LittleEndian.Uint64(b[16:24]),
		Offset: int64(binary.LittleEndian.Uint64(b[24:32])),
		Length: n,
	}, nil
}

// PutRawReply encodes one reply header into b, which must be RawRepLen bytes.
func PutRawReply(b []byte, r RawReply) error {
	if len(b) < RawRepLen {
		return fmt.Errorf("raw reply: %w", ErrParams)
	}
	copy(b[0:4], rawRepMagic)
	binary.LittleEndian.PutUint32(b[4:8], r.Errno)
	binary.LittleEndian.PutUint32(b[8:12], r.Length)
	binary.LittleEndian.PutUint32(b[12:16], 0)
	return nil
}

// ParseRawReply decodes one reply header.
func ParseRawReply(b []byte) (RawReply, error) {
	if len(b) < RawRepLen || string(b[0:4]) != rawRepMagic {
		return RawReply{}, fmt.Errorf("raw reply: %w", ErrParams)
	}
	if binary.LittleEndian.Uint32(b[12:16]) != 0 {
		return RawReply{}, fmt.Errorf("raw reply: %w", ErrParams)
	}
	return RawReply{
		Errno:  binary.LittleEndian.Uint32(b[4:8]),
		Length: binary.LittleEndian.Uint32(b[8:12]),
	}, nil
}
