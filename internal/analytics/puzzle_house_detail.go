package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PuzzleHouseBlock is one detail of a house: its shape, where it belongs,
// what must be placed before it, and how players actually fared with it.
// Geometry and metrics arrive together because the view needs both to
// draw a single block and splitting them would double the round trips
// for no gain.
type PuzzleHouseBlock struct {
	BlockID      int                `json:"block_id"`
	WaveIndex    int                `json:"wave_index"`
	OrderInLayer int                `json:"order_in_layer"`
	IsGround     bool               `json:"is_ground"`
	VisualKey    string             `json:"visual_key"`
	Bounds       *PuzzleBlockBounds `json:"bounds_milli,omitempty"`
	Outline      []PuzzlePoint      `json:"outline_milli,omitempty"`
	// RequiredGroups is OR of AND groups: the block may be placed once any
	// one group is fully placed. Empty for a ground block.
	RequiredGroups [][]int `json:"required_groups"`
	Placements     int64   `json:"placements"`
	Falls          int64   `json:"falls"`
	FallRate       float64 `json:"fall_rate"`
	// FallsByReason keys are the raw outcome values; the dashboard maps
	// them to plain language.
	FallsByReason map[string]int64 `json:"falls_by_reason"`
	// RateIsReliable is false below minPlacementsForRate — see its comment.
	RateIsReliable bool `json:"rate_is_reliable"`
}

type PuzzleHouseDetail struct {
	CityID          int                `json:"city_id"`
	HouseID         int                `json:"house_id"`
	DisplayLabel    string             `json:"display_label"`
	ContentRevision string             `json:"content_revision"`
	WaveCount       int                `json:"wave_count"`
	Blocks          []PuzzleHouseBlock `json:"blocks"`
}

func GetPuzzleHouse(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time) (*PuzzleHouseDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	revision, err := latestPuzzleRevision(ctx, pool, projectID)
	if err != nil {
		return nil, err
	}
	detail := &PuzzleHouseDetail{CityID: cityID, HouseID: houseID, ContentRevision: revision, Blocks: []PuzzleHouseBlock{}}
	if err := loadPuzzleHouseBlocks(ctx, pool, detail, projectID, revision); err != nil {
		return nil, err
	}
	if err := loadPuzzleHouseMetrics(ctx, pool, detail, projectID, from, to); err != nil {
		return nil, err
	}
	// The jsonpath is built here rather than concatenated in SQL: the
	// values are already integers, so there is nothing to inject, and
	// pgx cannot encode an int parameter into a text-typed slot.
	path := fmt.Sprintf("$.cities[*] ? (@.city_id == %d).houses[*] ? (@.house_id == %d).display_label", cityID, houseID)
	return detail, pool.QueryRow(ctx, `SELECT COALESCE(jsonb_path_query_first(catalog, $2::jsonpath) #>> '{}', '') FROM puzzle_content_revisions WHERE project_id=$1 AND content_revision=$3`,
		projectID, path, revision).Scan(&detail.DisplayLabel)
}

func loadPuzzleHouseBlocks(ctx context.Context, pool *pgxpool.Pool, detail *PuzzleHouseDetail, projectID, revision string) error {
	rows, err := pool.Query(ctx, `
SELECT b.block_id, b.wave_index, b.order_in_layer, b.is_ground, b.visual_key,
       b.bounds_min_x_milli, b.bounds_min_y_milli, b.bounds_max_x_milli, b.bounds_max_y_milli,
       b.outline_milli, r.alternative_groups
FROM puzzle_content_blocks b
LEFT JOIN puzzle_content_rules r
       ON r.project_id=b.project_id AND r.content_revision=b.content_revision
      AND r.city_id=b.city_id AND r.house_id=b.house_id AND r.target_id=b.block_id
WHERE b.project_id=$1 AND b.content_revision=$2 AND b.city_id=$3 AND b.house_id=$4
ORDER BY b.order_in_layer`, projectID, revision, detail.CityID, detail.HouseID)
	if err != nil {
		return err
	}
	defer rows.Close()
	waves := map[int]bool{}
	for rows.Next() {
		block, err := scanPuzzleHouseBlock(rows)
		if err != nil {
			return err
		}
		waves[block.WaveIndex] = true
		detail.Blocks = append(detail.Blocks, block)
	}
	detail.WaveCount = len(waves)
	return rows.Err()
}

type rowScanner interface{ Scan(dest ...any) error }

func scanPuzzleHouseBlock(rows rowScanner) (PuzzleHouseBlock, error) {
	var block PuzzleHouseBlock
	var minX, minY, maxX, maxY *int
	var outline, groups []byte
	block.RequiredGroups = [][]int{}
	block.FallsByReason = map[string]int64{}
	if err := rows.Scan(&block.BlockID, &block.WaveIndex, &block.OrderInLayer, &block.IsGround,
		&block.VisualKey, &minX, &minY, &maxX, &maxY, &outline, &groups); err != nil {
		return block, err
	}
	if minX != nil && minY != nil && maxX != nil && maxY != nil {
		block.Bounds = &PuzzleBlockBounds{MinX: *minX, MinY: *minY, MaxX: *maxX, MaxY: *maxY}
	}
	if len(outline) > 0 {
		_ = json.Unmarshal(outline, &block.Outline)
	}
	if len(groups) > 0 {
		_ = json.Unmarshal(groups, &block.RequiredGroups)
	}
	return block, nil
}

// loadPuzzleHouseMetrics folds play outcomes onto the blocks already
// loaded. A block with no events keeps its zeroes, which the dashboard
// renders as "not played yet" rather than as a perfect score.
func loadPuzzleHouseMetrics(ctx context.Context, pool *pgxpool.Pool, detail *PuzzleHouseDetail, projectID string, from, to time.Time) error {
	rows, err := pool.Query(ctx, `
SELECT (properties->>'block_id')::int block_id,
       COALESCE(properties->>'outcome','') outcome,
       COUNT(*)
FROM events
WHERE project_id=$1 AND name='placement_resolved'
  AND effective_at>=$2 AND effective_at<$3
  AND (properties->>'city_id')::int=$4 AND (properties->>'house_id')::int=$5
  AND properties ? 'block_id'
GROUP BY 1,2`, projectID, from, to, detail.CityID, detail.HouseID)
	if err != nil {
		return err
	}
	defer rows.Close()
	byBlock := map[int]map[string]int64{}
	for rows.Next() {
		var blockID int
		var outcome string
		var count int64
		if err := rows.Scan(&blockID, &outcome, &count); err != nil {
			return err
		}
		if byBlock[blockID] == nil {
			byBlock[blockID] = map[string]int64{}
		}
		byBlock[blockID][outcome] = count
	}
	if err := rows.Err(); err != nil {
		return err
	}
	applyPuzzleBlockOutcomes(detail, byBlock)
	return nil
}

func applyPuzzleBlockOutcomes(detail *PuzzleHouseDetail, byBlock map[int]map[string]int64) {
	for i := range detail.Blocks {
		block := &detail.Blocks[i]
		for outcome, count := range byBlock[block.BlockID] {
			block.Placements += count
			if len(outcome) > 5 && outcome[:5] == "fell_" {
				block.Falls += count
				block.FallsByReason[outcome] = count
			}
		}
		if block.Placements > 0 {
			block.FallRate = float64(block.Falls) / float64(block.Placements)
		}
		block.RateIsReliable = block.Placements >= minPlacementsForRate
	}
}
