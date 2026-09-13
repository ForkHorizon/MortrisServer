package analytics

// MemoryCohort represents an aggregated hardware memory tier.
// Metrics strictly describe correlational co-occurrence, not causation.
type MemoryCohort struct {
	Tier                  string  `json:"tier"`
	Label                 string  `json:"label"`
	MinMemoryMB           int64   `json:"min_memory_mb"`
	MaxMemoryMB           int64   `json:"max_memory_mb"`
	DeviceCount           int64   `json:"device_count"`
	NaturalRunsCount      int64   `json:"natural_runs_count"`
	CompletedRunsCount    int64   `json:"completed_runs_count"`
	CompletionRate        float64 `json:"completion_rate"`
	PlacementsCount       int64   `json:"placements_count"`
	FallsCount            int64   `json:"falls_count"`
	FallRate              float64 `json:"fall_rate"`
	SampleCountSufficient bool    `json:"sample_count_sufficient"`
	EvidenceNote          string  `json:"evidence_note"`
}

// memoryTierForMB classifies a memory amount in MB into a neutral tier.
func memoryTierForMB(ramMB int64) string {
	if ramMB <= 0 {
		return "unknown"
	}
	if ramMB < 4096 {
		return "low"
	}
	if ramMB <= 6144 {
		return "medium"
	}
	return "high"
}

// newCohortTemplate creates an initialized cohort struct for a tier.
func newCohortTemplate(tier, label string, minMB, maxMB int64) MemoryCohort {
	return MemoryCohort{
		Tier:        tier,
		Label:       label,
		MinMemoryMB: minMB,
		MaxMemoryMB: maxMB,
	}
}

// initializeCohorts prepares the standard cohort buckets.
func initializeCohorts() map[string]*MemoryCohort {
	cohorts := map[string]*MemoryCohort{
		"low": {
			Tier:        "low",
			Label:       "Low (< 4 GB)",
			MinMemoryMB: 1,
			MaxMemoryMB: 4095,
		},
		"medium": {
			Tier:        "medium",
			Label:       "Medium (4–6 GB)",
			MinMemoryMB: 4096,
			MaxMemoryMB: 6144,
		},
		"high": {
			Tier:        "high",
			Label:       "High (>= 8 GB)",
			MinMemoryMB: 6145,
			MaxMemoryMB: 0,
		},
	}
	return cohorts
}

// finalizeCohortMetrics computes rates and neutral evidence notes for a cohort.
func finalizeCohortMetrics(c *MemoryCohort) {
	if c.NaturalRunsCount > 0 {
		c.CompletionRate = float64(c.CompletedRunsCount) / float64(c.NaturalRunsCount)
	}
	if c.PlacementsCount > 0 {
		c.FallRate = float64(c.FallsCount) / float64(c.PlacementsCount)
	}
	c.SampleCountSufficient = c.NaturalRunsCount >= 5 && c.DeviceCount >= 1
	if c.SampleCountSufficient {
		c.EvidenceNote = "Correlational distribution across memory tier"
	} else {
		c.EvidenceNote = "Insufficient sample size (<5 natural runs) for reliable rates"
	}
}

// BuildMemoryCohorts groups devices into standard memory tiers and aggregates
// natural run and fall metrics with visible sample counts.
func BuildMemoryCohorts(devices []PuzzleDeviceSummary) []MemoryCohort {
	bucketMap := initializeCohorts()

	for _, d := range devices {
		tier := memoryTierForMB(d.DeviceTotalMemoryMB)
		cohort, exists := bucketMap[tier]
		if !exists {
			continue
		}
		cohort.DeviceCount++
		cohort.NaturalRunsCount += d.NaturalRunsCount
		cohort.CompletedRunsCount += d.CompletedRunsCount
		cohort.PlacementsCount += d.PlacementsCount
		cohort.FallsCount += d.FallsCount
	}

	order := []string{"low", "medium", "high"}
	result := make([]MemoryCohort, 0, len(order))
	for _, tier := range order {
		c := bucketMap[tier]
		finalizeCohortMetrics(c)
		result = append(result, *c)
	}
	return result
}
