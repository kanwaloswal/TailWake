// Package config handles loading and parsing the TailWake configuration.
// It supports loading settings from a JSON file, environment variables, or CLI flags.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// Device represents a machine that can be woken up via Wake-on-LAN.
// Go struct tags (`json:"..."`) map JSON key names to struct fields.
type Device struct {
	ID          string `json:"id"`           // Unique identifier (e.g., "macbook-pro")
	Name        string `json:"name"`         // Display name (e.g., "MacBook Pro M3")
	MAC         string `json:"mac"`          // Hardware MAC address (e.g., "AA:BB:CC:DD:EE:FF")
	BroadcastIP string `json:"broadcast_ip"` // Broadcast IP address (default "255.255.255.255")
	WOLPort     int    `json:"wol_port"`     // UDP port for WoL magic packet (default 9)
	PingHost    string `json:"ping_host"`    // IP/hostname used for status checking
	PingPort    int    `json:"ping_port"`    // TCP port to check for online status (e.g., 22 for SSH, 5900 for Screen Sharing)
}

// Config represents the application's overall runtime configuration.
type Config struct {
	Port        int      `json:"port"`         // HTTP server listening port (default 8080)
	BindAddress string   `json:"bind_address"` // IP interface to bind to (e.g. "0.0.0.0" or "127.0.0.1")
	AuthToken   string   `json:"auth_token"`   // Optional security secret token for API access
	Devices     []Device `json:"devices"`      // List of configured devices
}

// LoadConfig attempts to read configuration from the given file path.
// If the file does not exist, it falls back to sensible defaults.
func LoadConfig(path string) (*Config, error) {
	// Initialize default configuration
	cfg := &Config{
		Port:        8080,
		BindAddress: "0.0.0.0",
		AuthToken:   "",
		Devices:     []Device{},
	}

	// Check if configuration file exists
	if _, err := os.Stat(path); err == nil {
		fileBytes, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file at %s: %w", path, err)
		}

		if err := json.Unmarshal(fileBytes, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config JSON: %w", err)
		}
	} else if path != "config.json" {
		// Return error only if user specified a custom config path that is missing
		return nil, fmt.Errorf("config file not found at path: %s", path)
	}

	// Apply Environment Variable Overrides
	if envPort := os.Getenv("TAILWAKE_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			cfg.Port = p
		}
	}

	if envToken := os.Getenv("TAILWAKE_TOKEN"); envToken != "" {
		cfg.AuthToken = envToken
	}

	if envBind := os.Getenv("TAILWAKE_BIND"); envBind != "" {
		cfg.BindAddress = envBind
	}

	// Ensure sensible defaults for each device
	for i := range cfg.Devices {
		if cfg.Devices[i].BroadcastIP == "" {
			cfg.Devices[i].BroadcastIP = "255.255.255.255"
		}
		if cfg.Devices[i].WOLPort == 0 {
			cfg.Devices[i].WOLPort = 9
		}
	}

	return cfg, nil
}

// GetDeviceByID finds a device by its unique ID string.
func (c *Config) GetDeviceByID(id string) (*Device, bool) {
	for _, dev := range c.Devices {
		if dev.ID == id {
			return &dev, true
		}
	}
	return nil, false
}
