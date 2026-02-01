package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Radio    RadioConfig    `yaml:"radio"`
	MQTT     MQTTConfig     `yaml:"mqtt"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
	Host string     `yaml:"host"`
	Port int        `yaml:"port"`
	CORS CORSConfig `yaml:"cors"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type RadioConfig struct {
	AutoConnect      bool   `yaml:"auto_connect"`
	ConnectionType   string `yaml:"connection_type"`
	Address          string `yaml:"address"`
	HeartbeatSecs    int    `yaml:"heartbeat_secs"`
	ReconnectDelaySecs int  `yaml:"reconnect_delay_secs"`
}

type MQTTConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Server    string `yaml:"server"`
	Port      int    `yaml:"port"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	RootTopic string `yaml:"root_topic"`
	JSONTopic string `yaml:"json_topic"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	setDefaults(cfg)

	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	loadFromEnv(cfg)

	return cfg, nil
}

func setDefaults(cfg *Config) {
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080
	cfg.Server.CORS.Enabled = true
	cfg.Server.CORS.AllowedOrigins = []string{"*"}

	cfg.Database.Path = "./meshtastic.db"

	cfg.Radio.HeartbeatSecs = 30
	cfg.Radio.ReconnectDelaySecs = 5

	cfg.MQTT.Server = "mqtt.meshtastic.org"
	cfg.MQTT.Port = 1883
	cfg.MQTT.RootTopic = "msh/2/e/"
	cfg.MQTT.JSONTopic = "msh/2/json/"

	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
}

func loadFromEnv(cfg *Config) {
	if host := os.Getenv("MESHTASTIC_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if port := os.Getenv("MESHTASTIC_PORT"); port != "" {
		// Parse port if needed
	}
	if dbPath := os.Getenv("MESHTASTIC_DB_PATH"); dbPath != "" {
		cfg.Database.Path = dbPath
	}
	if logLevel := os.Getenv("MESHTASTIC_LOG_LEVEL"); logLevel != "" {
		cfg.Logging.Level = logLevel
	}
}
