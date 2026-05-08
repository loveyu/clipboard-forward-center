package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	content := `
dsn: mqtt://user:pass@127.0.0.1:1883?clientId=test
http:
  addr: ":9090"
filter:
  time: 3s
storage:
  maxRecords: 50
  expire: 5m
clients:
  - name: test-client
    token: test-token
forward:
  - from: ["clipboard-in-text/test-client"]
    to: ["clipboard-out-text/other"]
    type: text
    format: json
    contentField: content
`
	tmp := t.TempDir() + "/config.yaml"
	os.WriteFile(tmp, []byte(content), 0644)

	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DSN == "" {
		t.Error("DSN should not be empty")
	}
	if cfg.HTTP.Addr != ":9090" {
		t.Errorf("HTTP.Addr = %q, want :9090", cfg.HTTP.Addr)
	}
	if cfg.Filter.Time != "3s" {
		t.Errorf("Filter.Time = %q, want 3s", cfg.Filter.Time)
	}
	if cfg.Storage.MaxRecords != 50 {
		t.Errorf("Storage.MaxRecords = %d, want 50", cfg.Storage.MaxRecords)
	}
	if len(cfg.Clients) != 1 {
		t.Fatalf("len(Clients) = %d, want 1", len(cfg.Clients))
	}
	if cfg.Clients[0].Name != "test-client" {
		t.Errorf("Client.Name = %q", cfg.Clients[0].Name)
	}
	if len(cfg.Forward) != 1 {
		t.Fatalf("len(Forward) = %d, want 1", len(cfg.Forward))
	}
}

func TestLoadDefaults(t *testing.T) {
	content := `
dsn: mqtt://localhost:1883
clients:
  - name: c
    token: t
`
	tmp := t.TempDir() + "/config.yaml"
	os.WriteFile(tmp, []byte(content), 0644)

	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("default addr = %q", cfg.HTTP.Addr)
	}
	if cfg.Storage.MaxRecords != 100 {
		t.Errorf("default maxRecords = %d", cfg.Storage.MaxRecords)
	}
}

func TestLoadMissingDSN(t *testing.T) {
	tmp := t.TempDir() + "/config.yaml"
	os.WriteFile(tmp, []byte("clients: []\n"), 0644)

	_, err := Load(tmp)
	if err == nil {
		t.Error("expected error for missing DSN")
	}
}

func TestFilterDuration(t *testing.T) {
	cfg := &Config{Filter: FilterConfig{Time: "3s"}}
	if d := cfg.FilterDuration(); d != 3*time.Second {
		t.Errorf("FilterDuration = %v", d)
	}
}

func TestStorageExpire(t *testing.T) {
	cfg := &Config{Storage: StorageConfig{Expire: "5m"}}
	if d := cfg.StorageExpire(); d != 5*time.Minute {
		t.Errorf("StorageExpire = %v", d)
	}
}

func TestMQTTOptions(t *testing.T) {
	cfg := &Config{DSN: "mqtt://user:pass@127.0.0.1:1883?clientId=myid&connectTimeout=5&keepAliveInterval=30"}
	opts, err := cfg.MQTTOptions()
	if err != nil {
		t.Fatalf("MQTTOptions: %v", err)
	}
	if opts.Broker != "tcp://127.0.0.1:1883" {
		t.Errorf("Broker = %q", opts.Broker)
	}
	if opts.Username != "user" {
		t.Errorf("Username = %q", opts.Username)
	}
	if opts.Password != "pass" {
		t.Errorf("Password = %q", opts.Password)
	}
	if opts.ClientID != "myid" {
		t.Errorf("ClientID = %q", opts.ClientID)
	}
	if opts.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout = %v", opts.ConnectTimeout)
	}
	if opts.KeepAlive != 30*time.Second {
		t.Errorf("KeepAlive = %v", opts.KeepAlive)
	}
}

func TestFindClientByToken(t *testing.T) {
	cfg := &Config{
		Clients: []Client{
			{Name: "a", Token: "tok-a"},
			{Name: "b", Token: "tok-b"},
		},
	}
	if c := cfg.FindClientByToken("tok-b"); c == nil || c.Name != "b" {
		t.Error("FindClientByToken failed")
	}
	if c := cfg.FindClientByToken("invalid"); c != nil {
		t.Error("FindClientByToken should return nil for unknown token")
	}
}

func TestMQTTOptionsTLS(t *testing.T) {
	cfg := &Config{DSN: "mqtts://user:pass@broker.example.com:8883?clientId=tls-test"}
	opts, err := cfg.MQTTOptions()
	if err != nil {
		t.Fatalf("MQTTOptions: %v", err)
	}
	if opts.Broker != "ssl://broker.example.com:8883" {
		t.Errorf("Broker = %q", opts.Broker)
	}
	if !opts.UseTLS {
		t.Error("UseTLS should be true for mqtts")
	}
}

func TestLoadConfigExample(t *testing.T) {
	cfg, err := Load("../../config-example.yaml")
	if err != nil {
		t.Fatalf("load config-example.yaml: %v", err)
	}
	if len(cfg.Clients) < 2 {
		t.Errorf("expected at least 2 clients, got %d", len(cfg.Clients))
	}
	if len(cfg.Forward) < 2 {
		t.Errorf("expected at least 2 forward rules, got %d", len(cfg.Forward))
	}
	if cfg.FilterDuration() <= 0 {
		t.Error("filter duration should be positive")
	}
	if cfg.StorageExpire() <= 0 {
		t.Error("storage expire should be positive")
	}
	if cfg.Storage.MaxRecords <= 0 {
		t.Error("storage maxRecords should be positive")
	}
	for _, rule := range cfg.Forward {
		if rule.Type == "" || rule.ContentField == "" {
			t.Errorf("forward rule missing type or contentField: %+v", rule)
		}
	}
}
