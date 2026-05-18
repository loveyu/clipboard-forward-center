package forward

import (
	"sync"
	"testing"
	"time"

	"clipboard-forward-center/internal/config"
	"clipboard-forward-center/internal/filter"
)

type mockPublisher struct {
	mu      sync.Mutex
	publishes []struct {
		topic   string
		payload string
	}
}

func (m *mockPublisher) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishes = append(m.publishes, struct {
		topic   string
		payload string
	}{topic: topic, payload: string(payload.([]byte))})
	return nil
}

func testEngine(rules []config.ForwardRule) (*Engine, *mockPublisher) {
	cfg := &config.Config{
		Forward: rules,
	}
	f := filter.New(5 * time.Second)
	pub := &mockPublisher{}
	return NewEngine(cfg, f, pub, false), pub
}

func TestSelfForwardSkipped(t *testing.T) {
	engine, pub := testEngine([]config.ForwardRule{
		{
			From:         []string{"clipboard-in-text/mobile-k50", "clipboard-in-text/work-min-debian"},
			To:           []string{"clipboard-out-text/mobile-k50", "clipboard-out-text/work-min-debian"},
			Type:         "text",
			ContentField: "",
		},
	})

	engine.HandleMessage("clipboard-in-text/mobile-k50", []byte("hello"))

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.publishes) != 1 {
		t.Fatalf("expected 1 publish (skip self-forward), got %d", len(pub.publishes))
	}
	if pub.publishes[0].topic != "clipboard-out-text/work-min-debian" {
		t.Errorf("expected publish to work-min-debian, got %s", pub.publishes[0].topic)
	}
}

func TestSelfForwardSkippedAllDevices(t *testing.T) {
	engine, pub := testEngine([]config.ForwardRule{
		{
			From:         []string{"clipboard-in-text/mobile-k50"},
			To:           []string{"clipboard-out-text/mobile-k50"},
			Type:         "text",
			ContentField: "",
		},
	})

	engine.HandleMessage("clipboard-in-text/mobile-k50", []byte("hello"))

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.publishes) != 0 {
		t.Fatalf("expected 0 publishes (self-forward only), got %d", len(pub.publishes))
	}
}

func TestCrossDeviceForwarding(t *testing.T) {
	engine, pub := testEngine([]config.ForwardRule{
		{
			From:         []string{"clipboard-in-text/mobile-k50", "clipboard-in-text/work-min-debian", "clipboard-in-text/notebook-debian"},
			To:           []string{"clipboard-out-text/mobile-k50", "clipboard-out-text/work-min-debian", "clipboard-out-text/notebook-debian"},
			Type:         "text",
			ContentField: "",
		},
	})

	engine.HandleMessage("clipboard-in-text/mobile-k50", []byte("hello"))

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.publishes) != 2 {
		t.Fatalf("expected 2 publishes (to other 2 devices), got %d", len(pub.publishes))
	}

	topics := map[string]bool{}
	for _, p := range pub.publishes {
		topics[p.topic] = true
	}
	if topics["clipboard-out-text/mobile-k50"] {
		t.Error("should not forward to self (mobile-k50)")
	}
	if !topics["clipboard-out-text/work-min-debian"] {
		t.Error("should forward to work-min-debian")
	}
	if !topics["clipboard-out-text/notebook-debian"] {
		t.Error("should forward to notebook-debian")
	}
}

func TestSelfForwardImageRule(t *testing.T) {
	engine, pub := testEngine([]config.ForwardRule{
		{
			From:         []string{"clipboard-in-image/mobile-k50", "clipboard-in-image/work-min-debian"},
			To:           []string{"clipboard-out-image/work-min-debian", "clipboard-out-image/notebook-debian"},
			Type:         "image",
			ContentField: "",
		},
	})

	engine.HandleMessage("clipboard-in-image/work-min-debian", []byte("img-data"))

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.publishes) != 1 {
		t.Fatalf("expected 1 publish (skip self-forward to work-min-debian), got %d", len(pub.publishes))
	}
	if pub.publishes[0].topic != "clipboard-out-image/notebook-debian" {
		t.Errorf("expected publish to notebook-debian, got %s", pub.publishes[0].topic)
	}
}

func TestDebugLogSelfForward(t *testing.T) {
	cfg := &config.Config{
		Forward: []config.ForwardRule{
			{
				From:         []string{"clipboard-in-text/mobile-k50"},
				To:           []string{"clipboard-out-text/mobile-k50", "clipboard-out-text/work-pc"},
				Type:         "text",
				ContentField: "",
			},
		},
	}
	f := filter.New(5 * time.Second)
	pub := &mockPublisher{}
	engine := NewEngine(cfg, f, pub, true)

	engine.HandleMessage("clipboard-in-text/mobile-k50", []byte("hello"))

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.publishes) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(pub.publishes))
	}
	if pub.publishes[0].topic != "clipboard-out-text/work-pc" {
		t.Errorf("expected publish to work-pc, got %s", pub.publishes[0].topic)
	}
}

func TestNoMatchingRule(t *testing.T) {
	engine, pub := testEngine([]config.ForwardRule{
		{
			From:         []string{"clipboard-in-text/mobile-k50"},
			To:           []string{"clipboard-out-text/mobile-k50"},
			Type:         "text",
			ContentField: "",
		},
	})

	engine.HandleMessage("clipboard-in-text/unknown-device", []byte("hello"))

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.publishes) != 0 {
		t.Fatalf("expected 0 publishes for unmatched topic, got %d", len(pub.publishes))
	}
}
