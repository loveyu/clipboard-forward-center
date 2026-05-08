package forward

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gopkg.in/yaml.v3"

	"clipboard-forward-center/internal/config"
	"clipboard-forward-center/internal/filter"
)

type Publisher interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) error
}

type Engine struct {
	cfg    *config.Config
	filter *filter.Filter
	pub    Publisher
	debug  bool
}

func NewEngine(cfg *config.Config, f *filter.Filter, pub Publisher, debug bool) *Engine {
	return &Engine{
		cfg:    cfg,
		filter: f,
		pub:    pub,
		debug:  debug,
	}
}

func (e *Engine) HandleMessage(topic string, payload []byte) {
	for i := range e.cfg.Forward {
		rule := &e.cfg.Forward[i]

		matched := false
		for _, from := range rule.From {
			if from == topic {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		content, err := e.extractContent(payload, rule)
		if err != nil {
			log.Printf("forward: extract content from %s: %v", topic, err)
			continue
		}

		hash := filter.ComputeHash(rule.Type, content)

		sourceClient := clientFromTopic(topic)
		if sourceClient != "" {
			e.filter.Record(sourceClient, hash)
		}

		for _, to := range rule.To {
			targetClient := clientFromTopic(to)
			if targetClient == "" {
				continue
			}

			if !e.filter.ShouldForward(targetClient, hash) {
				if e.debug {
					log.Printf("forward: filter duplicate to %s (hash=%s)", targetClient, hash[:16])
				}
				continue
			}

			if e.pub == nil {
				continue
			}
			if err := e.pub.Publish(to, 0, false, payload); err != nil {
				log.Printf("forward: publish to %s: %v", to, err)
				continue
			}

			e.filter.Record(targetClient, hash)

			if e.debug {
				log.Printf("forward: %s -> %s (type=%s)", topic, to, rule.Type)
			}
		}
	}
}

func (e *Engine) extractContent(payload []byte, rule *config.ForwardRule) (string, error) {
	if rule.ContentField == "" {
		return string(payload), nil
	}

	var data map[string]interface{}

	switch rule.Format {
	case "yaml":
		if err := yaml.Unmarshal(payload, &data); err != nil {
			return "", fmt.Errorf("parse yaml: %w", err)
		}
	default: // json or empty
		if err := json.Unmarshal(payload, &data); err != nil {
			return "", fmt.Errorf("parse json: %w", err)
		}
	}

	v, ok := data[rule.ContentField]
	if !ok {
		return "", fmt.Errorf("field %q not found", rule.ContentField)
	}
	return valueToString(v), nil
}

func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}

func clientFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
}
