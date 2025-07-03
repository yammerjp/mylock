package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

type Executor struct {
	sigChan chan os.Signal
}

func New() *Executor {
	return &Executor{
		sigChan: make(chan os.Signal, 1),
	}
}

func (e *Executor) Execute(ctx context.Context, command []string) (int, error) {
	if len(command) == 0 {
		return -1, errors.New("command is required")
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// Pass through stdin, stdout, stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set up signal handling
	signal.Notify(e.sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(e.sigChan)

	// Start the command
	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("failed to start command: %w", err)
	}

	// Wait for command completion or signal
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Context cancelled
		if err := cmd.Process.Kill(); err != nil {
			return -1, fmt.Errorf("failed to kill process: %w", err)
		}
		return -1, ctx.Err()
	case sig := <-e.sigChan:
		// Forward signal to child process
		if err := cmd.Process.Signal(sig); err != nil {
			return -1, fmt.Errorf("failed to forward signal: %w", err)
		}
		// Wait for process to handle the signal
		err := <-done
		return GetExitCode(err), err
	case err := <-done:
		// Command completed
		return GetExitCode(err), err
	}
}

func GetExitCode(err error) int {
	if err == nil {
		return 0
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
		// Fallback if we can't get the exact exit status
		return 1
	}

	return -1
}
