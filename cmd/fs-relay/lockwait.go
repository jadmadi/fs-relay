package main

import "fs-relay/internal/wire"

func isLock(method string) bool {
	switch method {
	case wire.Getlk, wire.Setlk, wire.Setlkw:
		return true
	}
	return false
}
