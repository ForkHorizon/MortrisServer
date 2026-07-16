package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RetentionCohort struct {
	CohortDay  string `json:"cohort_day"`
	CohortSize int64  `json:"cohort_size"`
	D1         int64  `json:"d1"`
	D7         int64  `json:"d7"`
	D30        int64  `json:"d30"`
}

type RetentionResult struct {
	Cohorts []RetentionCohort `json:"cohorts"`
}

// GetRetention implements section 10.2 #4 / section 9's D1/D7/D30
// definition: cohort by the calendar date of first_product_event_at in
// loc, retained at D*N* if the same install_id has a product event on
// cohort_day+N in loc. A reinstall starts a new cohort member rather
// than extending the old one (section 9, non-goal) — this falls out
// naturally since it's keyed by install_id, not any cross-install identity.
func GetRetention(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location) (*RetentionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	tz := loc.String()
	rows, err := pool.Query(ctx, `
		WITH cohorts AS (
			SELECT install_id, (first_product_event_at AT TIME ZONE $4)::date AS cohort_day
			FROM installations
			WHERE project_id = $1 AND first_product_event_at >= $2 AND first_product_event_at < $3
		), retained AS (
			SELECT DISTINCT install_id, (effective_at AT TIME ZONE $4)::date AS active_day
			FROM events
			WHERE project_id = $1 AND event_kind = 'product'
			  AND effective_at >= $2 AND effective_at < $3 + interval '31 days'
		)
		SELECT
			c.cohort_day,
			COUNT(DISTINCT c.install_id) AS cohort_size,
			COUNT(DISTINCT r1.install_id) AS d1,
			COUNT(DISTINCT r7.install_id) AS d7,
			COUNT(DISTINCT r30.install_id) AS d30
		FROM cohorts c
		LEFT JOIN retained r1 ON r1.install_id = c.install_id AND r1.active_day = c.cohort_day + 1
		LEFT JOIN retained r7 ON r7.install_id = c.install_id AND r7.active_day = c.cohort_day + 7
		LEFT JOIN retained r30 ON r30.install_id = c.install_id AND r30.active_day = c.cohort_day + 30
		GROUP BY c.cohort_day
		ORDER BY c.cohort_day
	`, projectID, from, to, tz)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Cohorts starts as []RetentionCohort{}, not nil — encoding/json
	// emits null for a nil slice, which crashes a naive frontend list
	// render on the (common) empty-range case.
	result := RetentionResult{Cohorts: []RetentionCohort{}}
	for rows.Next() {
		var c RetentionCohort
		var day time.Time
		if err := rows.Scan(&day, &c.CohortSize, &c.D1, &c.D7, &c.D30); err != nil {
			return nil, err
		}
		c.CohortDay = day.Format("2006-01-02")
		result.Cohorts = append(result.Cohorts, c)
	}
	return &result, rows.Err()
}
