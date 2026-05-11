package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"clipboard-forward-center/internal/config"
	"clipboard-forward-center/internal/filter"
	"clipboard-forward-center/internal/forward"
	"clipboard-forward-center/internal/httpserver"
	"clipboard-forward-center/internal/mqttclient"
	"clipboard-forward-center/internal/store"
)

var version = "dev"

func main() {
	loadEnv(".env")

	if len(os.Args) < 2 {
		os.Args = append(os.Args, "start")
	}

	switch os.Args[1] {
	case "start":
		runStart()
	case "check":
		runCheck()
	case "help":
	case "--help":
		runHelp()
	case "download-config":
		runDownloadConfig()
	case "version":
	case "--version":
	case "-v":
		fmt.Println("clipboard-forward-center", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nUse 'help' for usage.\n", os.Args[1])
		os.Exit(1)
	}
}

func runCheck() {
	var maxTime time.Duration
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--max-time" && i+1 < len(args) {
			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				log.Fatalf("invalid --max-time: %v", err)
			}
			maxTime = d
			i++
		}
	}

	cfgPath := config.ConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	opts, err := cfg.MQTTOptions()
	if err != nil {
		log.Fatalf("parse DSN: %v", err)
	}
	log.Printf("checking MQTT broker: %s", opts.Broker)

	deadline := time.Now().Add(maxTime)
	if maxTime == 0 {
		deadline = time.Now()
	}

	for attempt := 1; ; attempt++ {
		mqttClient := mqttclient.New(cfg, nil, false)
		err := mqttClient.Connect()
		if err == nil {
			log.Printf("MQTT broker is available (attempt %d)", attempt)
			mqttClient.Disconnect()
			os.Exit(0)
		}

		log.Printf("attempt %d: connect failed: %v", attempt, err)

		if maxTime == 0 || time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "MQTT broker is not available")
			os.Exit(1)
		}

		time.Sleep(2 * time.Second)
	}
}

func runStart() {
	cfgPath := config.ConfigPath()
	debug := config.IsDebug()

	log.Printf("clipboard-forward-center %s starting...", version)
	log.Printf("config: %s", cfgPath)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	f := filter.New(cfg.FilterDuration())
	s := store.New(cfg.Storage.MaxRecords, cfg.StorageExpire(), cfg.Storage.MaxBodySize, cfg.Storage.FileStorageSize)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartCleanup(ctx)

	var engine *forward.Engine
	mqttClient := mqttclient.New(cfg, func(topic string, payload []byte) {
		engine.HandleMessage(topic, payload)
	}, debug)
	engine = forward.NewEngine(cfg, f, mqttClient, debug)

	log.Println("connecting to MQTT broker...")
	if err := mqttClient.Connect(); err != nil {
		log.Fatalf("mqtt connect: %v", err)
	}

	httpServer := httpserver.New(cfg, s)
	go func() {
		if err := httpServer.Start(cfg.HTTP.Addr); err != nil {
			log.Fatalf("http server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	cancel()
	s.Close()
	mqttClient.Disconnect()
}

func runHelp() {
	fmt.Print(`clipboard-forward-center - Clipboard message forwarding center

Usage:
  clipboard-forward-center [command]

Commands:
  start            Start the service (default)
  check            Check MQTT broker connectivity (exit 0 on success, 1 on failure)
  help             Show this help message
  download-config  Download remote config from REMOTE_CONFIG_URL
  version          Show version

Check Options:
  --max-time <duration>  Maximum time to keep retrying (e.g. 30s, 1m).
                         Default: single attempt, no retry.

Environment Variables:
  DEBUG             Enable debug logging
  REMOTE_CONFIG_URL URL for downloading config (used by download-config)
  CONFIG_PATH       Config file path (default: config.yaml)

HTTP API:
  PUT|POST /client/{client}/{msgId}  Write message (client must match token)
  GET      /client/{client}/{msgId}  Read message (any valid token)

`)
}

func runDownloadConfig() {
	url := config.RemoteConfigURL()
	if url == "" {
		log.Fatal("REMOTE_CONFIG_URL is not set")
	}

	cfgPath := config.ConfigPath()
	log.Printf("downloading config from %s to %s", url, url)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("download config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("download config: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		log.Fatalf("write config: %v", err)
	}

	log.Printf("config saved to %s (%d bytes)", cfgPath, len(data))
}

// loadEnv reads KEY=VALUE pairs from a .env file. Existing env vars are not overwritten.
// Lines starting with # and empty lines are skipped.
func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env file is optional
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if os.Getenv(key) != "" {
			continue
		}
		os.Setenv(key, value)
	}
}
