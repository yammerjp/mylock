package cli

import (
	"testing"
)

func TestHashCommand(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		want    string
	}{
		{
			name:    "simple command",
			command: []string{"echo", "hello"},
			want:    "mylock-6d9387f23a79ea8f3b0f1b033f7c1990e31eea0d290d3a889e37ae698",
		},
		{
			name:    "command with arguments",
			command: []string{"ls", "-la", "/tmp"},
			want:    "mylock-0d2556cdcd8dd78482b40a429103b45bd4700c4ca9e614240b01532e2",
		},
		{
			name:    "same command produces same hash",
			command: []string{"echo", "hello"},
			want:    "mylock-6d9387f23a79ea8f3b0f1b033f7c1990e31eea0d290d3a889e37ae698",
		},
		{
			name:    "order matters",
			command: []string{"hello", "echo"},
			want:    "mylock-b82a23c9ccfa870c5d47dbf7ff1b34301113968cf8b43902a46f93300",
		},
		{
			name:    "empty command",
			command: []string{},
			want:    "mylock-e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7",
		},
		{
			name:    "single command",
			command: []string{"date"},
			want:    "mylock-0e87632cd46bd4907c516317eb6d81fe0f921a23c7643018f21292894",
		},
		{
			name:    "complex shell command",
			command: []string{"sh", "-c", "echo 'hello' | grep 'ell'"},
			want:    "mylock-27786956280528d352a9fa12f87630b0a8124ef800a85f0a38bc5d2c7",
		},
		{
			name:    "max length hash should be truncated to 64 chars",
			command: []string{"very", "long", "command", "with", "many", "arguments", "that", "would", "produce", "a", "very", "long", "hash"},
			want:    "mylock-a5c9a35c6ccee6676961539115787b9ff19ecc7091d9537817b4d4a64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashCommand(tt.command)
			if got != tt.want {
				t.Errorf("HashCommand() = %v, want %v", got, tt.want)
			}
			// Ensure the hash is no longer than 64 characters (MySQL limit)
			if len(got) > 64 {
				t.Errorf("HashCommand() produced hash longer than 64 chars: %d", len(got))
			}
		})
	}
}
