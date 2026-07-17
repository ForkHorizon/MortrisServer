// Package diskstate evaluates the disk-pressure state from section 12.
// Uses syscall.Statfs directly (available on both Linux, the production
// target, and Darwin, for local dev) instead of pulling in a dependency
// for a five-field struct.
package diskstate

import "syscall"

type State string

const (
	Normal    State = "normal"
	Warning   State = "warning"
	High      State = "high"
	Critical  State = "critical"
	Rejecting State = "rejecting"
)

type Reading struct {
	PercentUsed float64
	FreeBytes   uint64
	TotalBytes  uint64
}

const gib = 1 << 30

// Evaluate reads free space at path and classifies it per section 12's
// table. Where percentage-used and absolute-free-bytes disagree, the more
// conservative (worse) state wins — each case below checks both
// conditions with OR, most severe first, so the first match is the
// binding one.
func Evaluate(path string) (State, Reading, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return "", Reading{}, err
	}

	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	free := stat.Bavail * blockSize // available to non-root, i.e. actually usable

	var percentUsed float64
	if total > 0 {
		used := total - stat.Bfree*blockSize
		percentUsed = float64(used) / float64(total) * 100
	}

	reading := Reading{PercentUsed: percentUsed, FreeBytes: free, TotalBytes: total}
	return classify(percentUsed, free), reading, nil
}

func classify(percentUsed float64, freeBytes uint64) State {
	switch {
	case percentUsed >= 90 || freeBytes <= 5*gib:
		return Rejecting
	case percentUsed >= 85 || freeBytes <= uint64(7.5*gib):
		return Critical
	case percentUsed >= 80 || freeBytes <= 10*gib:
		return High
	case percentUsed >= 70 || freeBytes <= 15*gib:
		return Warning
	default:
		return Normal
	}
}
