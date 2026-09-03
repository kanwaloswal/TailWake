// Package web implements the HTTP web server, REST API, and static file embedder for TailWake.
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/tailwake/tailwake/pkg/config"
	"github.com/tailwake/tailwake/pkg/status"
	"github.com/tailwake/tailwake/pkg/wol"
)

// Server encapsulates the HTTP routing, configuration, and WoL engine state.
type Server struct {
	cfg     *config.Config
	monitor *status.Monitor
	fs      http.FileSystem
}

// NewServer initializes the HTTP server with embedded web assets.
func NewServer(cfg *config.Config, staticEmbed embed.FS) (*Server, error) {
	// Extract sub-filesystem under "web" directory
	webSubFS, err := fs.Sub(staticEmbed, "web")
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded static filesystem: %w", err)
	}

	return &Server{
		cfg:     cfg,
		monitor: status.NewMonitor(0),
		fs:      http.FS(webSubFS),
	}, nil
}

// Routes constructs the HTTP handler routes.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("GET /api/devices", s.requireAuth(s.handleGetDevices))
	mux.HandleFunc("POST /api/wake/{id}", s.requireAuth(s.handleWakeDevice))
	mux.HandleFunc("GET /api/wake/{id}", s.requireAuth(s.handleWakeDevice))

	// Simple GET URL Trigger (for quick browser bookmarks, Siri Shortcuts, iOS Action Buttons)
	mux.HandleFunc("GET /wake/{id}", s.requireAuth(s.handleWakeDevice))

	// Static Web UI Assets
	fileServer := http.FileServer(s.fs)
	mux.Handle("GET /", fileServer)

	return mux
}

// Auth middleware enforcing TAILWAKE_TOKEN if configured.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthToken != "" {
			// Check query parameter ?token=secret
			queryToken := r.URL.Query().Get("token")

			// Check Header Authorization: Bearer secret
			authHeader := r.Header.Get("Authorization")
			headerToken := ""
			if strings.HasPrefix(authHeader, "Bearer ") {
				headerToken = strings.TrimPrefix(authHeader, "Bearer ")
			}

			if queryToken != s.cfg.AuthToken && headerToken != s.cfg.AuthToken {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Unauthorized: Invalid or missing auth token",
				})
				return
			}
		}

		next(w, r)
	}
}

// DeviceResponse format for JSON serialization
type DeviceResponse struct {
	config.Device
	Status    string `json:"status"`
	CheckErr  string `json:"check_error,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
}

// GET /api/devices
func (s *Server) handleGetDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	responses := make([]DeviceResponse, 0, len(s.cfg.Devices))

	for _, dev := range s.cfg.Devices {
		devState := s.monitor.CheckDevice(dev)

		resp := DeviceResponse{
			Device:   dev,
			Status:   string(devState.Status),
			CheckErr: devState.CheckError,
		}

		if devState.LastSeen != nil {
			resp.LastSeen = devState.LastSeen.Format("2006-01-02 15:04:05")
		}

		responses = append(responses, resp)
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"devices": responses,
	})
}

// POST /api/wake/{id} or GET /wake/{id}
func (s *Server) handleWakeDevice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	deviceID := r.PathValue("id")
	if deviceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Device ID parameter is required",
		})
		return
	}

	dev, found := s.cfg.GetDeviceByID(deviceID)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Device '%s' not found in configuration", deviceID),
		})
		return
	}

	// Validate MAC Address before sending
	if !wol.ValidateMAC(dev.MAC) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid MAC address '%s' configured for device '%s'", dev.MAC, dev.Name),
		})
		return
	}

	// Transmit Wake-on-LAN magic packet over local broadcast UDP
	err := wol.SendMagicPacket(dev.MAC, dev.BroadcastIP, dev.WOLPort)
	if err != nil {
		log.Printf("❌ Failed to send WoL packet to %s (%s): %v", dev.Name, dev.MAC, err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to transmit magic packet: %v", err),
		})
		return
	}

	// Mark status monitor as waking
	s.monitor.SetWaking(dev.ID)

	log.Printf("⚡ WoL magic packet successfully transmitted to %s (%s) via %s:%d", dev.Name, dev.MAC, dev.BroadcastIP, dev.WOLPort)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("Magic packet transmitted to %s", dev.Name),
		"device_id": dev.ID,
		"mac":       dev.MAC,
	})
}
