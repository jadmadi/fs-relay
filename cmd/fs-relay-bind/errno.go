package main

import (
	"errors"
	"syscall"

	"fs-relay/internal/wire"
)

func toErrno(err error) syscall.Errno {
	if err == nil {
		return 0
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
