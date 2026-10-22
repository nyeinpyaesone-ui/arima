package config

import (
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadFromEnvUsesSafeDefaults(t *testing.T) {
	cfg, err := LoadFromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second || cfg.ReadTimeout != 15*time.Second ||
		cfg.WriteTimeout != 15*time.Second || cfg.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected timeout defaults: %#v", cfg)
	}
}

func TestLoadFromEnvAcceptsExplicitValues(t *testing.T) {
	cfg, err := LoadFromEnv(env(map[string]string{
		"ARIMA_ADDR": ":9090",
		"ARIMA_READ_HEADER_TIMEOUT": "3s",
		"ARIMA_READ_TIMEOUT": "10s",
		"ARIMA_WRITE_TIMEOUT": "20s",
		"ARIMA_IDLE_TIMEOUT": "90s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" || cfg.ReadHeaderTimeout != 3*time.Second ||
		cfg.ReadTimeout != 10*time.Second || cfg.WriteTimeout != 20*time.Second ||
		cfg.IdleTimeout != 90*time.Second {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadFromEnvRejectsInvalidAddress(t *testing.T) {
	for _, addr := range []string{"", "not-an-address", ":0", ":65536", "localhost:nope"} {
		if addr == "" {
			continue // Empty values intentionally select the documented default.
		}
		t.Run(addr, func(t *testing.T) {
			_, err := LoadFromEnv(env(map[string]string{"ARIMA_ADDR": addr}))
			if err == nil {
				t.Fatalf("expected invalid address %q to fail", addr)
			}
		})
	}
}

func TestLoadFromEnvRejectsInvalidTimeouts(t *testing.T) {
	for _, value := range []string{"abc", "0s", "-1s", "6m"} {
		t.Run(value, func(t *testing.T) {
			_, err := LoadFromEnv(env(map[string]string{"ARIMA_READ_TIMEOUT": value}))
			if err == nil {
				t.Fatalf("expected timeout %q to fail", value)
			}
		})
	}
}

func TestLoadFromEnvRejectsNilReader(t *testing.T) {
	_, err := LoadFromEnv(nil)
	if err == nil || !strings.Contains(err.Error(), "must not be nil") {
		t.Fatalf("expected nil-reader error, got %v", err)
	}
}
