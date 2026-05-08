package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DSN     string        `yaml:"dsn"`
	HTTP    HTTPConfig    `yaml:"http"`
	Filter  FilterConfig  `yaml:"filter"`
	Storage StorageConfig `yaml:"storage"`
	Clients []Client      `yaml:"clients"`
	Forward []ForwardRule `yaml:"forward"`
}

type HTTPConfig struct {
	Addr string `yaml:"addr"`
}

type FilterConfig struct {
	Time string `yaml:"time"`
}

type StorageConfig struct {
	MaxRecords int    `yaml:"maxRecords"`
	Expire     string `yaml:"expire"`
}

type Client struct {
	Name  string `yaml:"name"`
	Token string `yaml:"token"`
}

type ForwardRule struct {
	From         []string `yaml:"from"`
	To           []string `yaml:"to"`
	Type         string   `yaml:"type"`
	Format       string   `yaml:"format"`
	ContentField string   `yaml:"contentField"`
}

type MQTTOptions struct {
	Broker               string
	Username             string
	Password             string
	ClientID             string
	ConnectTimeout       time.Duration
	KeepAlive            time.Duration
	AutoReconnect        bool
	MaxReconnectInterval time.Duration
	UseTLS               bool
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		HTTP: HTTPConfig{Addr: ":8080"},
		Storage: StorageConfig{
			MaxRecords: 100,
			Expire:     "10m",
		},
		Filter: FilterConfig{Time: "5s"},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.DSN == "" {
		return nil, fmt.Errorf("dsn is required")
	}

	return cfg, nil
}

func (c *Config) FilterDuration() time.Duration {
	d, err := time.ParseDuration(c.Filter.Time)
	if err != nil {
		return 5 * time.Second
	}
	return d
}

func (c *Config) StorageExpire() time.Duration {
	d, err := time.ParseDuration(c.Storage.Expire)
	if err != nil {
		return 10 * time.Minute
	}
	return d
}

func (c *Config) MQTTOptions() (*MQTTOptions, error) {
	u, err := url.Parse(c.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse DSN: %w", err)
	}

	scheme := "tcp"
	if u.Scheme == "mqtts" {
		scheme = "ssl"
	}

	opts := &MQTTOptions{
		Broker:               scheme + "://" + u.Host,
		UseTLS:               u.Scheme == "mqtts",
		AutoReconnect:        true,
		ConnectTimeout:       3 * time.Second,
		KeepAlive:            20 * time.Second,
		MaxReconnectInterval: 60 * time.Second,
	}

	if u.User != nil {
		opts.Username = u.User.Username()
		opts.Password, _ = u.User.Password()
	}

	q := u.Query()
	opts.ClientID = q.Get("clientId")

	if v := q.Get("connectTimeout"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts.ConnectTimeout = time.Duration(n) * time.Second
		}
	}
	if v := q.Get("keepAliveInterval"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts.KeepAlive = time.Duration(n) * time.Second
		}
	}
	if v := q.Get("automaticReconnect"); v != "" {
		opts.AutoReconnect = v == "true"
	}
	if v := q.Get("reconnectMaxInterval"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts.MaxReconnectInterval = time.Duration(n) * time.Second
		}
	}

	return opts, nil
}

func (c *Config) FindClientByToken(token string) *Client {
	for i := range c.Clients {
		if c.Clients[i].Token == token {
			return &c.Clients[i]
		}
	}
	return nil
}

func ConfigPath() string {
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		return p
	}
	return "config.yaml"
}

func IsDebug() bool {
	return os.Getenv("DEBUG") != ""
}

func RemoteConfigURL() string {
	return os.Getenv("REMOTE_CONFIG_URL")
}
