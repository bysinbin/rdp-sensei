package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"rdp-app/internal/api"
	"rdp-app/internal/desktop"
	"rdp-app/internal/store"
)

//go:embed all:static
var staticFS embed.FS

func main() {
	portFlag := flag.Int("port", 8080, "Port to listen on (default 8080)")
	hostFlag := flag.String("host", "", "Host address to bind to (default 127.0.0.1 for desktop, 0.0.0.0 for web)")
	webMode := flag.Bool("web", false, "Run in Web Server mode (does not launch desktop window)")
	dataDirFlag := flag.String("data", "", "Data directory path for storing devices (default ~/.rdp-app)")
	flag.Parse()

	// Determine data directory
	dataDir := *dataDirFlag
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			dataDir = "./data"
		} else {
			dataDir = filepath.Join(homeDir, ".rdp-app")
		}
	}

	// Initialize store
	st, err := store.NewStore(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Determine host address
	bindHost := *hostFlag
	if bindHost == "" {
		if *webMode {
			bindHost = "0.0.0.0"
		} else {
			bindHost = "127.0.0.1"
		}
	}

	addr := fmt.Sprintf("%s:%d", bindHost, *portFlag)

	// Prepare router
	mux := http.NewServeMux()

	// 1. WebSocket Proxy
	mux.HandleFunc("/ws", api.HandleWebSocketProxy)

	// 2. REST API Handlers
	apiHandler := api.NewAPIHandler(st)
	apiHandler.RegisterRoutes(mux)

	// 3. TouchBar Endpoints (Physical Touch Bar integration)
	mux.HandleFunc("/api/touchbar/press", api.HandleTouchBarPress)
	mux.HandleFunc("/api/touchbar/state", api.HandleTouchBarState)
	mux.HandleFunc("/api/touchbar/events", api.HandleTouchBarEvents)

	// 4. Static Assets (embedded)
	subStatic, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("Failed to load static assets: %v", err)
	}
	fileServer := http.FileServer(http.FS(subStatic))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) == ".wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		}
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		fileServer.ServeHTTP(w, r)
	})

	// Check if port is available
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// If 8080 is occupied, pick an available port
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:0", bindHost))
		if err != nil {
			log.Fatalf("Failed to bind port: %v", err)
		}
		addr = listener.Addr().String()
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	serverURL := fmt.Sprintf("http://%s", addr)
	if bindHost == "0.0.0.0" {
		serverURL = fmt.Sprintf("http://localhost:%d", actualPort)
	}

	// Launch physical macOS Touch Bar agent if available
	tbProc := desktop.StartPhysicalTouchBarAgent(actualPort)
	if tbProc != nil {
		defer tbProc.Stop()
		api.GlobalTouchBarHub.SetStateCallback(func(active bool) {
			tbProc.SetState(active)
		})
	}

	fmt.Println("==================================================")
	fmt.Println("🥋 RDP SENSEI - Remote Desktop Platform")
	fmt.Println("==================================================")
	fmt.Printf("📍 Local URL     : %s\n", serverURL)
	fmt.Printf("📁 Data Storage  : %s\n", dataDir)
	if *webMode {
		fmt.Printf("🌐 Mode          : Web Server (accessible on network)\n")
	} else {
		fmt.Printf("💻 Mode          : Desktop App (native window)\n")
	}
	fmt.Println("🔒 Engine        : Pure Go WASM RDP (Zero external dependencies)")
	fmt.Println("==================================================")

	// Launch desktop window if not in web-only mode
	if !*webMode {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := desktop.OpenDesktopWindow(serverURL); err != nil {
				log.Printf("Note: desktop launcher fallback: %v", err)
			}
		}()
	}

	server := &http.Server{
		Handler: mux,
	}

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
