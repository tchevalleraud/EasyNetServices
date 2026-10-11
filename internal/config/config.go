// Package config loads the runtime configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Role defines which components a container runs.
type Role string

const (
	RoleStandalone Role = "standalone"
	RoleController Role = "controller"
	RoleNode       Role = "node"
)

// TLSMode defines how the web server handles TLS.
type TLSMode string

const (
	// TLSSelfSigned generates and reuses a self-signed certificate in the data directory.
	TLSSelfSigned TLSMode = "self-signed"
	// TLSCustom uses the certificate and key provided by the operator.
	TLSCustom TLSMode = "custom"
	// TLSOff serves plain HTTP, typically behind a TLS-terminating reverse proxy.
	TLSOff TLSMode = "off"
)

// Config holds the runtime configuration.
type Config struct {
	Role       Role
	DataDir    string
	Listen     string
	TLSMode    TLSMode
	TLSCert    string
	TLSKey     string
	SessionTTL time.Duration
	LogLevel   slog.Level
}

// Load reads the configuration from ENS_* environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		Role:    Role(env("ENS_ROLE", string(RoleStandalone))),
		DataDir: env("ENS_DATA_DIR", "/data"),
		Listen:  env("ENS_LISTEN", ":8443"),
		TLSMode: TLSMode(env("ENS_TLS_MODE", string(TLSSelfSigned))),
		TLSCert: os.Getenv("ENS_TLS_CERT"),
		TLSKey:  os.Getenv("ENS_TLS_KEY"),
	}

	ttl, err := time.ParseDuration(env("ENS_SESSION_TTL", "12h"))
	if err != nil || ttl <= 0 {
		return nil, fmt.Errorf("invalid ENS_SESSION_TTL: %q", os.Getenv("ENS_SESSION_TTL"))
	}
	cfg.SessionTTL = ttl

	if err := cfg.LogLevel.UnmarshalText([]byte(env("ENS_LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("invalid ENS_LOG_LEVEL: %w", err)
	}

	switch cfg.Role {
	case RoleStandalone:
	case RoleController, RoleNode:
		return nil, fmt.Errorf("role %q is not supported yet", cfg.Role)
	default:
		return nil, fmt.Errorf("invalid ENS_ROLE: %q", cfg.Role)
	}

	switch cfg.TLSMode {
	case TLSSelfSigned, TLSOff:
	case TLSCustom:
		if cfg.TLSCert == "" || cfg.TLSKey == "" {
			return nil, fmt.Errorf("ENS_TLS_CERT and ENS_TLS_KEY are required when ENS_TLS_MODE=custom")
		}
	default:
		return nil, fmt.Errorf("invalid ENS_TLS_MODE: %q", cfg.TLSMode)
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
