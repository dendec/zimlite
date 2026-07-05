package storage

import (
	"context"
	"encoding/json"
	"os"
	"sync"
)

type DownloadInfo struct {
	URL       string `json:"url"`
	TotalSize int64  `json:"total_size"`
}

type DownloadManager struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

var Manager = &DownloadManager{
	cancels: make(map[string]context.CancelFunc),
}

func (m *DownloadManager) Add(filename string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancels[filename] = cancel
}

func (m *DownloadManager) Remove(filename string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cancels, filename)
}

func (m *DownloadManager) Stop(filename string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.cancels[filename]; ok {
		cancel()
		delete(m.cancels, filename)
	}
}

func (m *DownloadManager) IsActive(filename string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.cancels[filename]
	return ok
}

func SaveDownloadInfo(path string, info DownloadInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadDownloadInfo(path string) (DownloadInfo, error) {
	var info DownloadInfo
	data, err := os.ReadFile(path)
	if err != nil {
		return info, err
	}
	err = json.Unmarshal(data, &info)
	return info, err
}
