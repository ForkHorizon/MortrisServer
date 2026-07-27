package analytics

import (
	"context"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

type PropertyValuesFilter struct {
	Name        string
	PropertyKey string
}

// ParsePropertyValuesFilter validates the Property Value Inspector's
// (plan item 2b) required name+property_key pair against the project's
// event catalog — same allowlist as the Event Explorer's optional
// property filter, just mandatory here since there's nothing to inspect
// without both.
func ParsePropertyValuesFilter(ctx context.Context, pool *pgxpool.Pool, projectID string, q url.Values) (PropertyValuesFilter, error) {
	name := q.Get("name")
	key := q.Get("property_key")
	if name == "" || key == "" {
		return PropertyValuesFilter{}, apierr.New(400, contracts.CodeInvalidRequest, "name and property_key are required")
	}
	if err := validateEventName(ctx, pool, projectID, name); err != nil {
		return PropertyValuesFilter{}, err
	}
	if err := validateCatalogProperty(ctx, pool, projectID, name, key); err != nil {
		return PropertyValuesFilter{}, err
	}
	return PropertyValuesFilter{Name: name, PropertyKey: key}, nil
}

type PropertyValueCount struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

type PropertyValuesResult struct {
	Values []PropertyValueCount `json:"values"`
}

// GetPropertyValues implements the Property Value Inspector (2b): the top
// 20 values (by count) a cataloged property took on for a given event in
// the date range — good for spotting bad enum values coming from the
// client.
func GetPropertyValues(ctx context.Context, pool *pgxpool.Pool, projectID string, f PropertyValuesFilter, from, to time.Time) (*PropertyValuesResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT properties ->> $5 AS value, COUNT(*)
		FROM events
		WHERE project_id = $1 AND name = $2 AND effective_at >= $3 AND effective_at < $4
		GROUP BY 1
		ORDER BY 2 DESC
		LIMIT 20
	`, projectID, f.Name, from, to, f.PropertyKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Values starts as []PropertyValueCount{}, not nil — see catalog.go's
	// Entries for why a nil slice here would crash a naive frontend list.
	result := &PropertyValuesResult{Values: []PropertyValueCount{}}
	for rows.Next() {
		var value *string
		var count int64
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		v := "(missing)"
		if value != nil {
			v = *value
		}
		result.Values = append(result.Values, PropertyValueCount{Value: v, Count: count})
	}
	return result, rows.Err()
}
