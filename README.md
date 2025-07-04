# mylock

`mylock` is a minimal CLI tool for acquiring a **MySQL advisory lock** and executing a command.  
It is designed to prevent **duplicate executions in Kubernetes CronJobs** or other concurrent environments.

## 🧠 Motivation

When migrating from `crontab` to Kubernetes `CronJob`, ensuring single execution is essential.  
`mylock` uses MySQL's `GET_LOCK()` and `RELEASE_LOCK()` to enforce mutual exclusion without relying on external services like Redis or etcd.

To avoid conflicts with application-level MySQL configuration, `mylock` uses dedicated environment variables prefixed with `MYLOCK_`.

## 🚀 Usage

    mylock --lock-name <name> --timeout <seconds> -- <command> [args...]
    mylock --lock-name-from-command --timeout <seconds> -- <command> [args...]

## 🌱 Required Environment Variables

| Variable         | Required | Example            | Description         |
|------------------|----------|--------------------|---------------------|
| MYLOCK_HOST       | ✅        | 127.0.0.1          | MySQL hostname      |
| MYLOCK_PORT       | ⬜️        | 3306 (default)     | MySQL port          |
| MYLOCK_USER       | ✅        | cronuser           | MySQL username      |
| MYLOCK_PASSWORD   | ✅        | secret             | MySQL password      |
| MYLOCK_DATABASE   | ✅        | jobs               | MySQL database name |

## 📘 Help Output

    mylock - Acquire a MySQL advisory lock and run a command

    Usage:
      mylock --lock-name <name> --timeout <seconds> -- <command> [args...]
      mylock --lock-name-from-command --timeout <seconds> -- <command> [args...]

    Environment Variables (required):
      MYLOCK_HOST         MySQL host (e.g., localhost)
      MYLOCK_PORT         MySQL port (default: 3306)
      MYLOCK_USER         MySQL username
      MYLOCK_PASSWORD     MySQL password
      MYLOCK_DATABASE     MySQL database name

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

      # Or use command-based lock name (useful for dynamic commands)
      mylock --lock-name-from-command --timeout 10 -- ./process_file.sh /data/input.csv

## 🏗 Installation

### Binary (Recommended)

Download the latest release from the [releases page](https://github.com/yammerjp/mylock/releases/latest).

```bash
# Example for Linux amd64
curl -L https://github.com/yammerjp/mylock/releases/latest/download/mylock_Linux_x86_64.tar.gz | tar xz
chmod +x mylock
sudo mv mylock /usr/local/bin/
```

### Go

    go install github.com/yammerjp/mylock/cmd/mylock@latest

### Build from source

    git clone https://github.com/yammerjp/mylock.git
    cd mylock
    go build -o mylock ./cmd/mylock

## ✅ Summary

- Lightweight lock mechanism using MySQL only
- Ideal for Kubernetes CronJob deduplication
- Simple CLI interface with structured configuration
- Only standard Go libraries and `kong` required

## 📦 License

MIT