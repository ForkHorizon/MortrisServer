package analytics

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

// PuzzleCatalog is intentionally an import format, not a projection of the
// Unity runtime. It makes every historical content_revision self-contained.
type PuzzleCatalog struct {
	SchemaVersion   int                 `json:"schema_version"`
	ContentRevision string              `json:"content_revision"`
	Cities          []PuzzleCatalogCity `json:"cities"`
}
type PuzzleCatalogCity struct {
	CityID int                  `json:"city_id"`
	Houses []PuzzleCatalogHouse `json:"houses"`
}
type PuzzleCatalogHouse struct {
	HouseID         int                    `json:"house_id"`
	AssetKey        string                 `json:"asset_key"`
	Waves           []PuzzleCatalogWave    `json:"waves"`
	Blocks          []PuzzleCatalogBlock   `json:"blocks"`
	InventoryGroups []PuzzleInventoryGroup `json:"inventory_groups"`
	Targets         []PuzzleCatalogTarget  `json:"targets"`
	PlacementRules  []PuzzlePlacementRule  `json:"placement_rules"`
	DisplayLabel    string                 `json:"display_label,omitempty"`
	PreviewAssetKey string                 `json:"preview_asset_key,omitempty"`
}
type PuzzleCatalogWave struct {
	WaveIndex int   `json:"wave_index"`
	BlockIDs  []int `json:"block_ids"`
}
type PuzzleCatalogBlock struct {
	BlockID      int    `json:"block_id"`
	OrderInLayer int    `json:"order_in_layer"`
	VisualKey    string `json:"visual_key"`
	IsGround     bool   `json:"is_ground"`
	LocalXMilli  int    `json:"local_x_milli"`
	LocalYMilli  int    `json:"local_y_milli"`
}
type PuzzleInventoryGroup struct {
	InventoryGroupID string `json:"inventory_group_id"`
	BlockIDs         []int  `json:"block_ids"`
}
type PuzzleCatalogTarget struct {
	TargetID           int   `json:"target_id"`
	WaveIndex          int   `json:"wave_index"`
	CompatibleBlockIDs []int `json:"compatible_block_ids"`
	LocalXMilli        int   `json:"local_x_milli"`
	LocalYMilli        int   `json:"local_y_milli"`
}
type PuzzlePlacementRule struct {
	TargetID                  int     `json:"target_id"`
	AlternativeRequiredGroups [][]int `json:"alternative_required_block_groups"`
}

var revisionPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (c PuzzleCatalog) Validate() error {
	if c.SchemaVersion != 1 {
		return apierr.New(400, contracts.CodeInvalidRequest, "schema_version must be 1")
	}
	if !revisionPattern.MatchString(c.ContentRevision) {
		return apierr.New(400, contracts.CodeInvalidRequest, "content_revision must be a lowercase SHA-256 hex string")
	}
	if len(c.Cities) == 0 {
		return apierr.New(400, contracts.CodeInvalidRequest, "catalogue must contain at least one city")
	}
	cities := map[int]bool{}
	for _, city := range c.Cities {
		if cities[city.CityID] {
			return invalidCatalog("duplicate city_id")
		}
		cities[city.CityID] = true
		houses := map[int]bool{}
		for _, house := range city.Houses {
			if houses[house.HouseID] {
				return invalidCatalog("duplicate house_id in city")
			}
			houses[house.HouseID] = true
			blocks, waves, targets := map[int]bool{}, map[int]bool{}, map[int]bool{}
			for _, block := range house.Blocks {
				if blocks[block.BlockID] {
					return invalidCatalog("duplicate block_id in house")
				}
				blocks[block.BlockID] = true
			}
			for _, wave := range house.Waves {
				if waves[wave.WaveIndex] {
					return invalidCatalog("duplicate wave_index in house")
				}
				waves[wave.WaveIndex] = true
				for _, id := range wave.BlockIDs {
					if !blocks[id] {
						return invalidCatalog("wave references unknown block_id")
					}
				}
			}
			groups := map[string]bool{}
			for _, group := range house.InventoryGroups {
				if group.InventoryGroupID == "" || groups[group.InventoryGroupID] {
					return invalidCatalog("empty or duplicate inventory_group_id")
				}
				groups[group.InventoryGroupID] = true
				for _, id := range group.BlockIDs {
					if !blocks[id] {
						return invalidCatalog("inventory group references unknown block_id")
					}
				}
			}
			for _, target := range house.Targets {
				if targets[target.TargetID] {
					return invalidCatalog("duplicate target_id in house")
				}
				targets[target.TargetID] = true
				if !waves[target.WaveIndex] {
					return invalidCatalog("target references unknown wave_index")
				}
				for _, id := range target.CompatibleBlockIDs {
					if !blocks[id] {
						return invalidCatalog("target references unknown compatible block_id")
					}
				}
			}
			rules := map[int]bool{}
			for _, rule := range house.PlacementRules {
				if rules[rule.TargetID] {
					return invalidCatalog("duplicate placement rule target_id")
				}
				rules[rule.TargetID] = true
				if !targets[rule.TargetID] {
					return invalidCatalog("placement rule references unknown target_id")
				}
				for _, group := range rule.AlternativeRequiredGroups {
					for _, id := range group {
						if !blocks[id] {
							return invalidCatalog("placement rule references unknown block_id")
						}
					}
				}
			}
		}
	}
	return nil
}
func invalidCatalog(message string) error {
	return apierr.New(400, contracts.CodeInvalidRequest, "invalid puzzle catalogue: "+message)
}

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
	command, err := tx.Exec(ctx, `INSERT INTO puzzle_content_revisions (project_id, content_revision, schema_version, catalog) VALUES ($1,$2,$3,$4::jsonb) ON CONFLICT DO NOTHING`, projectID, catalog.ContentRevision, catalog.SchemaVersion, string(payload))
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return tx.Commit(ctx)
	} // immutable and idempotent
	for _, city := range catalog.Cities {
		for _, house := range city.Houses {
			waveByBlock := map[int]int{}
			for _, wave := range house.Waves {
				for _, id := range wave.BlockIDs {
					waveByBlock[id] = wave.WaveIndex
				}
			}
			for _, block := range house.Blocks {
				if _, err := tx.Exec(ctx, `INSERT INTO puzzle_content_blocks (project_id,content_revision,city_id,house_id,wave_index,block_id,order_in_layer,visual_key,is_ground,local_x_milli,local_y_milli) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, projectID, catalog.ContentRevision, city.CityID, house.HouseID, waveByBlock[block.BlockID], block.BlockID, block.OrderInLayer, block.VisualKey, block.IsGround, block.LocalXMilli, block.LocalYMilli); err != nil {
					return err
				}
			}
			for _, target := range house.Targets {
				compatible, _ := json.Marshal(target.CompatibleBlockIDs)
				if _, err := tx.Exec(ctx, `INSERT INTO puzzle_content_targets (project_id,content_revision,city_id,house_id,wave_index,target_id,compatible_block_ids,local_x_milli,local_y_milli) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)`, projectID, catalog.ContentRevision, city.CityID, house.HouseID, target.WaveIndex, target.TargetID, string(compatible), target.LocalXMilli, target.LocalYMilli); err != nil {
					return err
				}
			}
			for _, rule := range house.PlacementRules {
				groups, _ := json.Marshal(rule.AlternativeRequiredGroups)
				if _, err := tx.Exec(ctx, `INSERT INTO puzzle_content_rules (project_id,content_revision,city_id,house_id,target_id,alternative_groups) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, projectID, catalog.ContentRevision, city.CityID, house.HouseID, rule.TargetID, string(groups)); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit(ctx)
}

type GameplayFilter struct{ CityID, HouseID, WaveIndex *int }

func ParseGameplayFilter(q url.Values) (GameplayFilter, error) {
	parse := func(name string) (*int, error) {
		raw := q.Get(name)
		if raw == "" {
			return nil, nil
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, apierr.New(400, contracts.CodeInvalidRequest, name+" must be an integer")
		}
		return &value, nil
	}
	var f GameplayFilter
	var err error
	if f.CityID, err = parse("city_id"); err != nil {
		return f, err
	}
	if f.HouseID, err = parse("house_id"); err != nil {
		return f, err
	}
	f.WaveIndex, err = parse("wave_index")
	return f, err
}

type GameplaySummary struct {
	Attempts        int64 `json:"attempts"`
	Placements      int64 `json:"placements"`
	Falls           int64 `json:"falls"`
	Hints           int64 `json:"hints"`
	CompletedWaves  int64 `json:"completed_waves"`
	CompletedHouses int64 `json:"completed_houses"`
	ActiveElapsedMS int64 `json:"active_elapsed_ms"`
	WallElapsedMS   int64 `json:"wall_elapsed_ms"`
	PauseCount      int64 `json:"pause_count"`
	PauseElapsedMS  int64 `json:"pause_elapsed_ms"`
}
type GameplayScope struct {
	CityID          int   `json:"city_id"`
	HouseID         int   `json:"house_id"`
	WaveIndex       int   `json:"wave_index"`
	Attempts        int64 `json:"attempts"`
	Placements      int64 `json:"placements"`
	Falls           int64 `json:"falls"`
	Hints           int64 `json:"hints"`
	CompletedWaves  int64 `json:"completed_waves"`
	CompletedHouses int64 `json:"completed_houses"`
	ActiveElapsedMS int64 `json:"active_elapsed_ms"`
	WallElapsedMS   int64 `json:"wall_elapsed_ms"`
	PauseCount      int64 `json:"pause_count"`
	PauseElapsedMS  int64 `json:"pause_elapsed_ms"`
}
type GameplayFriction struct {
	BlockID, TargetID       int     `json:"block_id"`
	Attempts                int64   `json:"attempts"`
	Placements              int64   `json:"placements"`
	Falls                   int64   `json:"falls"`
	Hints                   int64   `json:"hints"`
	FallRate                float64 `json:"fall_rate"`
	FirstAttemptFailureRate float64 `json:"first_attempt_failure_rate"`
}
type GameplayDaily struct {
	Day             string `json:"day"`
	CityID          int    `json:"city_id"`
	HouseID         int    `json:"house_id"`
	WaveIndex       int    `json:"wave_index"`
	ContentRevision string `json:"content_revision"`
	BuildNumber     string `json:"build_number"`
	Attempts        int64  `json:"attempts"`
	Placements      int64  `json:"placements"`
	Falls           int64  `json:"falls"`
	Hints           int64  `json:"hints"`
}
type GameplayDiagnostics struct {
	Summary  GameplaySummary    `json:"summary"`
	Scopes   []GameplayScope    `json:"scopes"`
	Friction []GameplayFriction `json:"friction"`
	Daily    []GameplayDaily    `json:"daily"`
}

func GetGameplayDiagnostics(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location, f GameplayFilter) (*GameplayDiagnostics, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result := &GameplayDiagnostics{Scopes: []GameplayScope{}, Friction: []GameplayFriction{}, Daily: []GameplayDaily{}}
	// The maxima are client boundary snapshots; the wall time is derived from raw server-normalized timestamps, never a duplicated client counter.
	err := pool.QueryRow(ctx, `WITH e AS (SELECT name, effective_at, properties FROM events WHERE project_id=$1 AND event_kind='product' AND effective_at >= $2 AND effective_at < $3 AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)), a AS (SELECT properties->>'attempt_id' attempt_id, MAX((properties->>'active_elapsed_ms')::bigint) active_ms, EXTRACT(EPOCH FROM MAX(effective_at)-MIN(effective_at))*1000 wall_ms, COUNT(*) FILTER (WHERE name='app_backgrounded') pauses FROM e WHERE properties ? 'attempt_id' GROUP BY 1) SELECT (SELECT COUNT(DISTINCT attempt_id) FROM a), (SELECT COUNT(*) FROM e WHERE name='placement_resolved'), (SELECT COUNT(*) FROM e WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'), (SELECT COUNT(*) FROM e WHERE name='hint_used'), (SELECT COUNT(*) FROM e WHERE name='wave_completed'), (SELECT COUNT(*) FROM e WHERE name='house_completed'), COALESCE((SELECT SUM(active_ms) FROM a),0), COALESCE((SELECT SUM(wall_ms) FROM a),0)::bigint, COALESCE((SELECT SUM(pauses) FROM a),0), COALESCE((SELECT SUM(GREATEST(wall_ms-active_ms,0)) FROM a),0)::bigint`, projectID, from, to, f.CityID, f.HouseID, f.WaveIndex).Scan(&result.Summary.Attempts, &result.Summary.Placements, &result.Summary.Falls, &result.Summary.Hints, &result.Summary.CompletedWaves, &result.Summary.CompletedHouses, &result.Summary.ActiveElapsedMS, &result.Summary.WallElapsedMS, &result.Summary.PauseCount, &result.Summary.PauseElapsedMS)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `WITH e AS (SELECT name,effective_at,properties FROM events WHERE project_id=$1 AND effective_at >=$2 AND effective_at<$3 AND properties ? 'attempt_id' AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)), a AS (SELECT (properties->>'city_id')::int city_id,(properties->>'house_id')::int house_id,(properties->>'wave_index')::int wave_index,properties->>'attempt_id' attempt_id,MAX((properties->>'active_elapsed_ms')::bigint) active_ms,EXTRACT(EPOCH FROM MAX(effective_at)-MIN(effective_at))*1000 wall_ms,COUNT(*) FILTER (WHERE name='app_backgrounded') pauses FROM e GROUP BY 1,2,3,4), m AS (SELECT city_id,house_id,wave_index,COUNT(*) attempts,COALESCE(SUM(active_ms),0)::bigint active_ms,COALESCE(SUM(wall_ms),0)::bigint wall_ms,COALESCE(SUM(pauses),0)::bigint pauses,COALESCE(SUM(GREATEST(wall_ms-active_ms,0)),0)::bigint pause_ms FROM a GROUP BY 1,2,3), c AS (SELECT (properties->>'city_id')::int city_id,(properties->>'house_id')::int house_id,(properties->>'wave_index')::int wave_index,COUNT(*) FILTER (WHERE name='placement_resolved') placements,COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%') falls,COUNT(*) FILTER (WHERE name='hint_used') hints,COUNT(*) FILTER (WHERE name='wave_completed') completed_waves,COUNT(*) FILTER (WHERE name='house_completed') completed_houses FROM e GROUP BY 1,2,3) SELECT m.city_id,m.house_id,m.wave_index,m.attempts,c.placements,c.falls,c.hints,c.completed_waves,c.completed_houses,m.active_ms,m.wall_ms,m.pauses,m.pause_ms FROM m JOIN c USING (city_id,house_id,wave_index) ORDER BY 1,2,3 LIMIT 500`, projectID, from, to, f.CityID, f.HouseID, f.WaveIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row GameplayScope
		if err := rows.Scan(&row.CityID, &row.HouseID, &row.WaveIndex, &row.Attempts, &row.Placements, &row.Falls, &row.Hints, &row.CompletedWaves, &row.CompletedHouses, &row.ActiveElapsedMS, &row.WallElapsedMS, &row.PauseCount, &row.PauseElapsedMS); err != nil {
			return nil, err
		}
		result.Scopes = append(result.Scopes, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = pool.Query(ctx, `WITH p AS (SELECT properties, ROW_NUMBER() OVER (PARTITION BY properties->>'attempt_id', properties->>'block_id', properties->>'candidate_target_id' ORDER BY effective_at) ordinal FROM events WHERE project_id=$1 AND name='placement_resolved' AND effective_at >=$2 AND effective_at<$3 AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)) SELECT (properties->>'block_id')::int, COALESCE((properties->>'candidate_target_id')::int,-1),COUNT(*),COUNT(*) FILTER (WHERE properties->>'outcome'='placed'),COUNT(*) FILTER (WHERE properties->>'outcome' LIKE 'fell_%'),COUNT(*) FILTER (WHERE ordinal=1 AND properties->>'outcome' LIKE 'fell_%') FROM p GROUP BY 1,2 ORDER BY 5 DESC,3 DESC LIMIT 500`, projectID, from, to, f.CityID, f.HouseID, f.WaveIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row GameplayFriction
		var firstFailures int64
		if err := rows.Scan(&row.BlockID, &row.TargetID, &row.Attempts, &row.Placements, &row.Falls, &firstFailures); err != nil {
			return nil, err
		}
		if row.Attempts > 0 {
			row.FallRate = float64(row.Falls) / float64(row.Attempts)
			row.FirstAttemptFailureRate = float64(firstFailures) / float64(row.Attempts)
		}
		result.Friction = append(result.Friction, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = pool.Query(ctx, `SELECT TO_CHAR(effective_at AT TIME ZONE $7,'YYYY-MM-DD'),(properties->>'city_id')::int,(properties->>'house_id')::int,(properties->>'wave_index')::int,properties->>'content_revision',build_number,COUNT(DISTINCT properties->>'attempt_id'),COUNT(*) FILTER (WHERE name='placement_resolved'),COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),COUNT(*) FILTER (WHERE name='hint_used') FROM events WHERE project_id=$1 AND effective_at >=$2 AND effective_at<$3 AND properties ? 'attempt_id' AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6) GROUP BY 1,2,3,4,5,6 ORDER BY 1 DESC,2,3,4 LIMIT 1000`, projectID, from, to, f.CityID, f.HouseID, f.WaveIndex, loc.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row GameplayDaily
		if err := rows.Scan(&row.Day, &row.CityID, &row.HouseID, &row.WaveIndex, &row.ContentRevision, &row.BuildNumber, &row.Attempts, &row.Placements, &row.Falls, &row.Hints); err != nil {
			return nil, err
		}
		result.Daily = append(result.Daily, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

type GameplayAttemptEvent struct {
	EventID, Name        string
	EffectiveAt          time.Time       `json:"effective_at"`
	Properties           json.RawMessage `json:"properties"`
	MissingSupportGroups [][]int         `json:"missing_support_groups,omitempty"`
}
type GameplayAttempt struct {
	AttemptID, ContentRevision string                 `json:"attempt_id"`
	Events                     []GameplayAttemptEvent `json:"events"`
	Truncated                  bool                   `json:"truncated"`
}

func GetGameplayAttempt(ctx context.Context, pool *pgxpool.Pool, projectID, attemptID string) (*GameplayAttempt, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result := &GameplayAttempt{AttemptID: attemptID, Events: []GameplayAttemptEvent{}}
	rows, err := pool.Query(ctx, `SELECT event_id,name,effective_at,properties FROM events WHERE project_id=$1 AND properties->>'attempt_id'=$2 ORDER BY effective_at LIMIT 501`, projectID, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e GameplayAttemptEvent
		if err := rows.Scan(&e.EventID, &e.Name, &e.EffectiveAt, &e.Properties); err != nil {
			return nil, err
		}
		if result.ContentRevision == "" {
			var p map[string]any
			_ = json.Unmarshal(e.Properties, &p)
			result.ContentRevision, _ = p["content_revision"].(string)
		}
		result.Events = append(result.Events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result.Events) > 500 {
		result.Events = result.Events[:500]
		result.Truncated = true
	}
	if len(result.Events) == 0 {
		return nil, apierr.New(404, "not_found", "attempt not found")
	}
	var raw json.RawMessage
	if err := pool.QueryRow(ctx, `SELECT catalog FROM puzzle_content_revisions WHERE project_id=$1 AND content_revision=$2`, projectID, result.ContentRevision).Scan(&raw); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	var catalog PuzzleCatalog
	_ = json.Unmarshal(raw, &catalog)
	installed := map[int]bool{}
	for i := range result.Events {
		if result.Events[i].Name != "placement_resolved" {
			continue
		}
		var p map[string]any
		if json.Unmarshal(result.Events[i].Properties, &p) != nil {
			continue
		}
		// The payload describes state before the outcome. When its compact form
		// is omitted for size, carry forward the last known successful state.
		if ids, ok := p["placed_block_ids"].(string); ok {
			installed = integerSet(ids)
		}
		city, house, target := number(p["city_id"]), number(p["house_id"]), number(p["candidate_target_id"])
		result.Events[i].MissingSupportGroups = missingGroups(catalog, city, house, target, installed)
		blockID := number(p["block_id"])
		switch p["outcome"] {
		case "placed":
			installed[blockID] = true
		case "returned":
			delete(installed, blockID)
		}
	}
	return result, nil
}
func number(value any) int {
	switch x := value.(type) {
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return -1
}
func integerSet(csv string) map[int]bool {
	set := map[int]bool{}
	for _, part := range strings.Split(csv, ",") {
		id, err := strconv.Atoi(part)
		if err == nil {
			set[id] = true
		}
	}
	return set
}
func missingGroups(c PuzzleCatalog, cityID, houseID, targetID int, installed map[int]bool) [][]int {
	for _, city := range c.Cities {
		if city.CityID != cityID {
			continue
		}
		for _, house := range city.Houses {
			if house.HouseID != houseID {
				continue
			}
			for _, rule := range house.PlacementRules {
				if rule.TargetID != targetID {
					continue
				}
				result := make([][]int, 0, len(rule.AlternativeRequiredGroups))
				for _, group := range rule.AlternativeRequiredGroups {
					missing := []int{}
					for _, id := range group {
						if !installed[id] {
							missing = append(missing, id)
						}
					}
					result = append(result, missing)
				}
				return result
			}
		}
	}
	return nil
}
