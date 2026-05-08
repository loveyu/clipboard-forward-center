package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"clipboard-forward-center/internal/config"
	"clipboard-forward-center/internal/filter"
	"clipboard-forward-center/internal/forward"
	"clipboard-forward-center/internal/httpserver"
	"clipboard-forward-center/internal/mqttclient"
	"clipboard-forward-center/internal/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "start")
	}

	switch os.Args[1] {
	case "start":
		runStart()
	case "help":
		runHelp()
	case "download-config":
		runDownloadConfig()
	case "version":
		fmt.Println("clipboard-forward-center", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nUse 'help' for usage.\n", os.Args[1])
		os.Exit(1)
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
	s := store.New(cfg.Storage.MaxRecords, cfg.StorageExpire())

	engine := forward.NewEngine(cfg, f, nil, debug)
	mqttClient := mqttclient.New(cfg, engine.HandleMessage, debug)
	engine.SetPublisher(mqttClient)

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
	mqttClient.Disconnect()
}

func runHelp() {
	fmt.Print(`clipboard-forward-center - Clipboard message forwarding center

Usage:
  clipboard-forward-center [command]

Commands:
  start            Start the service (default)
  help             Show this help message
  download-config  Download remote config from REMOTE_CONFIG_URL
  version          Show version

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
