package config

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "all required environment variables set",
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			want: Config{
				Host:     "localhost",
				Port:     3306,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
			},
			wantErr: false,
		},
		{
			name: "custom port specified",
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_PORT":     "3307",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			want: Config{
				Host:     "localhost",
				Port:     3307,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
			},
			wantErr: false,
		},
		{
			name: "missing MYLOCK_HOST",
			envVars: map[string]string{
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
		{
			name: "missing MYLOCK_USER",
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
		{
			name: "missing MYLOCK_PASSWORD",
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
		{
			name: "missing MYLOCK_DATABASE",
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
			},
			wantErr: true,
		},
		{
			name: "invalid port number",
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_PORT":     "invalid",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save current environment
			oldEnv := make(map[string]string)
			for key := range tt.envVars {
				oldEnv[key] = os.Getenv(key)
			}
			// Also save for keys that might not be in envVars but need to be cleared
			for _, key := range []string{"MYLOCK_HOST", "MYLOCK_PORT", "MYLOCK_USER", "MYLOCK_PASSWORD", "MYLOCK_DATABASE"} {
				if _, ok := oldEnv[key]; !ok {
					oldEnv[key] = os.Getenv(key)
				}
			}

			// Clear environment
			for key := range oldEnv {
				os.Unsetenv(key)
			}

			// Set test environment
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Restore environment after test
			defer func() {
				for key, value := range oldEnv {
					if value == "" {
						os.Unsetenv(key)
					} else {
						os.Setenv(key, value)
					}
				}
			}()

			got, err := NewConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NewConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_DSN(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name: "standard configuration",
			config: Config{
				Host:     "localhost",
				Port:     3306,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
			},
			want: "testuser:testpass@tcp(localhost:3306)/testdb",
		},
		{
			name: "custom port",
			config: Config{
				Host:     "db.example.com",
				Port:     3307,
				User:     "appuser",
				Password: "secret123",
				Database: "myapp",
			},
			want: "appuser:secret123@tcp(db.example.com:3307)/myapp",
		},
		{
			name: "special characters in password",
			config: Config{
				Host:     "localhost",
				Port:     3306,
				User:     "user",
				Password: "p@ss:word/123",
				Database: "db",
			},
			want: "user:p@ss:word/123@tcp(localhost:3306)/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.DSN(); got != tt.want {
				t.Errorf("Config.DSN() = %v, want %v", got, tt.want)
			}
		})
	}
}