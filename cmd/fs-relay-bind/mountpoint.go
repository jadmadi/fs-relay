package main

import (
	"errors"
	"fmt"
	"os"
)

// checkMountpoint refuses a path that is missing, not a directory, or not
// empty unless the caller forces it. It runs before any network connection.
func checkMountpoint(path string, force bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("mountpoint %s does not exist", path)
		}
		return fmt.Errorf("mountpoint %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("mountpoint %s is not a directory", path)
	}
	if force {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("mountpoint %s: %w", path, err)
	}
	if len(entries) > 0 && !force {
		return fmt.Errorf("mountpoint %s is not empty; use -force-existing-mnt", path)
	}
	return nil
}
