// Package main is the entrypoint for the TailWake Wake-on-LAN server utility.
package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tailwake/tailwake/pkg/config"
	"github.com/tailwake/tailwake/pkg/service"
	"github.com/tailwake/tailwake/pkg/web"
	"github.com/tailwake/tailwake/pkg/wol"
)

// Version info
const Version = "1.0.0"

//go:embed web/*
var staticEmbed embed.FS

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "serve":
		runServe(os.Args[2:])
	case "wake":
		runWake(os.Args[2:])
	case "service":
		runService(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("TailWake v%s\n", Version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`TailWake - Wake-on-LAN HTTP Daemon & Web UI over Tailscale / LAN

Usage:
  tailwake <command> [flags]

Commands:
  serve              Start the HTTP web daemon and REST API server
  wake <device_id>   Send a Wake-on-LAN packet directly from CLI
  service install    Install and register as a background daemon (macOS launchd)
  service uninstall  Remove the background daemon
  version            Show version information

Flags for 'serve':
  -c, --config       Path to config.json file (default "config.json")
  -p, --port         HTTP server listening port (overrides config)
  -b, --bind         Bind IP address (default "0.0.0.0")

Examples:
  tailwake serve --config config.json
  tailwake wake macbook-pro
  tailwake service install`)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "config.json", "Path to config.json")
	fs.StringVar(configPath, "c", "config.json", "Path to config.json (shorthand)")
	portFlag := fs.Int("port", 0, "HTTP server listening port")
	fs.IntVar(portFlag, "p", 0, "HTTP server listening port (shorthand)")
	bindFlag := fs.String("bind", "", "Bind IP address")
	fs.StringVar(bindFlag, "b", "", "Bind IP address (shorthand)")

	_ = fs.Parse(args)

	// Attempt to create example config if no config file exists yet
	if _, err := os.Stat(*configPath); os.IsNotExist(err) && *configPath == "config.json" {
		createSampleConfig(*configPath)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("❌ Error loading config: %v", err)
	}

	if *portFlag > 0 {
		cfg.Port = *portFlag
	}
	if *bindFlag != "" {
		cfg.BindAddress = *bindFlag
	}

	srv, err := web.NewServer(cfg, staticEmbed)
	if err != nil {
		log.Fatalf("❌ Failed to initialize web server: %v", err)
	}

	listenAddr := fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.Port)
	log.Printf("🚀 Starting TailWake v%s on http://%s", Version, listenAddr)
	log.Printf("📋 Loaded %d device(s) from %s", len(cfg.Devices), *configPath)
	if cfg.AuthToken != "" {
		log.Printf("🔒 Auth Token enabled (protecting API endpoints)")
	}

	httpServer := &http.Server{
		Addr:         listenAddr,
		Handler:      srv.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Server error: %v", err)
	}
}

func runWake(args []string) {
	fs := flag.NewFlagSet("wake", flag.ExitOnError)
	configPath := fs.String("config", "config.json", "Path to config.json")
	_ = fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) == 0 {
		log.Fatal("❌ Device ID or MAC address required. Example: tailwake wake macbook-pro")
	}

	target := remaining[0]

	// Check if target is a MAC address directly
	if wol.ValidateMAC(target) {
		err := wol.SendMagicPacket(target, "255.255.255.255", 9)
		if err != nil {
			log.Fatalf("❌ Failed to send WoL packet: %v", err)
		}
		fmt.Printf("⚡ Magic packet sent to MAC %s\n", target)
		return
	}

	// Otherwise, lookup device ID in config
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("❌ Error loading config: %v", err)
	}

	dev, found := cfg.GetDeviceByID(target)
	if !found {
		log.Fatalf("❌ Device '%s' not found in %s", target, *configPath)
	}

	err = wol.SendMagicPacket(dev.MAC, dev.BroadcastIP, dev.WOLPort)
	if err != nil {
		log.Fatalf("❌ Failed to send magic packet: %v", err)
	}

	fmt.Printf("⚡ WoL magic packet successfully transmitted to %s (%s)\n", dev.Name, dev.MAC)
}

func runService(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: tailwake service <install|uninstall> [--config path]")
		os.Exit(1)
	}

	action := args[0]
	configPath := "config.json"

	if len(args) >= 3 && (args[1] == "--config" || args[1] == "-c") {
		configPath = args[2]
	}

	switch action {
	case "install":
		err := service.InstallLaunchdService(configPath)
		if err != nil {
			log.Fatalf("❌ Failed to install launchd service: %v", err)
		}
	case "uninstall":
		err := service.UninstallLaunchdService()
		if err != nil {
			log.Fatalf("❌ Failed to uninstall launchd service: %v", err)
		}
	default:
		fmt.Printf("Unknown service command: %s. Use 'install' or 'uninstall'.\n", action)
		os.Exit(1)
	}
}

func createSampleConfig(path string) {
	sample := `{
  "port": 8080,
  "bind_address": "0.0.0.0",
  "auth_token": "",
  "devices": [
    {
      "id": "macbook-pro",
      "name": "MacBook Pro",
      "mac": "AA:BB:CC:DD:EE:FF",
      "broadcast_ip": "255.255.255.255",
      "wol_port": 9,
      "ping_host": "192.168.1.50",
      "ping_port": 22
    }
  ]
}`
	absPath, _ := filepath.Abs(path)
	_ = os.WriteFile(path, []byte(sample), 0644)
	fmt.Printf("📝 Created default config file at %s\n", absPath)
}
