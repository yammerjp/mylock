package cli

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/yammerjp/mylock/internal/config"
)

type CLI struct {
	LockName            string   `kong:"optional,help:'A unique name for the advisory lock.'"`
	LockNameFromCommand bool     `kong:"optional,help:'Generate lock name from command hash.'"`
	Timeout             int      `kong:"required,help:'Max seconds to wait for the lock.'"`
	Command             []string `kong:"arg,required,name:'command',help:'Command to run once the lock is acquired.'"`
	// Config is populated from environment variables, not from CLI flags
	Config config.Config `kong:"-"`
}

func ParseCLI(args []string) (CLI, error) {
	var cli CLI

	// Parse config from environment first
	cfg, err := config.NewConfig()
	if err != nil {
		// For help, we don't need valid config
		if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
			// Continue with empty config for help
		} else {
			return cli, err
		}
	} else {
		cli.Config = cfg
	}

	parser, err := kong.New(&cli,
		kong.Name("mylock"),
		kong.Description("Acquire a MySQL advisory lock and run a command"),
		kong.UsageOnError(),
		kong.Exit(func(int) {}), // Prevent os.Exit during testing
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: false,
			Summary: false,
		}),
		kong.Help(helpFormatter),
		kong.Vars{
			"version": "1.0.0",
		},
	)
	if err != nil {
		return cli, err
	}

	ctx, err := parser.Parse(args)
	if err != nil {
		return cli, err
	}

	if ctx.Command() == "help" {
		return cli, fmt.Errorf("help requested")
	}

	// Validate that exactly one of lock-name or lock-name-from-command is specified
	if cli.LockName == "" && !cli.LockNameFromCommand {
		return cli, fmt.Errorf("either --lock-name or --lock-name-from-command must be specified")
	}
	if cli.LockName != "" && cli.LockNameFromCommand {
		return cli, fmt.Errorf("cannot specify both --lock-name and --lock-name-from-command")
	}

	return cli, nil
}

func helpFormatter(options kong.HelpOptions, ctx *kong.Context) error {
	w := os.Stdout
	if options.NoExpandSubcommands {
		// This is for error help, use stderr
		w = os.Stderr
	}

	fmt.Fprintf(w, `mylock - Acquire a MySQL advisory lock and run a command

Usage:
  mylock --lock-name <name> --timeout <seconds> -- <command> [args...]
  mylock --lock-name-from-command --timeout <seconds> -- <command> [args...]

Environment Variables:
  MYLOCK_HOST         MySQL host (required, e.g., localhost)
  MYLOCK_PORT         MySQL port (optional, default: 3306)
  MYLOCK_USER         MySQL username (required)
  MYLOCK_PASSWORD     MySQL password (optional, empty allowed)
  MYLOCK_DATABASE     MySQL database name (required)

Options:
  --lock-name              A unique name for the advisory lock.
  --lock-name-from-command Generate lock name from command hash.
  --timeout                Required. Max seconds to wait for the lock.
  --help                   Show this help message.

Note: Either --lock-name or --lock-name-from-command must be specified (but not both).

Behavior:
  - Connects to MySQL using the environment variables above.
  - Acquires a named advisory lock using GET_LOCK().
  - If the lock is acquired within the timeout, runs the given command.
  - stdin/stdout/stderr are passed through. Signals (SIGINT, SIGTERM) are forwarded.
  - Releases the lock using RELEASE_LOCK() after execution or interruption.

Exit Codes:
   0–127   Exit code from the executed command
   200     Failed to acquire lock within timeout
   201     Internal error in mylock (e.g., MySQL connection failure)

Example:
  MYLOCK_HOST=127.0.0.1 \
  MYLOCK_PORT=3306 \
  MYLOCK_USER=cronuser \
  MYLOCK_PASSWORD=secret \
  MYLOCK_DATABASE=jobs \
  mylock --lock-name daily-report --timeout 10 -- ./generate_report.sh
`)
	return nil
}
