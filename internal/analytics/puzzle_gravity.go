package analytics

import (
	"net/url"
	"regexp"
	"strconv"

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
	var filter GameplayFilter
	var err error
	if filter.CityID, err = parse("city_id"); err != nil {
		return filter, err
	}
	if filter.HouseID, err = parse("house_id"); err != nil {
		return filter, err
	}
	filter.WaveIndex, err = parse("wave_index")
	return filter, err
}
