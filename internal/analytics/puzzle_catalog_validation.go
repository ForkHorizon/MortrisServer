package analytics

import (
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

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
		if err := validatePuzzleCity(city); err != nil {
			return err
		}
	}
	return nil
}

func validatePuzzleCity(city PuzzleCatalogCity) error {
	houses := map[int]bool{}
	for _, house := range city.Houses {
		if houses[house.HouseID] {
			return invalidCatalog("duplicate house_id in city")
		}
		houses[house.HouseID] = true
		if err := validatePuzzleHouse(house); err != nil {
			return err
		}
	}
	return nil
}

func validatePuzzleHouse(house PuzzleCatalogHouse) error {
	blocks, err := validatePuzzleBlocks(house.Blocks)
	if err != nil {
		return err
	}
	waves, err := validatePuzzleWaves(house.Waves, blocks)
	if err != nil {
		return err
	}
	if err := validateInventoryGroups(house.InventoryGroups, blocks); err != nil {
		return err
	}
	targets, err := validatePuzzleTargets(house.Targets, blocks, waves)
	if err != nil {
		return err
	}
	return validatePlacementRules(house.PlacementRules, blocks, targets)
}

func validatePuzzleBlocks(blocks []PuzzleCatalogBlock) (map[int]bool, error) {
	known := map[int]bool{}
	for _, block := range blocks {
		if known[block.BlockID] {
			return nil, invalidCatalog("duplicate block_id in house")
		}
		known[block.BlockID] = true
	}
	return known, nil
}

func validatePuzzleWaves(waves []PuzzleCatalogWave, blocks map[int]bool) (map[int]bool, error) {
	known := map[int]bool{}
	for _, wave := range waves {
		if known[wave.WaveIndex] {
			return nil, invalidCatalog("duplicate wave_index in house")
		}
		known[wave.WaveIndex] = true
		for _, id := range wave.BlockIDs {
			if !blocks[id] {
				return nil, invalidCatalog("wave references unknown block_id")
			}
		}
	}
	return known, nil
}

func validateInventoryGroups(groups []PuzzleInventoryGroup, blocks map[int]bool) error {
	known := map[string]bool{}
	for _, group := range groups {
		if group.InventoryGroupID == "" || known[group.InventoryGroupID] {
			return invalidCatalog("empty or duplicate inventory_group_id")
		}
		known[group.InventoryGroupID] = true
		for _, id := range group.BlockIDs {
			if !blocks[id] {
				return invalidCatalog("inventory group references unknown block_id")
			}
		}
	}
	return nil
}

func validatePuzzleTargets(targets []PuzzleCatalogTarget, blocks, waves map[int]bool) (map[int]bool, error) {
	known := map[int]bool{}
	for _, target := range targets {
		if known[target.TargetID] {
			return nil, invalidCatalog("duplicate target_id in house")
		}
		known[target.TargetID] = true
		if !waves[target.WaveIndex] {
			return nil, invalidCatalog("target references unknown wave_index")
		}
		for _, id := range target.CompatibleBlockIDs {
			if !blocks[id] {
				return nil, invalidCatalog("target references unknown compatible block_id")
			}
		}
	}
	return known, nil
}

func validatePlacementRules(rules []PuzzlePlacementRule, blocks, targets map[int]bool) error {
	known := map[int]bool{}
	for _, rule := range rules {
		if known[rule.TargetID] {
			return invalidCatalog("duplicate placement rule target_id")
		}
		known[rule.TargetID] = true
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
	return nil
}

func invalidCatalog(message string) error {
	return apierr.New(400, contracts.CodeInvalidRequest, "invalid puzzle catalogue: "+message)
}
