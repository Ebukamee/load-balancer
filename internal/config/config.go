// Package config loads and validates the load balancer's JSON config file.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"
)

// Config is the whole config file. The `json:"..."` tags say which JSON key
// fills which field.
type Config struct {
	Listen   string   `json:"listen"`   // address to listen on, e.g. ":8080"
	Backends []string `json:"backends"` // backend URLs, e.g. "http://localhost:7001"
	Health   Health   `json:"health"`
	Proxy    Proxy    `json:"proxy"`
}

// Health holds the health-check settings.
type Health struct {
	Interval Duration `json:"interval"` // time between checks, e.g. "5s"
	Timeout  Duration `json:"timeout"`  // how long one check may take, e.g. "2s"
}

// Proxy holds the settings for forwarding requests to backends.
type Proxy struct {
	ConnectTimeout  Duration `json:"connect_timeout"`  // max time to open a connection, e.g. "2s"
	ResponseTimeout Duration `json:"response_timeout"` // max wait for a backend's reply, e.g. "10s"
	MaxAttempts     int      `json:"max_attempts"`     // tries per request, counting the first, e.g. 3
}

// Defaults used when the file leaves a setting out.
const (
	DefaultListen   = ":8080"
	DefaultInterval = 5 * time.Second
	DefaultTimeout  = 2 * time.Second

	DefaultConnectTimeout  = 2 * time.Second
	DefaultResponseTimeout = 10 * time.Second
	DefaultMaxAttempts     = 3
)

// Load reads the config file at path, fills in defaults for anything left
// out, and validates the result. Any problem is returned as an error that
// names the file.
func Load(path string) (*Config, error) {

	data, err := os.ReadFile(path) //nolint:gosec // path comes from the operator's -config flag, not from clients

	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return &cfg, nil

}

// applyDefaults fills in any setting the file left empty.
func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = DefaultListen
	}
	if c.Health.Interval.Duration == 0 {
		c.Health.Interval.Duration = DefaultInterval
	}
	if c.Health.Timeout.Duration == 0 {
		c.Health.Timeout.Duration = DefaultTimeout
	}
	if c.Proxy.ConnectTimeout.Duration == 0 {
		c.Proxy.ConnectTimeout.Duration = DefaultConnectTimeout
	}
	if c.Proxy.ResponseTimeout.Duration == 0 {
		c.Proxy.ResponseTimeout.Duration = DefaultResponseTimeout
	}
	if c.Proxy.MaxAttempts == 0 {
		c.Proxy.MaxAttempts = DefaultMaxAttempts
	}
}

// Validate checks every setting and reports ALL problems at once, so the
// user can fix the whole file in one go instead of one error per restart.
func (c *Config) Validate() error {
	// TODO(step 4) Validate:
	//
	var errs []error

	if c.Listen == "" {
		errs = append(errs, errors.New("listen must not be empty"))
	}
	if len(c.Backends) == 0 {
		errs = append(errs, errors.New("at least one backend is required"))
	}
	seen := map[string]bool{}
	for _, raw := range c.Backends {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, fmt.Errorf("invalid backend %q: want http://host:port", raw))
		}
		if seen[raw] {
			errs = append(errs, fmt.Errorf("duplicate backend %q", raw))
		}
		seen[raw] = true
	}

	if c.Health.Interval.Duration <= 0 {
		errs = append(errs, errors.New("health interval must be positive"))
	}
	if c.Health.Timeout.Duration <= 0 {
		errs = append(errs, errors.New("health timeout must be positive"))
	}
	if c.Health.Timeout.Duration >= c.Health.Interval.Duration {
		errs = append(errs, errors.New("health timeout must be shorter than interval"))
	}
	if c.Proxy.ConnectTimeout.Duration <= 0 {
		errs = append(errs, errors.New("proxy connect_timeout must be positive"))
	}
	if c.Proxy.MaxAttempts < 1 {
		errs = append(errs, errors.New("proxy max_attempts must be at least 1"))
	}
	if c.Proxy.ResponseTimeout.Duration <= 0 {
		errs = append(errs, errors.New("proxy response_timeout must be positive"))
	}

	return errors.Join(errs...)

}

// Duration is a time.Duration that reads from JSON as text like "5s" or
// "1m30s". A plain time.Duration would expect a number of NANOSECONDS
// (5s = 5000000000), which nobody wants to type in a config file.
type Duration struct {
	time.Duration
}

// UnmarshalJSON is called by encoding/json whenever it fills a Duration.
// Any type with this method controls how it is read from JSON.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"5s\": %w", err)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}
