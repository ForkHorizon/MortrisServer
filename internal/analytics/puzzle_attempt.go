package analytics

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
)

type GameplayAttemptEvent struct {
	EventID, Name        string
	EffectiveAt          time.Time       `json:"effective_at"`
	Properties           json.RawMessage `json:"properties"`
	MissingSupportGroups [][]int         `json:"missing_support_groups,omitempty"`
}

type GameplayAttempt struct {
	AttemptID       string                 `json:"attempt_id"`
	ContentRevision string                 `json:"content_revision"`
	Events          []GameplayAttemptEvent `json:"events"`
	Truncated       bool                   `json:"truncated"`
}

func GetGameplayAttempt(ctx context.Context, pool *pgxpool.Pool, projectID, attemptID string) (*GameplayAttempt, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result, err := loadGameplayAttemptEvents(ctx, pool, projectID, attemptID)
	if err != nil || len(result.Events) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, apierr.New(404, "not_found", "attempt not found")
	}
	catalog, err := loadAttemptCatalog(ctx, pool, projectID, result.ContentRevision)
	if err != nil {
		return nil, err
	}
	populateMissingSupportGroups(result.Events, catalog)
	return result, nil
}

func loadGameplayAttemptEvents(ctx context.Context, pool *pgxpool.Pool, projectID, attemptID string) (*GameplayAttempt, error) {
	result := &GameplayAttempt{AttemptID: attemptID, Events: []GameplayAttemptEvent{}}
	rows, err := pool.Query(ctx, `SELECT event_id,name,effective_at,properties FROM events WHERE project_id=$1 AND properties->>'attempt_id'=$2 ORDER BY effective_at LIMIT 501`, projectID, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var event GameplayAttemptEvent
		if err := rows.Scan(&event.EventID, &event.Name, &event.EffectiveAt, &event.Properties); err != nil {
			return nil, err
		}
		setAttemptContentRevision(result, event.Properties)
		result.Events = append(result.Events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result.Events) > 500 {
		result.Events = result.Events[:500]
		result.Truncated = true
	}
	return result, nil
}

func setAttemptContentRevision(result *GameplayAttempt, properties json.RawMessage) {
	if result.ContentRevision != "" {
		return
	}
	var payload map[string]any
	_ = json.Unmarshal(properties, &payload)
	result.ContentRevision, _ = payload["content_revision"].(string)
}

func loadAttemptCatalog(ctx context.Context, pool *pgxpool.Pool, projectID, revision string) (PuzzleCatalog, error) {
	var raw json.RawMessage
	err := pool.QueryRow(ctx, `SELECT catalog FROM puzzle_content_revisions WHERE project_id=$1 AND content_revision=$2`, projectID, revision).Scan(&raw)
	if err != nil && err != pgx.ErrNoRows {
		return PuzzleCatalog{}, err
	}
	var catalog PuzzleCatalog
	_ = json.Unmarshal(raw, &catalog)
	return catalog, nil
}

func populateMissingSupportGroups(events []GameplayAttemptEvent, catalog PuzzleCatalog) {
	installed := map[int]bool{}
	for i := range events {
		if events[i].Name != "placement_resolved" {
			continue
		}
		payload, ok := attemptEventPayload(events[i].Properties)
		if !ok {
			continue
		}
		installed = applyPlacementState(&events[i], catalog, installed, payload)
	}
}

func applyPlacementState(event *GameplayAttemptEvent, catalog PuzzleCatalog, installed map[int]bool, payload map[string]any) map[int]bool {
	if ids, ok := payload["placed_block_ids"].(string); ok {
		installed = integerSet(ids)
	}
	city, house, target := number(payload["city_id"]), number(payload["house_id"]), number(payload["candidate_target_id"])
	event.MissingSupportGroups = missingGroups(catalog, city, house, target, installed)
	blockID := number(payload["block_id"])
	switch payload["outcome"] {
	case "placed":
		installed[blockID] = true
	case "returned":
		delete(installed, blockID)
	}
	return installed
}

func attemptEventPayload(raw json.RawMessage) (map[string]any, bool) {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil, false
	}
	return payload, true
}

func number(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		result, _ := strconv.Atoi(typed)
		return result
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

func missingGroups(catalog PuzzleCatalog, cityID, houseID, targetID int, installed map[int]bool) [][]int {
	for _, city := range catalog.Cities {
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
				return missingRequiredBlocks(rule.AlternativeRequiredGroups, installed)
			}
		}
	}
	return nil
}

func missingRequiredBlocks(groups [][]int, installed map[int]bool) [][]int {
	result := make([][]int, 0, len(groups))
	for _, group := range groups {
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
