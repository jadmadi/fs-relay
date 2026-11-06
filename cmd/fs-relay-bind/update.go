package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// bindSubcommand returns "update", "version", or "help" when invoked as a bare
// single-word command with no flags, or "" when arguments represent a mountpoint.
func bindSubcommand(fs *flag.FlagSet) string {
	if fs.NFlag() != 0 || fs.NArg() != 1 {
		return ""
	}
	switch fs.Arg(0) {
	case "update", "version", "help":
		return fs.Arg(0)
	default:
		return ""
	}
}

func dispatchBindCLI(flags *flag.FlagSet, showVersion, doUpdate bool) (bool, error) {
	sub := bindSubcommand(flags)
	if sub == "help" {
		fmt.Print(usageText(flags))
		return true, nil
	}
	if showVersion || sub == "version" {
		fmt.Println(versionLine("fs-relay-bind"))
		return true, nil
	}
	if doUpdate || sub == "update" {
		return true, runBindUpdate()
	}
	return false, nil
}

func runBindUpdate() error {
	if self, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), "fs-relay")
		if info, err := os.Stat(sibling); err == nil && info.Mode().IsRegular() {
			cmd := exec.Command(sibling, "update")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}
	if path, err := exec.LookPath("fs-relay"); err == nil {
		cmd := exec.Command(path, "update")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return fmt.Errorf("run \"fs-relay update\" to upgrade both fs-relay and fs-relay-bind")
}
