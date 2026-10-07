package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type WolRequest struct {
	MAC       string `json:"mac"`
	Broadcast string `json:"broadcast,omitempty"` // defaults to 255.255.255.255:9
}

// HandleWakeOnLan sends a WOL magic packet to wake up a sleeping remote computer.
func HandleWakeOnLan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req WolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.MAC == "" {
		http.Error(w, "mac address is required", http.StatusBadRequest)
		return
	}

	macAddr, err := net.ParseMAC(strings.TrimSpace(req.MAC))
	if err != nil || len(macAddr) != 6 {
		http.Error(w, fmt.Sprintf("invalid MAC address: %s", req.MAC), http.StatusBadRequest)
		return
	}

	// Build 102-byte Magic Packet: 6x 0xFF followed by 16x MAC
	var packet [102]byte
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 1; i <= 16; i++ {
		copy(packet[i*6:(i+1)*6], macAddr)
	}

	bcastAddr := req.Broadcast
	if bcastAddr == "" {
		bcastAddr = "255.255.255.255:9"
	}
	if !strings.Contains(bcastAddr, ":") {
		bcastAddr += ":9"
	}

	udpAddr, err := net.ResolveUDPAddr("udp", bcastAddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	if _, err := conn.Write(packet[:]); err != nil {
		http.Error(w, fmt.Sprintf("failed to send WOL packet: %v", err), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Magic packet sent to %s via %s", macAddr.String(), bcastAddr),
		"mac":     macAddr.String(),
	})
}
