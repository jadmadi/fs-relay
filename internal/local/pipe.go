package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"syscall"

	"golang.org/x/sys/unix"
)

// Pipe is one kernel pipe used to move bytes with splice.
type Pipe struct {
	r, w int
	size int
}

// NewPipe creates a pipe of at least size bytes, or the largest the kernel allows.
func NewPipe(size int) (*Pipe, error) {
	var fds [2]int
	if err := unix.Pipe2(fds[:], unix.O_CLOEXEC); err != nil {
		return nil, fmt.Errorf("pipe: %w", err)
	}
	got, err := unix.FcntlInt(uintptr(fds[0]), unix.F_SETPIPE_SZ, size)
	if err != nil {
		got, err = unix.FcntlInt(uintptr(fds[0]), unix.F_GETPIPE_SZ, 0)
	}
	if err != nil || got <= 0 {
		unix.Close(fds[0])
		unix.Close(fds[1])
		if err == nil {
			err = fmt.Errorf("pipe size")
		}
		return nil, fmt.Errorf("pipe: %w", err)
	}
	return &Pipe{r: fds[0], w: fds[1], size: got}, nil
}

// PipeCapacity reports the pipe size the kernel grants for want, falling back to 64 KiB.
func PipeCapacity(want int) int {
	p, err := NewPipe(want)
	if err != nil {
		return 64 << 10
	}
	defer p.Close()
	return p.Cap()
}

// ReadFd is the read end.
func (p *Pipe) ReadFd() int { return p.r }

// WriteFd is the write end.
func (p *Pipe) WriteFd() int { return p.w }

// Cap is the pipe's capacity in bytes.
func (p *Pipe) Cap() int { return p.size }

// Reset drops any buffered bytes and keeps the same capacity.
func (p *Pipe) Reset() error {
	size := p.size
	p.Close()
	n, err := NewPipe(size)
	if err != nil {
		return err
	}
	*p = *n
	return nil
}

// Close closes both ends. A second call is a no-op.
func (p *Pipe) Close() error {
	var err error
	if p.r >= 0 {
		err = unix.Close(p.r)
		p.r = -1
	}
	if p.w >= 0 {
		if e := unix.Close(p.w); err == nil {
			err = e
		}
		p.w = -1
	}
	return err
}

// SpliceMove moves exactly n bytes from rfd to wfd. The fds are nonblocking.
// waitFd is the socket, and waitEvents is POLLIN or POLLOUT.
// poll(2) is level-triggered, so bytes already queued are not missed.
func SpliceMove(ctx context.Context, rfd, wfd, n, waitFd int, waitEvents int16) error {
	got := 0
	stall := 0
	for got < n {
		if err := ctx.Err(); err != nil {
			return err
		}
		c, err := unix.Splice(rfd, nil, wfd, nil, n-got, unix.SPLICE_F_MOVE|unix.SPLICE_F_NONBLOCK)
		if c > 0 {
			got += int(c)
			stall = 0
		}
		if got >= n {
			return nil
		}
		if err == nil {
			if c == 0 {
				return io.ErrUnexpectedEOF
			}
			continue
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
		ready, err := waitSocket(ctx, waitFd, waitEvents)
		if err != nil {
			return err
		}
		if !ready {
			stall = 0
			continue
		}
		stall++
		if stall > 8 {
			return fmt.Errorf("splice: stalled")
		}
	}
	return nil
}

func waitSocket(ctx context.Context, fd int, ev int16) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	p := []unix.PollFd{{Fd: int32(fd), Events: ev | unix.POLLERR | unix.POLLHUP}}
	_, err := unix.Poll(p, 100)
	if errors.Is(err, syscall.EINTR) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	re := p[0].Revents
	if re&(unix.POLLIN|unix.POLLOUT) != 0 {
		return true, nil
	}
	if re&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		return false, io.ErrUnexpectedEOF
	}
	return false, nil
}
