package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	// DefaultMySQLPort is the default port for MySQL/MariaDB connections
	DefaultMySQLPort = 3306
	// MinPort is the minimum valid port number
	MinPort = 1
	// MaxPort is the maximum valid port number
	MaxPort = 65535
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

func NewConfig() (Config, error) {
	var cfg Config
	var err error

	cfg.Host = os.Getenv("MYLOCK_HOST")
	if cfg.Host == "" {
		return cfg, fmt.Errorf("MYLOCK_HOST environment variable is required")
	}

	portStr := os.Getenv("MYLOCK_PORT")
	if portStr == "" {
		cfg.Port = DefaultMySQLPort
	} else {
		cfg.Port, err = strconv.Atoi(portStr)
		if err != nil {
			return cfg, fmt.Errorf("invalid MYLOCK_PORT: %w", err)
		}
		if cfg.Port < MinPort || cfg.Port > MaxPort {
			return cfg, fmt.Errorf("MYLOCK_PORT must be between %d and %d", MinPort, MaxPort)
		}
	}

	cfg.User = os.Getenv("MYLOCK_USER")
	if cfg.User == "" {
		return cfg, fmt.Errorf("MYLOCK_USER environment variable is required")
	}

	cfg.Password = os.Getenv("MYLOCK_PASSWORD")
	if cfg.Password == "" {
		return cfg, fmt.Errorf("MYLOCK_PASSWORD environment variable is required")
	}

	cfg.Database = os.Getenv("MYLOCK_DATABASE")
	if cfg.Database == "" {
		return cfg, fmt.Errorf("MYLOCK_DATABASE environment variable is required")
	}

	return cfg, nil
}

func (c Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		c.User, c.Password, c.Host, c.Port, c.Database)
}
