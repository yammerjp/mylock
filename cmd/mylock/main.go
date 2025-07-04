package main

import (
	"context"
	"fmt"
	"os"

	"github.com/yammerjp/mylock/internal/cli"
	"github.com/yammerjp/mylock/internal/executor"
	"github.com/yammerjp/mylock/internal/locker"
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	// Parse CLI arguments
	cliArgs, err := cli.ParseCLI(args[1:])
	if err != nil {
		// Kong will output help automatically on --help
		// Check if help was requested
		for _, arg := range args {
			if arg == "--help" || arg == "-h" {
				return 0
			}
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return locker.InternalError
	}

	// Initialize locker
	lock, err := locker.NewLocker(cliArgs.Config.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to MySQL: %v\n", err)
		return locker.InternalError
	}
	defer lock.Close()

	// Create executor
	exec := executor.New()

	// Determine lock name
	lockName := cliArgs.LockName
	if cliArgs.LockNameFromCommand {
		lockName = cli.HashCommand(cliArgs.Command)
	}

	// Run command with lock
	ctx := context.Background()
	err = lock.WithLock(ctx, lockName, cliArgs.Timeout, func() error {
		_, execErr := exec.Execute(ctx, cliArgs.Command)
		return execErr
	})

	if err != nil {
		if err == locker.ErrLockTimeout {
			fmt.Fprintf(os.Stderr, "Failed to acquire lock '%s' within %d seconds\n", lockName, cliArgs.Timeout)
			return locker.LockTimeout
		}
		// Check if it's an execution error with specific exit code
		exitCode := executor.GetExitCode(err)
		if exitCode >= 0 {
			return exitCode
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return locker.InternalError
	}

	return 0
}
