package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Security  SecurityConfig  `yaml:"security"`
	Certbot   CertbotConfig   `yaml:"certbot"`
	Nginx     NginxConfig     `yaml:"nginx"`
	Log       LogConfig       `yaml:"log"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	DevMode   bool            `yaml:"-"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	DataDir string `yaml:"data_dir"`
}

type SecurityConfig struct {
	JWTSecret        string `yaml:"jwt_secret"`
	AESKey           string `yaml:"aes_key"`
	AllowRegister    bool   `yaml:"allow_register"`
	LoginMaxAttempts int    `yaml:"login_max_attempts"`
	LoginLockoutMin  int    `yaml:"login_lockout_min"`
}

type CertbotConfig struct {
	Email   string `yaml:"email"`
	Webroot string `yaml:"webroot"`
}

type NginxConfig struct {
	ConfDir   string `yaml:"conf_dir"`
	ReloadCmd string `yaml:"reload_cmd"`
	TestCmd   string `yaml:"test_cmd"`
}

type LogConfig struct {
	Level     string `yaml:"level"`
	File      string `yaml:"file"`
	MaxSizeMB int    `yaml:"max_size_mb"`
	MaxDays   int    `yaml:"max_days"`
}

type SchedulerConfig struct {
	MetricsIntervalSec int `yaml:"metrics_interval_sec"`
	CertCheckHour      int `yaml:"cert_check_hour"`
	DeployLogKeepDays  int `yaml:"deploy_log_keep_days"`
}

func Load(path string) (*Config, error) {
	cfg := defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    9999,
			DataDir: "/opt/serverhub",
		},
		Security: SecurityConfig{
			// dev defaults — override in production config.yaml
			JWTSecret:        "serverhub-dev-jwt-secret-change-in-production!!",
			AESKey:           "6465766b6579363436343634363436346465766b657936343634363436343634",
			AllowRegister:    false,
			LoginMaxAttempts: 5,
			LoginLockoutMin:  15,
		},
		Nginx: NginxConfig{
			ConfDir:   "/etc/nginx/conf.d",
			ReloadCmd: "nginx -s reload",
			TestCmd:   "nginx -t",
		},
		Log: LogConfig{
			Level:     "info",
			MaxSizeMB: 100,
			MaxDays:   30,
		},
		Scheduler: SchedulerConfig{
			MetricsIntervalSec: 5,
			CertCheckHour:      2,
			DeployLogKeepDays:  30,
		},
	}
}
