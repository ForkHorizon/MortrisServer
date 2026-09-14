package analytics

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// cleanProject removes all database rows for a project ID, ensuring integration
// tests run against clean, isolated state with no leaked data between runs.
func cleanProject(ctx context.Context, pool *pgxpool.Pool, projectID string) {
	CleanProjectData(ctx, pool, projectID)
}
