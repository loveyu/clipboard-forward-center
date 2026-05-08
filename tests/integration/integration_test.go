//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"clipboard-forward-center/internal/config"
	"clipboard-forward-center/internal/filter"
	"clipboard-forward-center/internal/forward"
	"clipboard-forward-center/internal/httpserver"
	"clipboard-forward-center/internal/mqttclient"
	"clipboard-forward-center/internal/store"
)

func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load("../../config-example.yaml")
	if err != nil {
		t.Fatalf("load config-example.yaml: %v", err)
	}
	cfg.DSN = "mqtt://127.0.0.1:1883?clientId=integration-test&connectTimeout=5"
	return cfg
}

func newObserverClient(t *testing.T, clientID string) mqtt.Client {
	t.Helper()
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://127.0.0.1:1883")
	opts.SetClientID(clientID)
	opts.SetConnectTimeout(5 * time.Second)
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		t.Fatalf("observer connect: %v", token.Error())
	}
	t.Cleanup(func() { c.Disconnect(500) })
	return c
}

func TestLoadConfigExample(t *testing.T) {
	cfg := loadTestConfig(t)

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

	opts, err := cfg.MQTTOptions()
	if err != nil {
		t.Fatalf("MQTTOptions: %v", err)
	}
	if opts.Broker == "" {
		t.Error("broker should not be empty")
	}

	for _, rule := range cfg.Forward {
		if len(rule.From) == 0 || len(rule.To) == 0 {
			t.Errorf("forward rule must have from and to: %+v", rule)
		}
		if rule.Type == "" || rule.ContentField == "" {
			t.Errorf("forward rule must have type and contentField: %+v", rule)
		}
	}
}

func TestForwardText(t *testing.T) {
	cfg := loadTestConfig(t)

	rule := cfg.Forward[0]
	sourceTopic := rule.From[0]
	targetTopic := rule.To[0]

	f := filter.New(cfg.FilterDuration())

	engine := forward.NewEngine(cfg, f, nil, true)
	mc := mqttclient.New(cfg, engine.HandleMessage, true)
	engine.SetPublisher(mc)

	if err := mc.Connect(); err != nil {
		t.Fatalf("service connect: %v", err)
	}
	t.Cleanup(func() { mc.Disconnect() })

	time.Sleep(500 * time.Millisecond)

	observer := newObserverClient(t, "observer-forward-text")

	payload, _ := json.Marshal(map[string]string{
		rule.ContentField: "hello integration test",
	})

	resultCh := make(chan []byte, 1)
	if token := observer.Subscribe(targetTopic, 0, func(_ mqtt.Client, msg mqtt.Message) {
		select {
		case resultCh <- msg.Payload():
		default:
		}
	}); token.Wait() && token.Error() != nil {
		t.Fatalf("observer subscribe: %v", token.Error())
	}
	time.Sleep(200 * time.Millisecond)

	if token := observer.Publish(sourceTopic, 0, false, payload); token.Wait() && token.Error() != nil {
		t.Fatalf("publish: %v", token.Error())
	}

	select {
	case result := <-resultCh:
		var got map[string]string
		if err := json.Unmarshal(result, &got); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if got[rule.ContentField] != "hello integration test" {
			t.Errorf("content = %q, want %q", got[rule.ContentField], "hello integration test")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: no forwarded message received")
	}
}

func TestForwardDedup(t *testing.T) {
	cfg := loadTestConfig(t)
	cfg.Filter.Time = "10s"

	rule := cfg.Forward[0]
	sourceTopic := rule.From[0]
	targetTopic := rule.To[0]

	f := filter.New(cfg.FilterDuration())
	engine := forward.NewEngine(cfg, f, nil, true)
	mc := mqttclient.New(cfg, engine.HandleMessage, true)
	engine.SetPublisher(mc)

	if err := mc.Connect(); err != nil {
		t.Fatalf("service connect: %v", err)
	}
	t.Cleanup(func() { mc.Disconnect() })

	time.Sleep(500 * time.Millisecond)

	observer := newObserverClient(t, "observer-dedup")

	payload, _ := json.Marshal(map[string]string{
		rule.ContentField: "dedup-test-content",
	})

	var received [][]byte
	done := make(chan struct{})
	if token := observer.Subscribe(targetTopic, 0, func(_ mqtt.Client, msg mqtt.Message) {
		received = append(received, msg.Payload())
		if len(received) >= 2 {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	}); token.Wait() && token.Error() != nil {
		t.Fatalf("observer subscribe: %v", token.Error())
	}
	time.Sleep(200 * time.Millisecond)

	for i := range 2 {
		if token := observer.Publish(sourceTopic, 0, false, payload); token.Wait() && token.Error() != nil {
			t.Fatalf("publish %d: %v", i, token.Error())
		}
		time.Sleep(200 * time.Millisecond)
	}

	select {
	case <-done:
		t.Errorf("dedup failed: received %d messages, expected 1", len(received))
	case <-time.After(3 * time.Second):
		if len(received) != 1 {
			t.Errorf("expected exactly 1 forwarded message, got %d", len(received))
		}
	}
}

func TestHTTPAPIRoundTrip(t *testing.T) {
	cfg := loadTestConfig(t)
	s := store.New(cfg.Storage.MaxRecords, cfg.StorageExpire())
	srv := httpserver.New(cfg, s)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := cfg.Clients[0]

	// PUT with image content type
	imgData := []byte("fake-png-data")
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/client/"+client.Name+"/img1", bytes.NewReader(imgData))
	req.Header.Set("Authorization", "Bearer "+client.Token)
	req.Header.Set("Content-Type", "image/png")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want 204", resp.StatusCode)
	}

	// GET - should return same Content-Type
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/client/"+client.Name+"/img1", nil)
	req.Header.Set("Authorization", "Bearer "+client.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, imgData) {
		t.Errorf("body mismatch")
	}
}
