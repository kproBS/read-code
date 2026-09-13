package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ProcessMetrics struct {
	RSSBytes  uint64  `json:"rssBytes"`
	CPUUsage  float64 `json:"cpuUsage"` // percentage e.g. 1.2%
	Goroutine int     `json:"goroutines"`
}

type metricsCollector struct {
	mu          sync.Mutex
	lastSample  time.Time
	lastCPUTime time.Duration
	lastUsage   float64
	numCPU      int
}

var globalMetrics = &metricsCollector{
	numCPU: runtime.NumCPU(),
}

func getProcessMetrics() ProcessMetrics {
	var m ProcessMetrics
	m.Goroutine = runtime.NumGoroutine()

	// 1. RSS Memory
	m.RSSBytes = readProcessRSS()

	// 2. CPU Usage
	m.CPUUsage = globalMetrics.sampleCPU()

	return m
}

func readProcessRSS() uint64 {
	// Try Linux /proc/self/statm
	if data, err := os.ReadFile("/proc/self/statm"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 2 {
			if pages, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				pageSize := uint64(os.Getpagesize())
				if pageSize == 0 {
					pageSize = 4096
				}
				return pages * pageSize
			}
		}
	}

	// Fallback to runtime.MemStats Sys/HeapAlloc if /proc not present (e.g. Darwin/Windows)
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.Sys
}

func (c *metricsCollector) sampleCPU() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cpuTime, err := readProcessCPUTime()
	if err != nil {
		return c.lastUsage
	}

	if c.lastSample.IsZero() {
		c.lastSample = now
		c.lastCPUTime = cpuTime
		return 0.0
	}

	wallDelta := now.Sub(c.lastSample)
	cpuDelta := cpuTime - c.lastCPUTime

	// Only recompute if at least 200ms has elapsed since last sample
	if wallDelta >= 200*time.Millisecond {
		usage := (float64(cpuDelta) / float64(wallDelta)) * 100.0
		if usage < 0 {
			usage = 0
		}
		c.lastUsage = usage
		c.lastSample = now
		c.lastCPUTime = cpuTime
	}

	return c.lastUsage
}

// readProcessCPUTime returns total CPU time consumed by the process (user + system)
func readProcessCPUTime() (time.Duration, error) {
	// Linux /proc/self/stat
	if data, err := os.ReadFile("/proc/self/stat"); err == nil {
		// The comm field is in parentheses and might contain spaces or parentheses: find last ')'
		idx := strings.LastIndex(string(data), ")")
		if idx != -1 && len(data) > idx+2 {
			fields := strings.Fields(string(data[idx+2:]))
			// after comm:
			// state is field 0
			// ppid is field 1
			// pgrp is field 2
			// session is field 3
			// tty_nr is field 4
			// tpgid is field 5
			// flags is field 6
			// minflt is field 7
			// cminflt is field 8
			// majflt is field 9
			// cmajflt is field 10
			// utime is field 11 (offset from field 0)
			// stime is field 12
			if len(fields) >= 13 {
				utime, err1 := strconv.ParseInt(fields[11], 10, 64)
				stime, err2 := strconv.ParseInt(fields[12], 10, 64)
				if err1 == nil && err2 == nil {
					// 100 clock ticks per second on Linux standard (CLK_TCK = 100)
					const clkTck = 100
					totalSec := float64(utime+stime) / float64(clkTck)
					return time.Duration(totalSec * float64(time.Second)), nil
				}
			}
		}
	}

	return 0, fmt.Errorf("cpu time unavailable")
}
