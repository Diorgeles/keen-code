package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigMissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Version != ConfigVersion {
		t.Fatalf("Version = %d, want %d", cfg.Version, ConfigVersion)
	}
	if len(cfg.Servers) != 0 {
		t.Fatalf("Servers length = %d, want 0", len(cfg.Servers))
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:    "invalid json",
			config:  `{`,
			wantErr: "parse MCP config",
		},
		{
			name: "unsupported version",
			config: `{
				"version": 2,
				"servers": {}
			}`,
			wantErr: "unsupported MCP config version",
		},
		{
			name: "invalid server name",
			config: `{
				"version": 1,
				"servers": {
					"bad name": {"transport": "streamable_http", "url": "https://example.com/mcp", "auth": {"type": "none"}}
				}
			}`,
			wantErr: "bad name",
		},
		{
			name: "invalid url",
			config: `{
				"version": 1,
				"servers": {
					"bad": {"transport": "streamable_http", "url": "ftp://example.com/mcp", "auth": {"type": "none"}}
				}
			}`,
			wantErr: "url must use http or https",
		},
		{
			name: "missing api key",
			config: `{
				"version": 1,
				"servers": {
					"bad": {"transport": "streamable_http", "url": "https://example.com/mcp", "auth": {"type": "api_key"}}
				}
			}`,
			wantErr: "api_key auth requires key",
		},
		{
			name: "missing stdio command",
			config: `{
				"version": 1,
				"servers": {
					"bad": {"transport": "stdio"}
				}
			}`,
			wantErr: "stdio transport requires command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			path := DefaultConfigPath()
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tt.config), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadConfig()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadConfig() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigValidTransports(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := DefaultConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{
		"version": 1,
		"servers": {
			"http": {"transport": "streamable_http", "url": "https://example.com/mcp", "auth": {"type": "none"}},
			"stdio": {"transport": "stdio", "command": "example-mcp", "args": ["--flag"], "env": {"TOKEN": "value"}}
		}
	}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("Servers length = %d, want 2", len(cfg.Servers))
	}
}

func TestAuthConfigDefaults(t *testing.T) {
	defaulted := (AuthConfig{Type: AuthAPIKey, Key: "secret"}).withDefaults()
	if defaulted.Header != "Authorization" || defaulted.Scheme != "Bearer" {
		t.Fatalf("default auth = %#v, want Authorization Bearer", defaulted)
	}

	raw := (AuthConfig{Type: AuthAPIKey, Header: "Authorization", Scheme: "", Key: "secret"}).withDefaults()
	if raw.Header != "Authorization" || raw.Scheme != "" {
		t.Fatalf("raw auth = %#v, want Authorization with empty scheme", raw)
	}
}
