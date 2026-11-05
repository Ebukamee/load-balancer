package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFile saves content to a file in a fresh temporary folder (deleted
// automatically when the test ends) and returns its path.
func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lb.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValidFile(t *testing.T) {
	path := writeFile(t, `{
		"listen": ":9090",
		"backends": ["http://localhost:7001", "http://localhost:7002"],
		"health": {"interval": "10s", "timeout": "1500ms"}
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":9090" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, ":9090")
	}
	if len(cfg.Backends) != 2 || cfg.Backends[1] != "http://localhost:7002" {
		t.Errorf("Backends = %v, want the 2 backends from the file", cfg.Backends)
	}
	if cfg.Health.Interval.Duration != 10*time.Second {
		t.Errorf("Interval = %v, want 10s", cfg.Health.Interval.Duration)
	}
	if cfg.Health.Timeout.Duration != 1500*time.Millisecond {
		t.Errorf("Timeout = %v, want 1.5s", cfg.Health.Timeout.Duration)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := writeFile(t, `{"backends": ["http://localhost:7001"]}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != DefaultListen {
		t.Errorf("Listen = %q, want default %q", cfg.Listen, DefaultListen)
	}
	if cfg.Health.Interval.Duration != DefaultInterval {
		t.Errorf("Interval = %v, want default %v", cfg.Health.Interval.Duration, DefaultInterval)
	}
	if cfg.Health.Timeout.Duration != DefaultTimeout {
		t.Errorf("Timeout = %v, want default %v", cfg.Health.Timeout.Duration, DefaultTimeout)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want one that wraps fs.ErrNotExist (use %%w, not %%v)", err)
	}
}

func TestLoadBadJSON(t *testing.T) {
	path := writeFile(t, `{"backends": ["http://localhost:7001",]}`) // trailing comma

	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted broken JSON")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	// "backend" is a typo for "backends": it must be an error, not ignored.
	path := writeFile(t, `{"backend": ["http://localhost:7001"]}`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), `unknown field "backend"`) {
		t.Fatalf("err = %v, want an unknown field error (use DisallowUnknownFields)", err)
	}
}

func TestLoadBadDuration(t *testing.T) {
	path := writeFile(t, `{"backends": ["http://localhost:7001"], "health": {"interval": "5 seconds"}}`)

	if _, err := Load(path); err == nil {
		t.Fatal(`Load accepted "5 seconds"; durations look like "5s"`)
	}
}

func TestLoadRunsValidate(t *testing.T) {
	path := writeFile(t, `{"backends": []}`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "at least one backend") {
		t.Fatalf("err = %v, want Load to call Validate", err)
	}
}

// validConfig returns a config that passes Validate. Each table case below
// breaks one thing about it.
func validConfig() Config {
	return Config{
		Listen:   ":8080",
		Backends: []string{"http://localhost:7001", "https://example.com"},
		Health: Health{
			Interval: Duration{5 * time.Second},
			Timeout:  Duration{2 * time.Second},
		},
	}
}

// A table-driven test: one row per scenario, one loop that runs them all.
// Adding a new rule later means adding one row, not a whole new function.
func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(c *Config) // how this case breaks the valid config
		wantErr string          // "" means "must be valid"
	}{
		{"valid config", func(*Config) {}, ""},
		{"empty listen", func(c *Config) { c.Listen = "" }, "listen must not be empty"},
		{"no backends", func(c *Config) { c.Backends = nil }, "at least one backend"},
		{"not a URL", func(c *Config) { c.Backends = []string{"localhost:7001"} }, `invalid backend "localhost:7001"`},
		{"wrong scheme", func(c *Config) { c.Backends = []string{"ftp://localhost:7001"} }, `invalid backend "ftp://localhost:7001"`},
		{"no host", func(c *Config) { c.Backends = []string{"http://"} }, `invalid backend "http://"`},
		{"duplicate", func(c *Config) { c.Backends = []string{"http://a:1", "http://a:1"} }, `duplicate backend "http://a:1"`},
		{"zero interval", func(c *Config) { c.Health.Interval = Duration{0} }, "health interval must be positive"},
		{"negative timeout", func(c *Config) { c.Health.Timeout = Duration{-time.Second} }, "health timeout must be positive"},
		{"timeout too long", func(c *Config) { c.Health.Timeout = Duration{5 * time.Second} }, "health timeout must be shorter than interval"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(&cfg)

			err := cfg.Validate()

			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("want valid, got error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("want error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	cfg := validConfig()
	cfg.Backends = nil
	cfg.Health.Interval = Duration{0}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("want errors, got nil")
	}
	for _, want := range []string{"at least one backend", "health interval must be positive"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q (report every problem, not just the first)", err, want)
		}
	}
}
