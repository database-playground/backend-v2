package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port           int      `env:"PORT" envDefault:"8080"`
	AllowedOrigins []string `env:"ALLOWED_ORIGINS"`
	TrustProxies   []string `env:"TRUST_PROXIES"`

	Database  DatabaseConfig  `envPrefix:"DATABASE_"`
	Redis     RedisConfig     `envPrefix:"REDIS_"`
	GAuth     GAuthConfig     `envPrefix:"GAUTH_"`
	Server    ServerConfig    `envPrefix:"SERVER_"`
	SqlRunner SqlRunnerConfig `envPrefix:"SQL_RUNNER_"`
	PostHog   PostHogConfig   `envPrefix:"POSTHOG_"`
}

func (c Config) Validate() error {
	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("DATABASE: %w", err)
	}
	if err := c.Redis.Validate(); err != nil {
		return fmt.Errorf("REDIS: %w", err)
	}
	if err := c.GAuth.Validate(); err != nil {
		return fmt.Errorf("GAUTH: %w", err)
	}
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("SERVER: %w", err)
	}
	if err := c.SqlRunner.Validate(); err != nil {
		return fmt.Errorf("SQL_RUNNER: %w", err)
	}
	if err := c.PostHog.Validate(); err != nil {
		return fmt.Errorf("POSTHOG: %w", err)
	}

	return nil
}

type DatabaseConfig struct {
	URI string `env:"URI"`
}

func (c DatabaseConfig) Validate() error {
	if c.URI == "" {
		return errors.New("DATABASE_URI is required")
	}

	if !strings.HasPrefix(c.URI, "postgres://") {
		return errors.New("DATABASE_URI must be a valid PostgreSQL URL")
	}

	return nil
}

type RedisConfig struct {
	Host     string `env:"HOST"`
	Port     int    `env:"PORT"`
	Username string `env:"USERNAME"`
	Password string `env:"PASSWORD"`
}

func (c RedisConfig) Validate() error {
	if c.Host == "" {
		return errors.New("REDIS_HOST is required")
	}
	if c.Port == 0 {
		return errors.New("REDIS_PORT is required")
	}

	return nil
}

type GAuthConfig struct {
	Secret       string   `env:"SECRET"`
	ClientID     string   `env:"CLIENT_ID"`
	ClientSecret string   `env:"CLIENT_SECRET"`
	RedirectURIs []string `env:"REDIRECT_URIS"`
}

func (c GAuthConfig) Validate() error {
	if c.Secret == "" {
		return errors.New("GAUTH_SECRET is required")
	}
	if len(c.Secret) != 32 {
		return errors.New("GAUTH_SECRET must be 32 characters long")
	}
	if c.ClientID == "" {
		return errors.New("GAUTH_CLIENT_ID is required")
	}
	if c.ClientSecret == "" {
		return errors.New("GAUTH_CLIENT_SECRET is required")
	}
	if len(c.RedirectURIs) == 0 {
		return errors.New("GAUTH_REDIRECT_URIS is required")
	}

	return nil
}

type ServerConfig struct {
	URI      string  `env:"URI"`
	CertFile *string `env:"CERT_FILE"`
	KeyFile  *string `env:"KEY_FILE"`
}

func (c ServerConfig) Validate() error {
	if c.URI == "" {
		return errors.New("SERVER_URI is required")
	}

	if (c.CertFile != nil && c.KeyFile == nil) || (c.CertFile == nil && c.KeyFile != nil) {
		return errors.New("SERVER_CERT_FILE and SERVER_KEY_FILE must be set together")
	}

	// check if both cert and key are there
	if c.CertFile != nil {
		if _, err := os.Stat(*c.CertFile); os.IsNotExist(err) {
			return errors.New("SERVER_CERT_FILE does not exist")
		}
	}

	if c.KeyFile != nil {
		if _, err := os.Stat(*c.KeyFile); os.IsNotExist(err) {
			return errors.New("SERVER_KEY_FILE does not exist")
		}
	}

	return nil
}

func (c ServerConfig) GetProto() string {
	if c.CertFile != nil && c.KeyFile != nil {
		return "https"
	}

	return "http"
}

type SqlRunnerConfig struct {
	URI string `env:"URI"`
}

func (c SqlRunnerConfig) Validate() error {
	if c.URI == "" {
		return errors.New("SQL_RUNNER_URI is required")
	}

	return nil
}

type PostHogConfig struct {
	APIKey *string `env:"API_KEY"`
}

func (c PostHogConfig) Validate() error {
	if c.APIKey != nil && *c.APIKey == "" {
		return errors.New("POSTHOG_API_KEY cannot be empty")
	}

	return nil
}
