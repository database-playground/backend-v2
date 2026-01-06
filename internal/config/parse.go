package config

import (
	"log/slog"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

func LoadBackendConfig() (BackendConfig, error) {
	var cfg BackendConfig

	if err := godotenv.Load(); err != nil {
		slog.Warn("error loading .env file", "error", err)
	}

	if err := env.Parse(&cfg); err != nil {
		return BackendConfig{}, err
	}

	if err := cfg.Validate(); err != nil {
		return BackendConfig{}, err
	}

	return cfg, nil
}

func LoadExporterConfig() (ExporterConfig, error) {
	var cfg ExporterConfig

	if err := godotenv.Load(); err != nil {
		slog.Warn("error loading .env file", "error", err)
	}

	if err := env.Parse(&cfg); err != nil {
		return ExporterConfig{}, err
	}

	if err := cfg.Validate(); err != nil {
		return ExporterConfig{}, err
	}

	return cfg, nil
}

func LoadAdminCLIConfig() (AdminCLIConfig, error) {
	var cfg AdminCLIConfig

	if err := godotenv.Load(); err != nil {
		slog.Warn("error loading .env file", "error", err)
	}

	if err := env.Parse(&cfg); err != nil {
		return AdminCLIConfig{}, err
	}

	if err := cfg.Validate(); err != nil {
		return AdminCLIConfig{}, err
	}

	return cfg, nil
}
