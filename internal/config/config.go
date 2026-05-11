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
	MaxRecords      int    `yaml:"maxRecords"`
	Expire          string `yaml:"expire"`
	MaxBodySize     int64  `yaml:"maxBodySize"`
	FileStorageSize int64  `yaml:"fileStorageSize"`
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
	QoS          *int     `yaml:"qos"`
	Retain       *bool    `yaml:"retain"`
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
	QoS                  byte
	Retain               bool
}

var dsnCache struct {
	dsn  string
	opts *MQTTOptions
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		HTTP: HTTPConfig{Addr: ":8080"},
		Storage: StorageConfig{
			MaxRecords:      100,
			Expire:          "10m",
			MaxBodySize:     100 * 1024 * 1024,
			FileStorageSize: 20 * 1024 * 1024,
		},
		Filter: FilterConfig{Time: "5s"},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.DSN == "" {
		return nil, fmt.Errorf("dsn is required")
	}

	if cfg.Storage.MaxBodySize <= 0 {
		cfg.Storage.MaxBodySize = 100 * 1024 * 1024
	}
	if cfg.Storage.FileStorageSize <= 0 {
		cfg.Storage.FileStorageSize = 20 * 1024 * 1024
	}
	if cfg.Storage.FileStorageSize > cfg.Storage.MaxBodySize {
		cfg.Storage.FileStorageSize = cfg.Storage.MaxBodySize
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

func (c *Config) DefaultQoS() byte {
	opts, err := c.MQTTOptions()
	if err != nil {
		return 0
	}
	return opts.QoS
}

func (c *Config) DefaultRetain() bool {
	opts, err := c.MQTTOptions()
	if err != nil {
		return false
	}
	return opts.Retain
}

func (r *ForwardRule) QoSValue(defaultQoS byte) byte {
	if r.QoS != nil {
		q := *r.QoS
		if q < 0 {
			return 0
		}
		if q > 2 {
			return 2
		}
		return byte(q)
	}
	return defaultQoS
}

func (r *ForwardRule) RetainValue(defaultRetain bool) bool {
	if r.Retain != nil {
		return *r.Retain
	}
	return defaultRetain
}

func (c *Config) StorageExpire() time.Duration {
	d, err := time.ParseDuration(c.Storage.Expire)
	if err != nil {
		return 10 * time.Minute
	}
	return d
}

func (c *Config) MQTTOptions() (*MQTTOptions, error) {
	if dsnCache.dsn == c.DSN && dsnCache.opts != nil {
		return dsnCache.opts, nil
	}

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
	if v := q.Get("qos"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n < 0 {
				n = 0
			}
			if n > 2 {
				n = 2
			}
			opts.QoS = byte(n)
		}
	}
	if v := q.Get("retain"); v != "" {
		opts.Retain = v == "true"
	}

	dsnCache.dsn = c.DSN
	dsnCache.opts = opts
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
