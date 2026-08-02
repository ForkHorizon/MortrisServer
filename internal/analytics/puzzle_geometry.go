package analytics

import (
	"context"
	"encoding/json"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Block geometry is deliberately NOT part of the catalogue document.
//
// content_revision is the SHA-256 of that document, and events are always
// interpreted against their own revision. Folding shapes into the
// catalogue would therefore mint a new revision, and every event already
// collected — every attempt the current testers have played — would keep
// pointing at the old shapeless one and could never be drawn.
//
// Shapes are not semantics. Where a block sits, which wave it belongs to,
// and what must be placed before it are meaning, and stay immutable. How
// that same block was drawn is a description of it, and can be attached
// to a revision afterwards without changing what the revision asserts.
//
// So geometry is an additive attachment: it may only add shape to blocks
// the revision already declares, and can never introduce a block, move
// one, or alter a rule. Applying it twice is safe, and re-importing the
// catalogue does not erase it.

// PuzzleGeometry is one upload, describing blocks already present in
// ContentRevision.
type PuzzleGeometry struct {
	SchemaVersion   int                   `json:"schema_version"`
	ContentRevision string                `json:"content_revision"`
	Blocks          []PuzzleBlockGeometry `json:"blocks"`
}

type PuzzleBlockGeometry struct {
	CityID  int               `json:"city_id"`
	HouseID int               `json:"house_id"`
	BlockID int               `json:"block_id"`
	Bounds  PuzzleBlockBounds `json:"bounds_milli"`
	Outline []PuzzlePoint     `json:"outline_milli,omitempty"`
}

// PuzzleBlockBounds is the block's axis-aligned extent, in the same
// quantized milli-unit space as a block's local_x_milli/local_y_milli.
type PuzzleBlockBounds struct {
	MinX int `json:"min_x"`
	MinY int `json:"min_y"`
	MaxX int `json:"max_x"`
	MaxY int `json:"max_y"`
}

// PuzzlePoint is [x, y] — an array rather than an object because a house
// carries thousands of these and the object form roughly triples the
// payload for no added clarity.
type PuzzlePoint [2]int

// maxOutlinePoints bounds one block's silhouette. The exporter simplifies
// to about 32 points; this limit is deliberately looser so a slightly more
// detailed piece is not rejected outright, while still refusing an
// untraced path with thousands of vertices.
const maxOutlinePoints = 64

func (g PuzzleGeometry) Validate() error {
	if g.SchemaVersion != 1 {
		return apierr.New(400, contracts.CodeInvalidRequest, "schema_version must be 1")
	}
	if !revisionPattern.MatchString(g.ContentRevision) {
		return apierr.New(400, contracts.CodeInvalidRequest, "content_revision must be a lowercase SHA-256 hex string")
	}
	if len(g.Blocks) == 0 {
		return invalidGeometry("must describe at least one block")
	}
	seen := make(map[[3]int]bool, len(g.Blocks))
	for _, block := range g.Blocks {
		key := [3]int{block.CityID, block.HouseID, block.BlockID}
		if seen[key] {
			return invalidGeometry("duplicate block in upload")
		}
		seen[key] = true
		if err := block.validate(); err != nil {
			return err
		}
	}
	return nil
}

// validate refuses geometry that cannot be drawn: inverted or degenerate
// bounds, a sub-triangular outline, or points outside the bounds the
// renderer derives its viewBox from.
func (b PuzzleBlockGeometry) validate() error {
	if b.Bounds.MinX >= b.Bounds.MaxX || b.Bounds.MinY >= b.Bounds.MaxY {
		return invalidGeometry("bounds_milli must have min strictly below max")
	}
	if len(b.Outline) == 0 {
		return nil
	}
	if len(b.Outline) < 3 {
		return invalidGeometry("outline_milli needs at least 3 points")
	}
	if len(b.Outline) > maxOutlinePoints {
		return invalidGeometry("outline_milli exceeds the point limit")
	}
	for _, point := range b.Outline {
		if point[0] < b.Bounds.MinX || point[0] > b.Bounds.MaxX ||
			point[1] < b.Bounds.MinY || point[1] > b.Bounds.MaxY {
			return invalidGeometry("outline_milli point falls outside bounds_milli")
		}
	}
	return nil
}

// ApplyPuzzleGeometry attaches shapes to an existing revision. It reports
// how many blocks it updated. A block the revision does not declare is a
// mismatched pairing — geometry from one export against a different
// revision — and fails the whole upload rather than silently landing
// partial shapes that would draw a broken house.
func ApplyPuzzleGeometry(ctx context.Context, pool *pgxpool.Pool, projectID string, geometry PuzzleGeometry) (int, error) {
	if err := geometry.Validate(); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	var revisionExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM puzzle_content_revisions WHERE project_id=$1 AND content_revision=$2)`, projectID, geometry.ContentRevision).Scan(&revisionExists); err != nil {
		return 0, err
	}
	if !revisionExists {
		return 0, apierr.New(404, contracts.CodeInvalidRequest, "unknown content_revision — import the catalogue first")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	updated := 0
	for _, block := range geometry.Blocks {
		command, err := tx.Exec(ctx, `UPDATE puzzle_content_blocks SET bounds_min_x_milli=$1,bounds_min_y_milli=$2,bounds_max_x_milli=$3,bounds_max_y_milli=$4,outline_milli=$5::jsonb WHERE project_id=$6 AND content_revision=$7 AND city_id=$8 AND house_id=$9 AND block_id=$10`,
			block.Bounds.MinX, block.Bounds.MinY, block.Bounds.MaxX, block.Bounds.MaxY, outlineJSON(block.Outline),
			projectID, geometry.ContentRevision, block.CityID, block.HouseID, block.BlockID)
		if err != nil {
			return 0, err
		}
		if command.RowsAffected() == 0 {
			return 0, invalidGeometry("describes a block this revision does not declare")
		}
		updated++
	}
	return updated, tx.Commit(ctx)
}

func outlineJSON(outline []PuzzlePoint) *string {
	if len(outline) == 0 {
		return nil
	}
	encoded, err := json.Marshal(outline)
	if err != nil {
		return nil
	}
	value := string(encoded)
	return &value
}

func invalidGeometry(message string) error {
	return apierr.New(400, contracts.CodeInvalidRequest, "invalid puzzle geometry: "+message)
}
