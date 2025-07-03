package main

import (
	"context"
	"fmt"
	"os"

	"github.com/yammerjp/mylock/internal/cli"
	"github.com/yammerjp/mylock/internal/config"
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
		return locker.InternalError
	}

	// Create database configuration
	cfg := config.Config{
		Host:     cliArgs.Config.Host,
		Port:     cliArgs.Config.Port,
		User:     cliArgs.Config.User,
		Password: cliArgs.Config.Password,
		Database: cliArgs.Config.Database,
	}

	// Initialize locker
	lock, err := locker.NewLocker(cfg.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to MySQL: %v\n", err)
		return locker.InternalError
	}
	defer lock.Close()

	// Create executor
	exec := executor.New()

	// Run command with lock
	ctx := context.Background()
	err = lock.WithLock(ctx, cliArgs.LockName, cliArgs.Timeout, func() error {
		exitCode, execErr := exec.Execute(ctx, cliArgs.Command)
		if execErr != nil {
			// If the command was found but exited with non-zero status,
			// we want to return that exit code
			if exitCode > 0 && exitCode <= 127 {
				// Store exit code for later use
				os.Exit(exitCode)
			}
			return execErr
		}
		return nil
	})

	if err != nil {
		if err == locker.ErrLockTimeout {
			fmt.Fprintf(os.Stderr, "Failed to acquire lock '%s' within %d seconds\n", cliArgs.LockName, cliArgs.Timeout)
			return locker.LockTimeout
		}
		// Check if it's an execution error with specific exit code
		exitCode := executor.GetExitCode(err)
		if exitCode > 0 && exitCode <= 127 {
			return exitCode
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return locker.InternalError
	}

	return 0
}
