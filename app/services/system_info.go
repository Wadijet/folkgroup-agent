/*
Package services chứa các services hỗ trợ cho agent.
File này thu thập system info (OS, arch, Go version, CPU, Memory, Disk).
*/
package services

import (
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemInfoCollector thu thập thông tin hệ thống
type SystemInfoCollector struct {
	startTime              time.Time
	systemMetricsCache     *SystemMetricsCache
	systemMetricsCacheMu   sync.RWMutex
	systemMetricsCacheInterval time.Duration
}

// SystemMetricsCache cache cho system metrics (CPU, Memory, Disk)
type SystemMetricsCache struct {
	CPUUsage    float64
	MemoryUsage float64
	DiskUsage   float64
	UpdatedAt   time.Time
}

// NewSystemInfoCollector tạo một instance mới của SystemInfoCollector
func NewSystemInfoCollector() *SystemInfoCollector {
	return &SystemInfoCollector{
		startTime:                time.Now(),
		systemMetricsCache:        &SystemMetricsCache{},
		systemMetricsCacheInterval: 5 * time.Minute, // Cache 5 phút
	}
}

// SystemInfo chứa thông tin hệ thống
type SystemInfo struct {
	// Static info (chỉ thu thập 1 lần khi khởi động)
	OS        string `json:"os"`        // "linux", "windows", "darwin"
	Arch      string `json:"arch"`      // "amd64", "arm64", etc.
	GoVersion string `json:"goVersion"` // "go1.21.0"

	// Dynamic info (cache 5-10 phút)
	Uptime      int64   `json:"uptime"`      // Uptime (giây) - tính từ start time, nhẹ
	MemoryUsage float64 `json:"memoryUsage"` // Memory usage (%)
	CPUUsage    float64 `json:"cpuUsage"`    // CPU usage (%)
	DiskUsage   float64 `json:"diskUsage"`    // Disk usage (%)
}

// Collect thu thập system info (tối ưu với cache)
func (s *SystemInfoCollector) Collect() SystemInfo {
	info := SystemInfo{
		// Static info (từ cache - chỉ thu thập 1 lần)
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
	}

	// Uptime (tính từ start time, nhẹ)
	info.Uptime = int64(time.Since(s.startTime).Seconds())

	// Dynamic info (từ cache, chỉ update mỗi 5-10 phút)
	s.systemMetricsCacheMu.RLock()
	cache := s.systemMetricsCache
	s.systemMetricsCacheMu.RUnlock()

	// Kiểm tra xem cache có cũ không
	now := time.Now()
	if now.Sub(cache.UpdatedAt) > s.systemMetricsCacheInterval {
		// Cache cũ → Update cache
		s.updateSystemMetricsCache()
		s.systemMetricsCacheMu.RLock()
		cache = s.systemMetricsCache
		s.systemMetricsCacheMu.RUnlock()
	}

	info.CPUUsage = cache.CPUUsage
	info.MemoryUsage = cache.MemoryUsage
	info.DiskUsage = cache.DiskUsage

	return info
}

// updateSystemMetricsCache cập nhật cache cho system metrics
func (s *SystemInfoCollector) updateSystemMetricsCache() {
	s.systemMetricsCacheMu.Lock()
	defer s.systemMetricsCacheMu.Unlock()

	// Double-check: có thể đã được update bởi goroutine khác
	now := time.Now()
	if now.Sub(s.systemMetricsCache.UpdatedAt) < s.systemMetricsCacheInterval {
		return
	}

	// Thu thập CPU usage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		s.systemMetricsCache.CPUUsage = cpuPercent[0]
	}

	// Thu thập Memory usage
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		s.systemMetricsCache.MemoryUsage = memInfo.UsedPercent
	}

	// Thu thập Disk usage (root partition)
	diskInfo, err := disk.Usage("/")
	if err == nil {
		s.systemMetricsCache.DiskUsage = diskInfo.UsedPercent
	}

	s.systemMetricsCache.UpdatedAt = now
}
