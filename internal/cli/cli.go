package cli

import (
	"fmt"
	"io"

	"github.com/alecthomas/kong"
)

type CLI struct {
	LockName string   `kong:"required,help:'A unique name for the advisory lock.'"`
	Timeout  int      `kong:"required,help:'Max seconds to wait for the lock.'"`
	Command  []string `kong:"arg,required,name:'command',help:'Command to run once the lock is acquired.'"`
	Config
}

type Config struct {
	Host     string `kong:"env='MYLOCK_HOST',required,help:'MySQL host'"`
	Port     int    `kong:"env='MYLOCK_PORT',default='3306',help:'MySQL port'"`
	User     string `kong:"env='MYLOCK_USER',required,help:'MySQL username'"`
	Password string `kong:"env='MYLOCK_PASSWORD',required,help:'MySQL password'"`
	Database string `kong:"env='MYLOCK_DATABASE',required,help:'MySQL database name'"`
}

func ParseCLI(args []string) (CLI, error) {
	var cli CLI
	parser, err := kong.New(&cli,
		kong.Name("mylock"),
		kong.Description("Acquire a MySQL advisory lock and run a command"),
		kong.UsageOnError(),
		kong.Exit(func(int) {}), // Prevent os.Exit during testing
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}),
		kong.Vars{
			"version": "1.0.0",
		},
	)
	if err != nil {
		return cli, err
	}

	// Suppress help output for testing
	ctx, err := parser.Parse(args)
	if err != nil {
		return cli, err
	}

	if ctx.Command() == "help" {
		return cli, fmt.Errorf("help requested")
	}

	return cli, nil
}

func (c CLI) PrintHelp(w io.Writer) {
	fmt.Fprintf(w, `mylock - Acquire a MySQL advisory lock and run a command

Usage:
  mylock --lock-name <name> --timeout <seconds> -- <command> [args...]

Environment Variables (required):
  MYLOCK_HOST         MySQL host (e.g., localhost)
  MYLOCK_PORT         MySQL port (default: 3306)
  MYLOCK_USER         MySQL username
  MYLOCK_PASSWORD     MySQL password
  MYLOCK_DATABASE     MySQL database name

Options:
  --lock-name         Required. A unique name for the advisory lock.
  --timeout           Required. Max seconds to wait for the lock.
  --help              Show this help message.

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
}

func (c Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

