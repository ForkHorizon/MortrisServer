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

	counts, err := queryDailyCounts(ctx, pool, projectID, loc, windowStart, today, now)
	if err != nil {
		return nil, err
	}

	todayKey := today.Format("2006-01-02")
	stats := make([]EventAnomaly, 0, len(counts))
	for name, byDay := range counts {
		s, ok := eventStatFor(today, todayKey, byDay)
		if !ok {
			continue
		}
		s.Name = name
		stats = append(stats, s)
	}
	return stats, nil
}

// queryDailyCounts buckets each event name's raw event count into local
// calendar days (in loc) over [windowStart, today], inclusive of today.
// Today is naturally partial (it can't have future events), so every
// baseline day is truncated to the same wall-clock cutoff as `now` —
// otherwise a partial "today" reads as an anomalous dip against full
// baseline days it was never going to match.
func queryDailyCounts(ctx context.Context, pool *pgxpool.Pool, projectID string, loc *time.Location, windowStart, today, now time.Time) (map[string]map[string]int64, error) {
	rows, err := pool.Query(ctx, `
		SELECT name, (effective_at AT TIME ZONE $4)::date AS day, COUNT(*) AS cnt
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		  AND ((effective_at AT TIME ZONE $4)::date = $5::date
		       OR (effective_at AT TIME ZONE $4)::time < $6::time)
		GROUP BY name, day
	`, projectID, windowStart, today.AddDate(0, 0, 1), loc.String(), today, now.Format("15:04:05"))
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
	return counts, rows.Err()
}

// eventStatFor computes one event name's today-vs-baseline stats, or
// false if its trailing baseline volume is too low to be meaningful
// (anomalyMinBaselineVolume). The returned EventAnomaly's Name is unset —
// callers fill it in, since this only sees the per-day count map.
func eventStatFor(today time.Time, todayKey string, byDay map[string]int64) (EventAnomaly, bool) {
	baseline := make([]float64, 0, anomalyBaselineDays)
	var baselineTotal float64
	for i := anomalyBaselineDays; i >= 1; i-- {
		day := today.AddDate(0, 0, -i).Format("2006-01-02")
		v := float64(byDay[day])
		baseline = append(baseline, v)
		baselineTotal += v
	}
	if baselineTotal < anomalyMinBaselineVolume {
		return EventAnomaly{}, false
	}
	todayCount := byDay[todayKey]

	median := medianOf(baseline)
	mad := medianAbsDeviation(baseline, median)
	z := modifiedZScore(float64(todayCount), median, mad)

	s := EventAnomaly{TodayCount: todayCount, Median: median, ModifiedZScore: z}
	if median > 0 {
		pct := (float64(todayCount) - median) / median * 100
		s.PctChange = &pct
	}
	return s, true
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
