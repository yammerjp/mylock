package cli

import (
	"os"
	"reflect"
	"testing"

	"github.com/yammerjp/mylock/internal/config"
)

func TestParseCLI(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		envVars map[string]string
		want    CLI
		wantErr bool
	}{
		{
			name: "valid arguments with all required fields",
			args: []string{"--lock-name", "test-lock", "--timeout", "30", "--", "echo", "hello"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			want: CLI{
				LockName: "test-lock",
				Timeout:  30,
				Command:  []string{"echo", "hello"},
				Config: config.Config{
					Host:     "localhost",
					Port:     3306,
					User:     "testuser",
					Password: "testpass",
					Database: "testdb",
				},
			},
			wantErr: false,
		},
		{
			name: "valid arguments with custom port",
			args: []string{"--lock-name", "another-lock", "--timeout", "10", "--", "ls", "-la"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "db.example.com",
				"MYLOCK_PORT":     "3307",
				"MYLOCK_USER":     "dbuser",
				"MYLOCK_PASSWORD": "dbpass",
				"MYLOCK_DATABASE": "mydb",
			},
			want: CLI{
				LockName: "another-lock",
				Timeout:  10,
				Command:  []string{"ls", "-la"},
				Config: config.Config{
					Host:     "db.example.com",
					Port:     3307,
					User:     "dbuser",
					Password: "dbpass",
					Database: "mydb",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with lock-name-from-command",
			args: []string{"--lock-name-from-command", "--timeout", "30", "--", "echo", "hello"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			want: CLI{
				LockNameFromCommand: true,
				Timeout:             30,
				Command:             []string{"echo", "hello"},
				Config: config.Config{
					Host:     "localhost",
					Port:     3306,
					User:     "testuser",
					Password: "testpass",
					Database: "testdb",
				},
			},
			wantErr: false,
		},
		{
			name: "both lock-name and lock-name-from-command should fail",
			args: []string{"--lock-name", "test", "--lock-name-from-command", "--timeout", "30", "--", "echo", "hello"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
		{
			name: "missing both lock-name and lock-name-from-command",
			args: []string{"--timeout", "30", "--", "echo", "hello"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
		{
			name: "missing timeout",
			args: []string{"--lock-name", "test-lock", "--", "echo", "hello"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
		{
			name: "missing command",
			args: []string{"--lock-name", "test-lock", "--timeout", "30"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
		{
			name: "missing environment variable",
			args: []string{"--lock-name", "test-lock", "--timeout", "30", "--", "echo", "hello"},
			envVars: map[string]string{
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true,
		},
		{
			name: "help flag",
			args: []string{"--help"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "testuser",
				"MYLOCK_PASSWORD": "testpass",
				"MYLOCK_DATABASE": "testdb",
			},
			wantErr: true, // kong prints help and returns error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and clear environment
			oldEnv := make(map[string]string)
			for _, key := range []string{"MYLOCK_HOST", "MYLOCK_PORT", "MYLOCK_USER", "MYLOCK_PASSWORD", "MYLOCK_DATABASE"} {
				oldEnv[key] = os.Getenv(key)
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

			got, err := ParseCLI(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCLI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseCLI() = %v, want %v", got, tt.want)
			}
		})
	}
}
