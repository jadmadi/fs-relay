package main

import (
	"context"
	"errors"
	"syscall"

	"fs-relay/internal/wire"
)

func toErrno(err error) syscall.Errno {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return syscall.EINTR
	}
	var w *wire.Error
	if errors.As(err, &w) {
		if w.Code == 0 {
			return syscall.EIO
		}
		return syscall.Errno(w.Code)
	}
	return syscall.EIO
}
