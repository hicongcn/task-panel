package service

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// SysStats 系统监控快照。
type SysStats struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemTotal    uint64  `json:"mem_total"`
	MemUsed     uint64  `json:"mem_used"`
	MemPercent  float64 `json:"mem_percent"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskPercent float64 `json:"disk_percent"`
	Load1       float64 `json:"load1"`
	Load5       float64 `json:"load5"`
	Load15      float64 `json:"load15"`
	Uptime      uint64  `json:"uptime_seconds"`
	Hostname    string  `json:"hostname"`
	Platform    string  `json:"platform"`
}

// SysMonitor 后台定时采样系统指标,接口零阻塞读取缓存。
type SysMonitor struct {
	mu    sync.RWMutex
	stats SysStats
}

var defaultSysMonitor = &SysMonitor{}

func GetSysMonitor() *SysMonitor { return defaultSysMonitor }

// Start 启动后台采样(立即采一次,之后每 2 秒刷新)。
func (m *SysMonitor) Start() {
	m.refresh()
	go func() {
		for {
			time.Sleep(2 * time.Second)
			m.refresh()
		}
	}()
}

func (m *SysMonitor) refresh() {
	s := SysStats{}
	if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
		s.CPUPercent = p[0]
	}
	if v, err := mem.VirtualMemory(); err == nil {
		s.MemTotal, s.MemUsed, s.MemPercent = v.Total, v.Used, v.UsedPercent
	}
	if d, err := disk.Usage("/"); err == nil {
		s.DiskTotal, s.DiskUsed, s.DiskPercent = d.Total, d.Used, d.UsedPercent
	}
	if a, err := load.Avg(); err == nil {
		s.Load1, s.Load5, s.Load15 = a.Load1, a.Load5, a.Load15
	}
	if h, err := host.Info(); err == nil {
		s.Uptime, s.Hostname, s.Platform = h.Uptime, h.Hostname, h.Platform
	}
	m.mu.Lock()
	m.stats = s
	m.mu.Unlock()
}

// Stats 返回最近一次采样快照。
func (m *SysMonitor) Stats() SysStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}
