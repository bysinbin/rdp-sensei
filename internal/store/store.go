package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password,omitempty"`
	Domain        string `json:"domain"`
	Group         string `json:"group"`
	Favorite      bool   `json:"favorite"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	SwapAltMeta   bool   `json:"swapAltMeta"`
	MacShortcuts  bool   `json:"macShortcuts"`
	MacAddress    string `json:"macAddress,omitempty"`
	EnableAudio   bool   `json:"enableAudio"`
	Thumbnail     string `json:"thumbnail"`
	LastConnected string `json:"lastConnected"`
	CreatedAt     string `json:"createdAt"`
	Status        string `json:"status"` // "online", "offline", "unknown"
	LatencyMs     int    `json:"latencyMs"`
}

type Store struct {
	filePath    string
	lastModTime time.Time
	mu          sync.RWMutex
	devices     map[string]*Device
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	filePath := filepath.Join(dataDir, "devices.json")
	s := &Store{
		filePath: filePath,
		devices:  make(map[string]*Device),
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Populate with default template devices matching user's screenshots
		for _, d := range DefaultDevices() {
			s.devices[d.ID] = d
		}
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	} else {
		if err := s.load(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Store) DataDir() string {
	return filepath.Dir(s.filePath)
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.filePath)
	if err == nil {
		s.lastModTime = info.ModTime()
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var list []*Device
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	s.devices = make(map[string]*Device)
	for _, d := range list {
		if pass, err := GetPasswordFromKeychain(d.ID); err == nil && pass != "" {
			d.Password = pass
		}
		s.devices[d.ID] = d
	}
	return nil
}

func (s *Store) checkReload() {
	if info, err := os.Stat(s.filePath); err == nil {
		if info.ModTime().After(s.lastModTime) {
			_ = s.load()
		}
	}
}

func (s *Store) saveLocked() error {
	list := make([]*Device, 0, len(s.devices))
	for _, d := range s.devices {
		list = append(list, d)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(s.filePath, data, 0644)
	if err == nil {
		if info, err := os.Stat(s.filePath); err == nil {
			s.lastModTime = info.ModTime()
		}
	}
	return err
}

func (s *Store) GetAll() []*Device {
	s.checkReload()

	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Device, 0, len(s.devices))
	for _, d := range s.devices {
		copyDev := *d
		list = append(list, &copyDev)
	}
	return list
}

func (s *Store) Get(id string) (*Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.devices[id]
	if !ok {
		return nil, false
	}
	copyDev := *d
	return &copyDev, true
}

func (s *Store) Save(d *Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.ID == "" {
		d.ID = fmt.Sprintf("pc-%d", time.Now().UnixNano())
	}
	if d.CreatedAt == "" {
		d.CreatedAt = time.Now().Format(time.RFC3339)
	}
	if d.Port <= 0 {
		d.Port = 3389
	}
	if d.Width <= 0 {
		d.Width = 1280
	}
	if d.Height <= 0 {
		d.Height = 800
	}
	if d.Group == "" {
		d.Group = "Saved Devices"
	}

	if d.Password != "" {
		_ = SavePasswordToKeychain(d.ID, d.Password)
	}

	s.devices[d.ID] = d
	return s.saveLocked()
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = DeletePasswordFromKeychain(id)
	delete(s.devices, id)
	return s.saveLocked()
}

func (s *Store) ToggleFavorite(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.devices[id]
	if !ok {
		return false, fmt.Errorf("device not found")
	}
	d.Favorite = !d.Favorite
	return d.Favorite, s.saveLocked()
}

func (s *Store) UpdateStatus(id string, status string, latencyMs int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d, ok := s.devices[id]; ok {
		d.Status = status
		d.LatencyMs = latencyMs
	}
}

func (s *Store) UpdateLastConnected(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d, ok := s.devices[id]; ok {
		d.LastConnected = time.Now().Format("2006-01-02 15:04:05")
		_ = s.saveLocked()
	}
}
