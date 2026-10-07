package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"rdp-app/internal/store"
)

type APIHandler struct {
	store *store.Store
}

func NewAPIHandler(s *store.Store) *APIHandler {
	return &APIHandler{store: s}
}

func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/devices", h.handleDevices)
	mux.HandleFunc("/api/devices/", h.handleDeviceByID)
	mux.HandleFunc("/api/ping", h.handlePingAll)
	mux.HandleFunc("/api/system", h.handleSystemInfo)
	mux.HandleFunc("/api/discover", HandleDiscover)
	mux.HandleFunc("/api/wol", HandleWakeOnLan)
	mux.HandleFunc("/api/files/upload", h.handleFileUpload)
	mux.HandleFunc("/api/files/list", h.handleFileList)
	mux.HandleFunc("/api/files/download/", h.handleFileDownload)
	mux.HandleFunc("/api/files/delete/", h.handleFileDelete)
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *APIHandler) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		devices := h.store.GetAll()
		for _, d := range devices {
			if d.Password != "" {
				d.Password = "••••••••"
			}
		}
		jsonResponse(w, http.StatusOK, devices)

	case http.MethodPost:
		var dev store.Device
		if err := json.NewDecoder(r.Body).Decode(&dev); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if dev.Name == "" || dev.Host == "" {
			http.Error(w, "name and host are required", http.StatusBadRequest)
			return
		}
		if err := h.store.Save(&dev); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respDev := dev
		if respDev.Password != "" {
			respDev.Password = "••••••••"
		}
		jsonResponse(w, http.StatusCreated, respDev)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleDeviceByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	parts := strings.Split(path, "/")
	id := parts[0]

	if id == "" {
		http.Error(w, "missing device id", http.StatusBadRequest)
		return
	}

	// Sub-actions: /api/devices/{id}/favorite, /api/devices/{id}/touch, /api/devices/{id}/credentials
	if len(parts) > 1 {
		subAction := parts[1]
		switch subAction {
		case "favorite":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			isFav, err := h.store.ToggleFavorite(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]any{"id": id, "favorite": isFav})
			return

		case "touch":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			h.store.UpdateLastConnected(id)
			jsonResponse(w, http.StatusOK, map[string]any{"id": id, "touched": true})
			return

		case "credentials":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			dev, found := h.store.Get(id)
			if !found {
				http.Error(w, "device not found", http.StatusNotFound)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]any{
				"id":          dev.ID,
				"username":    dev.Username,
				"password":    dev.Password,
				"domain":      dev.Domain,
				"hasPassword": dev.Password != "",
			})
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		dev, found := h.store.Get(id)
		if !found {
			http.Error(w, "device not found", http.StatusNotFound)
			return
		}
		respDev := *dev
		if respDev.Password != "" {
			respDev.Password = "••••••••"
		}
		jsonResponse(w, http.StatusOK, respDev)

	case http.MethodPut:
		var dev store.Device
		if err := json.NewDecoder(r.Body).Decode(&dev); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		dev.ID = id

		// If password is masked or unchanged, keep existing password
		if dev.Password == "••••••••" || dev.Password == "" {
			if existing, ok := h.store.Get(id); ok && existing.Password != "" {
				dev.Password = existing.Password
			}
		}

		if err := h.store.Save(&dev); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respDev := dev
		if respDev.Password != "" {
			respDev.Password = "••••••••"
		}
		jsonResponse(w, http.StatusOK, respDev)

	case http.MethodDelete:
		if err := h.store.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "deleted", "id": id})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type PingResult struct {
	ID        string `json:"id"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Status    string `json:"status"`
	LatencyMs int    `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

func (h *APIHandler) handlePingAll(w http.ResponseWriter, r *http.Request) {
	devices := h.store.GetAll()
	results := make([]PingResult, len(devices))

	var wg sync.WaitGroup
	wg.Add(len(devices))

	for i, dev := range devices {
		go func(idx int, d *store.Device) {
			defer wg.Done()
			addr := fmt.Sprintf("%s:%d", d.Host, d.Port)
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, 1200*time.Millisecond)
			duration := time.Since(start)

			res := PingResult{
				ID:   d.ID,
				Host: d.Host,
				Port: d.Port,
			}

			if err != nil {
				res.Status = "offline"
				res.LatencyMs = 0
				res.Error = err.Error()
				h.store.UpdateStatus(d.ID, "offline", 0)
			} else {
				_ = conn.Close()
				latency := int(duration.Milliseconds())
				if latency <= 0 {
					latency = 1
				}
				res.Status = "online"
				res.LatencyMs = latency
				h.store.UpdateStatus(d.ID, "online", latency)
			}
			results[idx] = res
		}(i, dev)
	}

	wg.Wait()
	jsonResponse(w, http.StatusOK, results)
}

func (h *APIHandler) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]any{
		"app":       "Rdp Sensei",
		"version":   "1.0.0",
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"goVersion": runtime.Version(),
		"pureGo":    true,
		"noPlugins": true,
	}
	jsonResponse(w, http.StatusOK, info)
}
