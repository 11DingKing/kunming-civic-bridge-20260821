package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Storage   StorageConfig   `yaml:"storage"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Logging   LoggingConfig   `yaml:"logging"`
	Business  BusinessConfig  `yaml:"business"`
	Auth      AuthConfig      `yaml:"auth"`
}

type AuthConfig struct {
	StorePath      string              `yaml:"store_path"`
	SessionTTL     time.Duration       `yaml:"session_ttl"`
	Required       bool                `yaml:"required"`
	BootstrapUsers []AuthBootstrapUser `yaml:"-"`
	BootstrapError string              `yaml:"-"`
}

type AuthBootstrapUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type StorageConfig struct {
	DataDir      string `yaml:"data_dir"`
	ShardMaxSize int64  `yaml:"shard_max_size"`
	SyncOnWrite  bool   `yaml:"sync_on_write"`
}

type SchedulerConfig struct {
	EscalationInterval     time.Duration `yaml:"escalation_interval"`
	ReconciliationInterval time.Duration `yaml:"reconciliation_interval"`
	ReevalInterval         time.Duration `yaml:"reeval_interval"`
	MaxRetries             int           `yaml:"max_retries"`
	BaseBackoff            time.Duration `yaml:"base_backoff"`
	TaskTimeout            time.Duration `yaml:"task_timeout"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type BusinessConfig struct {
	DefaultDeadline             time.Duration `yaml:"default_deadline"`
	EscalationDeadlineExtension time.Duration `yaml:"escalation_deadline_extension"`
	MaxEscalationLevel          int           `yaml:"max_escalation_level"`
}

func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            49660,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Storage: StorageConfig{
			DataDir:      "./data",
			ShardMaxSize: 1048576,
			SyncOnWrite:  true,
		},
		Scheduler: SchedulerConfig{
			EscalationInterval:     30 * time.Second,
			ReconciliationInterval: 5 * time.Minute,
			ReevalInterval:         1 * time.Minute,
			MaxRetries:             3,
			BaseBackoff:            1 * time.Second,
			TaskTimeout:            10 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Business: BusinessConfig{
			DefaultDeadline:             72 * time.Hour,
			EscalationDeadlineExtension: 48 * time.Hour,
			MaxEscalationLevel:          3,
		},
		Auth: AuthConfig{StorePath: "./data/auth.json", SessionTTL: 8 * time.Hour, Required: true},
	}
}

func Load(path string) (*Config, error) {
	cfg := Defaults()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config file %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config yaml: %w", err)
		}
	}
	applyEnvOverrides(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Storage.DataDir == "" {
		return fmt.Errorf("storage.data_dir must not be empty")
	}
	if c.Business.MaxEscalationLevel < 1 {
		return fmt.Errorf("business.max_escalation_level must be at least 1")
	}
	if c.Business.DefaultDeadline <= 0 {
		return fmt.Errorf("business.default_deadline must be positive")
	}
	if c.Business.EscalationDeadlineExtension <= 0 {
		return fmt.Errorf("business.escalation_deadline_extension must be positive")
	}
	if c.Scheduler.MaxRetries < 1 {
		return fmt.Errorf("scheduler.max_retries must be at least 1")
	}
	if c.Auth.StorePath == "" || c.Auth.SessionTTL <= 0 {
		return fmt.Errorf("auth.store_path and auth.session_ttl must be configured")
	}
	if c.Auth.BootstrapError != "" {
		return fmt.Errorf("auth bootstrap users are invalid: %s", c.Auth.BootstrapError)
	}
	return nil
}

