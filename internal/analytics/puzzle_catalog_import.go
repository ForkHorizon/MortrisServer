package analytics

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ImportPuzzleCatalog(ctx context.Context, pool *pgxpool.Pool, projectID string, catalog PuzzleCatalog) error {
	if err := catalog.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	payload, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	inserted, err := insertPuzzleRevision(ctx, tx, projectID, catalog, payload)
	if err != nil || !inserted {
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err := insertPuzzleCatalogRows(ctx, tx, projectID, catalog); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertPuzzleRevision(ctx context.Context, tx pgx.Tx, projectID string, catalog PuzzleCatalog, payload []byte) (bool, error) {
	command, err := tx.Exec(ctx, `INSERT INTO puzzle_content_revisions (project_id, content_revision, schema_version, catalog) VALUES ($1,$2,$3,$4::jsonb) ON CONFLICT DO NOTHING`, projectID, catalog.ContentRevision, catalog.SchemaVersion, string(payload))
	return command.RowsAffected() > 0, err
}

func insertPuzzleCatalogRows(ctx context.Context, tx pgx.Tx, projectID string, catalog PuzzleCatalog) error {
	for _, city := range catalog.Cities {
		for _, house := range city.Houses {
			if err := insertPuzzleHouse(ctx, tx, projectID, catalog.ContentRevision, city.CityID, house); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertPuzzleHouse(ctx context.Context, tx pgx.Tx, projectID, revision string, cityID int, house PuzzleCatalogHouse) error {
	waveByBlock := map[int]int{}
	for _, wave := range house.Waves {
		for _, id := range wave.BlockIDs {
			waveByBlock[id] = wave.WaveIndex
		}
	}
	if err := insertPuzzleBlocks(ctx, tx, projectID, revision, cityID, house, waveByBlock); err != nil {
		return err
	}
	if err := insertPuzzleTargets(ctx, tx, projectID, revision, cityID, house); err != nil {
		return err
	}
	return insertPuzzleRules(ctx, tx, projectID, revision, cityID, house)
}

func insertPuzzleBlocks(ctx context.Context, tx pgx.Tx, projectID, revision string, cityID int, house PuzzleCatalogHouse, waves map[int]int) error {
	for _, block := range house.Blocks {
		_, err := tx.Exec(ctx, `INSERT INTO puzzle_content_blocks (project_id,content_revision,city_id,house_id,wave_index,block_id,order_in_layer,visual_key,is_ground,local_x_milli,local_y_milli) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, projectID, revision, cityID, house.HouseID, waves[block.BlockID], block.BlockID, block.OrderInLayer, block.VisualKey, block.IsGround, block.LocalXMilli, block.LocalYMilli)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertPuzzleTargets(ctx context.Context, tx pgx.Tx, projectID, revision string, cityID int, house PuzzleCatalogHouse) error {
	for _, target := range house.Targets {
		compatible, _ := json.Marshal(target.CompatibleBlockIDs)
		_, err := tx.Exec(ctx, `INSERT INTO puzzle_content_targets (project_id,content_revision,city_id,house_id,wave_index,target_id,compatible_block_ids,local_x_milli,local_y_milli) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)`, projectID, revision, cityID, house.HouseID, target.WaveIndex, target.TargetID, string(compatible), target.LocalXMilli, target.LocalYMilli)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertPuzzleRules(ctx context.Context, tx pgx.Tx, projectID, revision string, cityID int, house PuzzleCatalogHouse) error {
	for _, rule := range house.PlacementRules {
		groups, _ := json.Marshal(rule.AlternativeRequiredGroups)
		_, err := tx.Exec(ctx, `INSERT INTO puzzle_content_rules (project_id,content_revision,city_id,house_id,target_id,alternative_groups) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, projectID, revision, cityID, house.HouseID, rule.TargetID, string(groups))
		if err != nil {
			return err
		}
	}
	return nil
}
