package analytics

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	anomalyBaselineDays = 14
	// anomalyZThreshold is Iglewicz & Hoaglin's standard modified
	// z-score outlier cutoff — robust to a skewed or bursty baseline in
	// a way a plain mean/stddev z-score isn't.
	anomalyZThreshold = 3.5
	// anomalyMinBaselineVolume: below ~1 event/day on average, "today's
	// count differs from the median" is just sampling noise, not a
	// signal. ponytail: fixed threshold, revisit if low-volume events
	// need their own bar.
	anomalyMinBaselineVolume = anomalyBaselineDays
)

type EventAnomaly struct {
	Name           string   `json:"name"`
	TodayCount     int64    `json:"today_count"`
	Median         float64  `json:"median"`
	PctChange      *float64 `json:"pct_change,omitempty"`
	ModifiedZScore float64  `json:"modified_z_score"`
}

type AnomaliesResult struct {
	Anomalies []EventAnomaly `json:"anomalies"`
}

// GetAnomalies compares each event name's count for "today" (in the
// project's display timezone) against its trailing 14-day median,
// flagging it when the modified z-score exceeds anomalyZThreshold. Pure
// SQL aggregation plus a stdlib stats pass — no ML.
func GetAnomalies(ctx context.Context, pool *pgxpool.Pool, projectID string, loc *time.Location) (*AnomaliesResult, error) {
	stats, err := computeEventStats(ctx, pool, projectID, loc)
	if err != nil {
		return nil, err
	}

	result := &AnomaliesResult{Anomalies: []EventAnomaly{}}
	for _, s := range stats {
		if math.Abs(s.ModifiedZScore) > anomalyZThreshold {
			result.Anomalies = append(result.Anomalies, s)
		}
	}
	sort.Slice(result.Anomalies, func(i, j int) bool {
		return math.Abs(result.Anomalies[i].ModifiedZScore) > math.Abs(result.Anomalies[j].ModifiedZScore)
	})
	return result, nil
}

// computeEventStats returns today-vs-trailing-14-day-median stats for
// every event name with enough baseline volume to be meaningful,
// regardless of whether they cross the anomaly threshold — shared by
// GetAnomalies (filtered to the outliers) and GetDigest (the broader
// "top movers" context sent to Claude).
func computeEventStats(ctx context.Context, pool *pgxpool.Pool, projectID string, loc *time.Location) ([]EventAnomaly, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	windowStart := today.AddDate(0, 0, -anomalyBaselineDays)

	rows, err := pool.Query(ctx, `
		SELECT name, (effective_at AT TIME ZONE $4)::date AS day, COUNT(*) AS cnt
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		GROUP BY name, day
	`, projectID, windowStart, today.AddDate(0, 0, 1), loc.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]map[string]int64{}
	for rows.Next() {
		var name string
		var day time.Time
		var cnt int64
		if err := rows.Scan(&name, &day, &cnt); err != nil {
			return nil, err
		}
		if counts[name] == nil {
			counts[name] = map[string]int64{}
		}
		counts[name][day.Format("2006-01-02")] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	todayKey := today.Format("2006-01-02")
	stats := make([]EventAnomaly, 0, len(counts))
	for name, byDay := range counts {
		baseline := make([]float64, 0, anomalyBaselineDays)
		var baselineTotal float64
		for i := anomalyBaselineDays; i >= 1; i-- {
			day := today.AddDate(0, 0, -i).Format("2006-01-02")
			v := float64(byDay[day])
			baseline = append(baseline, v)
			baselineTotal += v
		}
		if baselineTotal < anomalyMinBaselineVolume {
			continue
		}
		todayCount := byDay[todayKey]

		median := medianOf(baseline)
		mad := medianAbsDeviation(baseline, median)
		z := modifiedZScore(float64(todayCount), median, mad)

		s := EventAnomaly{Name: name, TodayCount: todayCount, Median: median, ModifiedZScore: z}
		if median > 0 {
			pct := (float64(todayCount) - median) / median * 100
			s.PctChange = &pct
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func medianOf(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func medianAbsDeviation(values []float64, median float64) float64 {
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - median)
	}
	return medianOf(deviations)
}

// modifiedZScore is Iglewicz & Hoaglin's robust z-score. When the
// baseline is perfectly flat (mad == 0), any deviation at all is
// unusual — clamp rather than divide by zero, so e.g. a steady-2-a-day
// event dropping to 0 is still flagged.
func modifiedZScore(value, median, mad float64) float64 {
	if mad == 0 {
		switch {
		case value == median:
			return 0
		case value > median:
			return anomalyZThreshold + 1
		default:
			return -(anomalyZThreshold + 1)
		}
	}
	return 0.6745 * (value - median) / mad
}