func applyEnvOverrides(cfg *Config) {
	envMap := map[string]string{
		"CIVICBRIDGE_SERVER_PORT":                            "",
		"CIVICBRIDGE_SERVER_READ_TIMEOUT":                    "",
		"CIVICBRIDGE_SERVER_WRITE_TIMEOUT":                   "",
		"CIVICBRIDGE_SERVER_SHUTDOWN_TIMEOUT":                "",
		"CIVICBRIDGE_STORAGE_DATA_DIR":                       "",
		"CIVICBRIDGE_STORAGE_SHARD_MAX_SIZE":                 "",
		"CIVICBRIDGE_STORAGE_SYNC_ON_WRITE":                  "",
		"CIVICBRIDGE_SCHEDULER_ESCALATION_INTERVAL":          "",
		"CIVICBRIDGE_SCHEDULER_RECONCILIATION_INTERVAL":      "",
		"CIVICBRIDGE_SCHEDULER_REEVAL_INTERVAL":              "",
		"CIVICBRIDGE_SCHEDULER_MAX_RETRIES":                  "",
		"CIVICBRIDGE_SCHEDULER_BASE_BACKOFF":                 "",
		"CIVICBRIDGE_SCHEDULER_TASK_TIMEOUT":                 "",
		"CIVICBRIDGE_LOGGING_LEVEL":                          "",
		"CIVICBRIDGE_LOGGING_FORMAT":                         "",
		"CIVICBRIDGE_BUSINESS_DEFAULT_DEADLINE":              "",
		"CIVICBRIDGE_BUSINESS_ESCALATION_DEADLINE_EXTENSION": "",
		"CIVICBRIDGE_BUSINESS_MAX_ESCALATION_LEVEL":          "",
		"CIVICBRIDGE_AUTH_STORE_PATH":                        "",
		"CIVICBRIDGE_AUTH_SESSION_TTL":                       "",
		"CIVICBRIDGE_AUTH_BOOTSTRAP_USERS":                   "",
	}

	if v := os.Getenv("CIVICBRIDGE_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("CIVICBRIDGE_SERVER_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ReadTimeout = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_SERVER_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.WriteTimeout = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_SERVER_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ShutdownTimeout = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_STORAGE_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	}
	if v := os.Getenv("CIVICBRIDGE_STORAGE_SHARD_MAX_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Storage.ShardMaxSize = n
		}
	}
	if v := os.Getenv("CIVICBRIDGE_STORAGE_SYNC_ON_WRITE"); v != "" {
		cfg.Storage.SyncOnWrite = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("CIVICBRIDGE_SCHEDULER_ESCALATION_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Scheduler.EscalationInterval = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_SCHEDULER_RECONCILIATION_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Scheduler.ReconciliationInterval = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_SCHEDULER_REEVAL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Scheduler.ReevalInterval = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_SCHEDULER_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Scheduler.MaxRetries = n
		}
	}
	if v := os.Getenv("CIVICBRIDGE_SCHEDULER_BASE_BACKOFF"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Scheduler.BaseBackoff = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_SCHEDULER_TASK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Scheduler.TaskTimeout = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_LOGGING_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("CIVICBRIDGE_LOGGING_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}
	if v := os.Getenv("CIVICBRIDGE_BUSINESS_DEFAULT_DEADLINE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Business.DefaultDeadline = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_BUSINESS_ESCALATION_DEADLINE_EXTENSION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Business.EscalationDeadlineExtension = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_BUSINESS_MAX_ESCALATION_LEVEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Business.MaxEscalationLevel = n
		}
	}
	if v := os.Getenv("CIVICBRIDGE_AUTH_STORE_PATH"); v != "" {
		cfg.Auth.StorePath = v
	}
	if v := os.Getenv("CIVICBRIDGE_AUTH_SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Auth.SessionTTL = d
		}
	}
	if v := os.Getenv("CIVICBRIDGE_AUTH_BOOTSTRAP_USERS"); v != "" {
		var users []AuthBootstrapUser
		if err := json.Unmarshal([]byte(v), &users); err != nil {
			cfg.Auth.BootstrapError = err.Error()
		} else {
			cfg.Auth.BootstrapUsers = users
		}
	}
	_ = envMap
}
