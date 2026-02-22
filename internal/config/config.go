package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Host           string
	Port           int
	DataDir        string
	DBPath         string
	ComfyBaseURL   string
	ComfyPollMs    int
	ComfyTimeoutMs int
	dbPathExplicit bool
}

func LoadFromEnv() (Config, error) {
	if err := loadDotEnvIfPresent(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Host:           envOrDefault("HOST", "127.0.0.1"),
		Port:           envIntOrDefault("PORT", 8877),
		DataDir:        envOrDefault("DATA_DIR", "./data"),
		ComfyBaseURL:   envOrDefault("COMFYUI_BASE_URL", "http://127.0.0.1:8000"),
		ComfyPollMs:    envIntOrDefault("COMFY_POLL_MS", 1200),
		ComfyTimeoutMs: envIntOrDefault("COMFY_TIMEOUT_MS", 90000),
	}

	if dbPath := strings.TrimSpace(os.Getenv("DB_PATH")); dbPath != "" {
		cfg.DBPath = dbPath
		cfg.dbPathExplicit = true
	} else {
		cfg.DBPath = "gts-comfy-helper.sqlite"
	}

	if err := cfg.ValidateAndPrepare(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) ValidateAndPrepare() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.ComfyPollMs < 100 {
		c.ComfyPollMs = 1200
	}
	if c.ComfyTimeoutMs < 1000 {
		c.ComfyTimeoutMs = 90000
	}

	dataDirAbs, err := filepath.Abs(c.DataDir)
	if err != nil {
		return fmt.Errorf("resolve DATA_DIR: %w", err)
	}
	c.DataDir = dataDirAbs
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return fmt.Errorf("create DATA_DIR: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(c.DataDir, "assets"), 0o755); err != nil {
		return fmt.Errorf("create assets dir: %w", err)
	}

	dbPath := strings.TrimSpace(c.DBPath)
	if dbPath == "" {
		dbPath = "gts-comfy-helper.sqlite"
	}
	if filepath.IsAbs(dbPath) {
		// keep
	} else if c.dbPathExplicit {
		abs, err := filepath.Abs(dbPath)
		if err != nil {
			return fmt.Errorf("resolve DB_PATH: %w", err)
		}
		dbPath = abs
	} else {
		dbPath = filepath.Join(c.DataDir, dbPath)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create DB_PATH dir: %w", err)
	}
	c.DBPath = dbPath
	c.ComfyBaseURL = strings.TrimRight(strings.TrimSpace(c.ComfyBaseURL), "/")
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func loadDotEnvIfPresent() error {
	return loadDotEnvFile(".env")
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		_ = os.Setenv(key, value)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}
