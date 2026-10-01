package api

import (
	"fmt"
	"net/http"
	"sync"
)

type TouchBarHub struct {
	mu        sync.RWMutex
	listeners map[chan string]struct{}
	onState   func(active bool)
}

var GlobalTouchBarHub = &TouchBarHub{
	listeners: make(map[chan string]struct{}),
}

func (h *TouchBarHub) SetStateCallback(cb func(active bool)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onState = cb
}

func (h *TouchBarHub) BroadcastKey(key string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.listeners {
		select {
		case ch <- key:
		default:
		}
	}
}

func (h *TouchBarHub) SetState(active bool) {
	h.mu.RLock()
	cb := h.onState
	h.mu.RUnlock()
	if cb != nil {
		cb(active)
	}
}

// HandleTouchBarPress receives key press from physical touchbar agent: POST /api/touchbar/press?key=F1
func HandleTouchBarPress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		key = r.FormValue("key")
	}
	if key != "" {
		GlobalTouchBarHub.BroadcastKey(key)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// HandleTouchBarState toggles physical Touch Bar visibility: POST /api/touchbar/state?active=1
func HandleTouchBarState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	active := r.URL.Query().Get("active") == "1" || r.URL.Query().Get("active") == "true"
	GlobalTouchBarHub.SetState(active)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// HandleTouchBarEvents provides Server-Sent Events (SSE) for web app: GET /api/touchbar/events
func HandleTouchBarEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan string, 16)
	GlobalTouchBarHub.mu.Lock()
	GlobalTouchBarHub.listeners[ch] = struct{}{}
	GlobalTouchBarHub.mu.Unlock()

	defer func() {
		GlobalTouchBarHub.mu.Lock()
		delete(GlobalTouchBarHub.listeners, ch)
		GlobalTouchBarHub.mu.Unlock()
		close(ch)
	}()

	// Send initial ping
	fmt.Fprintf(w, ":connected\n\n")
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case key, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", key)
			flusher.Flush()
		}
	}
}
