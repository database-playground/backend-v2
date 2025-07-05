package config

import "errors"

type Config struct {
	Port         int      `env:"PORT" envDefault:"8080"`
	ServerURI    string   `env:"SERVER_URI"`
	TrustProxies []string `env:"TRUST_PROXIES"`

	Redis RedisConfig `envPrefix:"REDIS_"`
	GAuth GAuthConfig `envPrefix:"GAUTH_"`
}

func (c Config) Validate() error {
	if c.ServerURI == "" {
		return errors.New("SERVER_URI is required")
	}

	if err := c.Redis.Validate(); err != nil {
		return err
	}
	if err := c.GAuth.Validate(); err != nil {
		return err
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
	ClientID     string `env:"CLIENT_ID"`
	ClientSecret string `env:"CLIENT_SECRET"`
	RedirectURL  string `env:"REDIRECT_URL"`
}

func (c GAuthConfig) Validate() error {
	if c.ClientID == "" {
		return errors.New("GAUTH_CLIENT_ID is required")
	}
	if c.ClientSecret == "" {
		return errors.New("GAUTH_CLIENT_SECRET is required")
	}
	if c.RedirectURL == "" {
		return errors.New("GAUTH_REDIRECT_URL is required")
	}

	return nil
}
