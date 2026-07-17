package maintenance

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ForkHorizon/Mortris/internal/diskstate"
)

// DiskMonitor periodically evaluates disk-pressure state (section 12) and
// caches it, so ingest.Service.Batch can check it on every request without
// a syscall per request.
type DiskMonitor struct {
	path     string
	interval time.Duration
	log      *slog.Logger
	current  atomic.Value // diskstate.State
}

func NewDiskMonitor(path string, interval time.Duration, log *slog.Logger) *DiskMonitor {
	m := &DiskMonitor{path: path, interval: interval, log: log}
	m.current.Store(diskstate.Normal)
	return m
}

// Get satisfies ingest.DiskStateSource.
func (m *DiskMonitor) Get() diskstate.State {
	return m.current.Load().(diskstate.State)
}

func (m *DiskMonitor) Run(ctx context.Context) {
	m.check()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.check()
		}
	}
}

func (m *DiskMonitor) check() {
	state, reading, err := diskstate.Evaluate(m.path)
	if err != nil {
		m.log.Error("disk state check failed", "path", m.path, "error", err)
		return
	}
	previous := m.Get()
	m.current.Store(state)
	if state != previous {
		m.log.Warn("disk state changed", "from", previous, "to", state,
			"percent_used", reading.PercentUsed, "free_bytes", reading.FreeBytes)
	}
}
