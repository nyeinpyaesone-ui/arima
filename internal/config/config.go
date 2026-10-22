package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr               = ":8080"
	defaultReadHeaderTimeout  = 5 * time.Second
	defaultReadTimeout        = 15 * time.Second
	defaultWriteTimeout       = 15 * time.Second
	defaultIdleTimeout        = 60 * time.Second
	maxHTTPTimeout            = 5 * time.Minute
)

// Config contains validated, non-secret process configuration.
type Config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// LoadFromEnv loads the runtime configuration and rejects malformed values.
// getenv is injected so validation can be tested without mutating process state.
func LoadFromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, fmt.Errorf("environment reader must not be nil")
	}

	cfg := Config{
		Addr:              valueOrDefault(getenv("ARIMA_ADDR"), defaultAddr),
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	var err error
	if cfg.ReadHeaderTimeout, err = durationFromEnv(getenv, "ARIMA_READ_HEADER_TIMEOUT", cfg.ReadHeaderTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ReadTimeout, err = durationFromEnv(getenv, "ARIMA_READ_TIMEOUT", cfg.ReadTimeout); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = durationFromEnv(getenv, "ARIMA_WRITE_TIMEOUT", cfg.WriteTimeout); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = durationFromEnv(getenv, "ARIMA_IDLE_TIMEOUT", cfg.IdleTimeout); err != nil {
		return Config{}, err
	}
	if err := validateAddr(cfg.Addr); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func durationFromEnv(getenv func(string) string, name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid Go duration: %w", name, err)
	}
	if value <= 0 || value > maxHTTPTimeout {
		return 0, fmt.Errorf("%s must be greater than 0 and at most %s", name, maxHTTPTimeout)
	}
	return value, nil
}

func validateAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("ARIMA_ADDR must be host:port or :port: %w", err)
	}
	_ = host
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("ARIMA_ADDR port must be an integer from 1 to 65535")
	}
	return nil
}
