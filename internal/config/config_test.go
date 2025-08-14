package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		routes  []string
	}{
		{
			name: "valid config",
			content: `routes:
  - "10.0.0.0/8"
  - "192.168.0.0/16"`,
			wantErr: false,
			routes:  []string{"10.0.0.0/8", "192.168.0.0/16"},
		},
		{
			name: "invalid CIDR",
			content: `routes:
  - "10.0.0.0/33"`,
			wantErr: true,
		},
		{
			name:    "empty routes",
			content: `routes: []`,
			wantErr: true,
		},
		{
			name:    "no routes key",
			content: `foo: bar`,
			wantErr: true,
		},
		{
			name: "invalid YAML",
			content: `routes:
  - "10.0.0.0/8"
    invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.content), 0600); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := Load(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(cfg.Routes) != len(tt.routes) {
					t.Errorf("Load() routes length = %v, want %v", len(cfg.Routes), len(tt.routes))
				}
				for i, route := range cfg.Routes {
					if route != tt.routes[i] {
						t.Errorf("Load() routes[%d] = %v, want %v", i, route, tt.routes[i])
					}
				}
			}
		})
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/non/existent/path/config.yaml")
	if err == nil {
		t.Error("Load() expected error for non-existent file")
	}
}
