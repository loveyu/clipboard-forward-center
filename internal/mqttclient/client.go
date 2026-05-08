package mqttclient

import (
	"crypto/tls"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"clipboard-forward-center/internal/config"
)

type MessageHandler func(topic string, payload []byte)

type Client struct {
	client  mqtt.Client
	cfg     *config.Config
	handler MessageHandler
	debug   bool
}

func New(cfg *config.Config, handler MessageHandler, debug bool) *Client {
	opts, err := cfg.MQTTOptions()
	if err != nil {
		log.Fatalf("mqtt: parse options: %v", err)
	}

	mqttOpts := mqtt.NewClientOptions()
	mqttOpts.AddBroker(opts.Broker)
	mqttOpts.SetClientID(opts.ClientID)
	mqttOpts.SetConnectTimeout(opts.ConnectTimeout)
	mqttOpts.SetKeepAlive(opts.KeepAlive)
	mqttOpts.SetAutoReconnect(opts.AutoReconnect)
	mqttOpts.SetMaxReconnectInterval(opts.MaxReconnectInterval)

	if opts.Username != "" {
		mqttOpts.SetUsername(opts.Username)
		mqttOpts.SetPassword(opts.Password)
	}

	if opts.UseTLS {
		mqttOpts.SetTLSConfig(&tls.Config{})
	}

	c := &Client{
		cfg:     cfg,
		handler: handler,
		debug:   debug,
	}

	mqttOpts.SetOnConnectHandler(func(_ mqtt.Client) {
		log.Println("mqtt: connected")
		c.subscribeAll()
	})

	mqttOpts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		log.Printf("mqtt: connection lost: %v", err)
	})

	c.client = mqtt.NewClient(mqttOpts)
	return c
}

func (c *Client) Connect() error {
	token := c.client.Connect()
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("connect: %w", token.Error())
	}
	return nil
}

func (c *Client) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	token := c.client.Publish(topic, qos, retained, payload)
	token.Wait()
	return token.Error()
}

func (c *Client) Disconnect() {
	c.client.Disconnect(1000)
	log.Println("mqtt: disconnected")
}

func (c *Client) subscribeAll() {
	for i := range c.cfg.Forward {
		rule := &c.cfg.Forward[i]
		for _, topic := range rule.From {
			t := c.client.Subscribe(topic, c.cfg.DefaultQoS(), func(_ mqtt.Client, msg mqtt.Message) {
				if c.debug {
					log.Printf("mqtt: recv %s (%d bytes)", msg.Topic(), len(msg.Payload()))
				}
				if c.handler != nil {
					c.handler(msg.Topic(), msg.Payload())
				}
			})
			t.Wait()
			if t.Error() != nil {
				log.Printf("mqtt: subscribe %s: %v", topic, t.Error())
			} else if c.debug {
				log.Printf("mqtt: subscribed %s", topic)
			}
		}
	}
}
