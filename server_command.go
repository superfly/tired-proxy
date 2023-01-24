//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/mattn/go-shellwords"
)

type ServerCommand struct {
	cmd    *exec.Cmd  // Interface with the server command
	exitCh chan error // is raised when the server command is closed
}

func StartServerCommand(ctx context.Context, command string) (*ServerCommand, error) {
	if command == "" {
		return nil, fmt.Errorf("empty command string given")
	}

	args, err := shellwords.Parse(command)
	if err != nil {
		return nil, fmt.Errorf("can not parse server command '%s': %w", command, err)
	}

	sv := &ServerCommand{
		exec.CommandContext(ctx, args[0], args[1:]...),
		make(chan error),
	}
	sv.cmd.Stdout = os.Stdout
	sv.cmd.Stderr = os.Stderr
	fmt.Printf("Starting subprocess as server command: %s %v\n", args[0], args[1:])
	if err := sv.cmd.Start(); err != nil {
		return nil, fmt.Errorf("can not start server command: %w", err)
	}

	go func() {
		sv.exitCh <- sv.cmd.Wait()
	}()

	return sv, nil
}
