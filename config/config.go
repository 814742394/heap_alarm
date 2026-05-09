package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// SMTPConfig holds SMTP server connection settings.
type SMTPConfig struct {
	Host     string   `toml:"host"`
	Port     int      `toml:"port"`
	Username string   `toml:"username"`
	Password string   `toml:"password"`
	From     string   `toml:"from"`
	To       []string `toml:"to"`
	UseTLS   bool     `toml:"use_tls"`
}

// Config is the top-level configuration structure.
type Config struct {
	ProcessName       string     `toml:"process_name"`
	MemoryThresholdMB uint64     `toml:"memory_threshold_mb"`
	CheckIntervalSec  int        `toml:"check_interval_seconds"`
	AlertCooldownSec  int        `toml:"alert_cooldown_seconds"`
	LogFile           string     `toml:"log_file"`
	ServiceMode       string     `toml:"service_mode"`
	SMTP              SMTPConfig `toml:"smtp"`
}

// LoadConfig reads and validates a TOML config file at the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Strip UTF-8 BOM if present (some editors on Windows add it).
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks that all required fields are present and reasonable.
func (c *Config) Validate() error {
	var errs []string

	if c.ProcessName == "" {
		errs = append(errs, "process_name is required")
	}
	if c.MemoryThresholdMB == 0 {
		errs = append(errs, "memory_threshold_mb must be > 0")
	}
	if c.CheckIntervalSec <= 0 {
		errs = append(errs, "check_interval_seconds must be > 0")
	}
	if c.AlertCooldownSec <= 0 {
		errs = append(errs, "alert_cooldown_seconds must be > 0")
	}
	if c.SMTP.Host == "" {
		errs = append(errs, "smtp.host is required")
	}
	if c.SMTP.Port == 0 {
		errs = append(errs, "smtp.port is required")
	}
	if c.SMTP.From == "" {
		errs = append(errs, "smtp.from is required")
	}
	if len(c.SMTP.To) == 0 {
		errs = append(errs, "smtp.to must have at least one recipient")
	}
	if c.SMTP.Username == "" && c.SMTP.Password != "" {
		errs = append(errs, "smtp.username is required when smtp.password is set")
	}
	if c.ServiceMode == "" {
		c.ServiceMode = "console"
	}
	if c.ServiceMode != "console" && c.ServiceMode != "service" {
		errs = append(errs, "service_mode must be \"console\" or \"service\"")
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
