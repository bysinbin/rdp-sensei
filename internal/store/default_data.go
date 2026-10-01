package store

// DefaultDevices returns clean template devices for initial setup.
// No sensitive credentials or private network IPs are stored in source control.
func DefaultDevices() []*Device {
	return []*Device{
		{
			ID:          "dev-demo-1",
			Name:        "Workstation 1",
			Host:        "192.168.1.100",
			Port:        3389,
			Username:    "",
			Domain:      "",
			Group:       "Office",
			Favorite:    true,
			Width:       1920,
			Height:      1080,
			SwapAltMeta: false,
			EnableAudio: true,
			Status:      "offline",
			LatencyMs:   0,
		},
		{
			ID:          "dev-demo-2",
			Name:        "Dev Server",
			Host:        "192.168.1.150",
			Port:        3389,
			Username:    "",
			Domain:      "",
			Group:       "Servers",
			Favorite:    true,
			Width:       1440,
			Height:      900,
			SwapAltMeta: false,
			EnableAudio: true,
			Status:      "offline",
			LatencyMs:   0,
		},
		{
			ID:          "dev-demo-3",
			Name:        "Local Test PC",
			Host:        "127.0.0.1",
			Port:        3389,
			Username:    "",
			Domain:      "",
			Group:       "Saved Devices",
			Favorite:    false,
			Width:       1440,
			Height:      900,
			SwapAltMeta: false,
			EnableAudio: false,
			Status:      "offline",
			LatencyMs:   0,
		},
	}
}
