package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type DiscoveredHost struct {
	IP        string `json:"ip"`
	Hostname  string `json:"hostname"`
	Port      int    `json:"port"`
	LatencyMs int    `json:"latencyMs"`
	IsOpen    bool   `json:"isOpen"`
}

// HandleDiscover scans the local network or specified subnet for active RDP (3389) hosts.
func HandleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	subnetQuery := r.URL.Query().Get("subnet")
	var ipPrefix string

	if subnetQuery != "" {
		parts := strings.Split(subnetQuery, ".")
		if len(parts) >= 3 {
			ipPrefix = fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])
		}
	}

	if ipPrefix == "" {
		// Auto-detect local active network interface
		ipPrefix = detectLocalSubnetPrefix()
	}

	if ipPrefix == "" {
		jsonResponse(w, http.StatusOK, []DiscoveredHost{})
		return
	}

	hosts := scanSubnetForRDP(ipPrefix)
	jsonResponse(w, http.StatusOK, hosts)
}

func detectLocalSubnetPrefix() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			ipv4 := ipNet.IP.To4()
			if ipv4 != nil && (ipv4[0] == 192 || ipv4[0] == 10 || (ipv4[0] == 172 && ipv4[1] >= 16 && ipv4[1] <= 31)) {
				return fmt.Sprintf("%d.%d.%d", ipv4[0], ipv4[1], ipv4[2])
			}
		}
	}
	return ""
}

func scanSubnetForRDP(prefix string) []DiscoveredHost {
	var results []DiscoveredHost
	var mu sync.Mutex

	// Worker pool for 254 IPs
	const concurrency = 64
	ipChan := make(chan int, 254)
	for i := 1; i <= 254; i++ {
		ipChan <- i
	}
	close(ipChan)

	var wg sync.WaitGroup
	timeout := 280 * time.Millisecond

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for lastOctet := range ipChan {
				ip := fmt.Sprintf("%s.%d", prefix, lastOctet)
				target := fmt.Sprintf("%s:3389", ip)

				start := time.Now()
				conn, err := net.DialTimeout("tcp", target, timeout)
				if err == nil {
					latency := int(time.Since(start).Milliseconds())
					if latency == 0 {
						latency = 1
					}
					_ = conn.Close()

					hostname := resolveHostname(ip)

					mu.Lock()
					results = append(results, DiscoveredHost{
						IP:        ip,
						Hostname:  hostname,
						Port:      3389,
						LatencyMs: latency,
						IsOpen:    true,
					})
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	return results
}

func resolveHostname(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err == nil && len(names) > 0 {
		h := strings.TrimSuffix(names[0], ".")
		return h
	}
	return ""
}
