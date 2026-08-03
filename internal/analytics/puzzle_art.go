package analytics

import (
	"context"
	"errors"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// House art is the assembled picture of a house, drawn under the block
// diagram so the view looks like the game rather than a schematic.
//
// Like geometry, it attaches to an existing revision and never changes
// what that revision asserts — see puzzle_geometry.go for the reasoning.
// It carries no placement data: the art is cropped to its opaque pixels,
// whose extent is exactly the union of the revision's block bounds, so
// the renderer positions it with the rect it already computes.

// maxHouseArtBytes bounds a single image. Measured, the 103-house set
// averages about 40 KB per house as cropped WebP; this leaves room for a
// larger or less compressible house while still refusing an accidental
// multi-megabyte upload of an uncropped source PNG.
const maxHouseArtBytes = 2 * 1024 * 1024

type PuzzleHouseArt struct {
	CityID    int    `json:"city_id"`
	HouseID   int    `json:"house_id"`
	MediaType string `json:"media_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Image     []byte `json:"-"`
}

// allowedArtMediaTypes is a strict allowlist. These bytes are served back
// to a browser, so the response Content-Type must never be attacker- or
// accident-chosen; anything not listed here is refused at import.
var allowedArtMediaTypes = map[string]bool{
	"image/webp": true,
	"image/png":  true,
}

// ImportPuzzleHouseArt replaces the art for the given revision's houses.
// Re-running it is safe and is how a re-render is published.
func ImportPuzzleHouseArt(ctx context.Context, pool *pgxpool.Pool, projectID, revision string, houses []PuzzleHouseArt) (int, error) {
	if !revisionPattern.MatchString(revision) {
		return 0, apierr.New(400, contracts.CodeInvalidRequest, "content_revision must be a lowercase SHA-256 hex string")
	}
	if len(houses) == 0 {
		return 0, apierr.New(400, contracts.CodeInvalidRequest, "no house art supplied")
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM puzzle_content_revisions WHERE project_id=$1 AND content_revision=$2)`, projectID, revision).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, apierr.New(404, contracts.CodeInvalidRequest, "unknown content_revision — import the catalogue first")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, house := range houses {
		if err := validateHouseArt(house); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO puzzle_house_art (project_id,content_revision,city_id,house_id,media_type,width,height,image)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (project_id,content_revision,city_id,house_id) DO UPDATE SET
    media_type=EXCLUDED.media_type, width=EXCLUDED.width, height=EXCLUDED.height,
    image=EXCLUDED.image, imported_at=clock_timestamp()`,
			projectID, revision, house.CityID, house.HouseID, house.MediaType, house.Width, house.Height, house.Image); err != nil {
			return 0, err
		}
	}
	return len(houses), tx.Commit(ctx)
}

func validateHouseArt(house PuzzleHouseArt) error {
	if !allowedArtMediaTypes[house.MediaType] {
		return apierr.New(400, contracts.CodeInvalidRequest, "unsupported house art media type: "+house.MediaType)
	}
	if len(house.Image) == 0 {
		return apierr.New(400, contracts.CodeInvalidRequest, "house art image is empty")
	}
	if len(house.Image) > maxHouseArtBytes {
		return apierr.New(400, contracts.CodeInvalidRequest, "house art image is too large")
	}
	if house.Width <= 0 || house.Height <= 0 {
		return apierr.New(400, contracts.CodeInvalidRequest, "house art dimensions must be positive")
	}
	return nil
}

// GetPuzzleHouseArt returns the stored image, or nil when the house has
// none — a house without art still renders as a diagram, so a miss is an
// ordinary outcome rather than an error.
func GetPuzzleHouseArt(ctx context.Context, pool *pgxpool.Pool, projectID, revision string, cityID, houseID int) (*PuzzleHouseArt, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	art := &PuzzleHouseArt{CityID: cityID, HouseID: houseID}
	err := pool.QueryRow(ctx, `SELECT media_type, width, height, image FROM puzzle_house_art
WHERE project_id=$1 AND content_revision=$2 AND city_id=$3 AND house_id=$4`,
		projectID, revision, cityID, houseID).Scan(&art.MediaType, &art.Width, &art.Height, &art.Image)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return art, nil
}
