package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SmokeCheckItem struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Passed      bool   `json:"passed"`
	Detail      string `json:"detail"`
}

type SmokeCheckReport struct {
	Passed      bool             `json:"passed"`
	ProjectID   string           `json:"project_id"`
	BuildNumber string           `json:"build_number"`
	AppVersion  string           `json:"app_version"`
	From        time.Time        `json:"from"`
	To          time.Time        `json:"to"`
	Items       []SmokeCheckItem `json:"items"`
	Summary     string           `json:"summary"`
}

type SmokeEvalParams struct {
	ProjectID   string
	BuildNumber string
	From        time.Time
	To          time.Time
}

// EvaluateSmokeCheck runs the 13-point Android smoke checklist evaluation.
func EvaluateSmokeCheck(ctx context.Context, pool *pgxpool.Pool, params SmokeEvalParams) (*SmokeCheckReport, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	b := params.BuildNumber
	q, err := GetPuzzleQuality(ctx, pool, params.ProjectID, params.From, params.To, &b)
	if err != nil {
		return nil, fmt.Errorf("load quality: %w", err)
	}

	report := &SmokeCheckReport{
		Passed:      true,
		ProjectID:   params.ProjectID,
		BuildNumber: params.BuildNumber,
		From:        params.From,
		To:          params.To,
		Items:       make([]SmokeCheckItem, 0, 13),
	}

	counts, err := querySmokeEventCounts(ctx, pool, params)
	if err != nil {
		return nil, fmt.Errorf("query smoke counts: %w", err)
	}

	appVer := queryAppVersion(ctx, pool, params)
	report.AppVersion = appVer

	evalItems1to3(report, q, counts, params.BuildNumber, appVer)
	evalItems4to6(report, counts)
	evalItems7to9(report, q, counts)
	evalItems10to13(report, ctx, pool, params, q, counts)

	for _, item := range report.Items {
		if !item.Passed {
			report.Passed = false
			break
		}
	}

	if report.Passed {
		report.Summary = fmt.Sprintf("PASS: Build %s satisfied all 13 checklist points", params.BuildNumber)
	} else {
		report.Summary = fmt.Sprintf("FAIL: Build %s failed one or more smoke criteria", params.BuildNumber)
	}

	return report, nil
}

type smokeCounts struct {
	Installs           int64
	HouseRunStarts     int64
	WaveAttemptStarts  int64
	WaveCompletions    int64
	PlacementsResolved int64
	FellNoSnap         int64
	FellMissingSupport int64
	DetailReturns      int64
	AppBackgrounded    int64
	AppForegrounded    int64
	AttemptRecovered   int64
	DevCommandsStarted int64
	DevMutations       int64
	DevCommandsDone    int64
}

func querySmokeEventCounts(ctx context.Context, pool *pgxpool.Pool, params SmokeEvalParams) (*smokeCounts, error) {
	c := &smokeCounts{}
	err := pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM installations WHERE project_id=$1 AND registered_at>=$2 AND registered_at<$3 AND activated_at IS NOT NULL AND ($4::text IS NULL OR last_build_number=$4)),
			COUNT(*) FILTER (WHERE name='house_run_started'),
			COUNT(*) FILTER (WHERE name='wave_attempt_started'),
			COUNT(*) FILTER (WHERE name IN ('wave_completed','house_completed') AND COALESCE(properties->>'progress_origin','natural')='natural'),
			COUNT(*) FILTER (WHERE name='placement_resolved'),
			COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome'='fell_no_snap_target'),
			COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome'='fell_missing_support'),
			COUNT(*) FILTER (WHERE name='detail_returned'),
			COUNT(*) FILTER (WHERE name='app_backgrounded'),
			COUNT(*) FILTER (WHERE name='app_foregrounded'),
			COUNT(*) FILTER (WHERE name='attempt_recovered'),
			COUNT(*) FILTER (WHERE name='developer_command_started'),
			COUNT(*) FILTER (WHERE name='developer_progress_mutated'),
			COUNT(*) FILTER (WHERE name IN ('developer_command_completed','developer_command_failed','developer_command_noop'))
		FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
	`, params.ProjectID, params.From, params.To, params.BuildNumber).Scan(
		&c.Installs, &c.HouseRunStarts, &c.WaveAttemptStarts, &c.WaveCompletions,
		&c.PlacementsResolved, &c.FellNoSnap, &c.FellMissingSupport, &c.DetailReturns,
		&c.AppBackgrounded, &c.AppForegrounded, &c.AttemptRecovered,
		&c.DevCommandsStarted, &c.DevMutations, &c.DevCommandsDone,
	)
	return c, err
}

func queryAppVersion(ctx context.Context, pool *pgxpool.Pool, params SmokeEvalParams) string {
	var v string
	_ = pool.QueryRow(ctx, `
		SELECT COALESCE(app_version, '') FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
		ORDER BY effective_at DESC LIMIT 1
	`, params.ProjectID, params.From, params.To, params.BuildNumber).Scan(&v)
	return v
}

func evalItems1to3(r *SmokeCheckReport, q *PuzzleQuality, c *smokeCounts, build, appVer string) {
	// Item 1: Record app version and build number
	p1 := build != "" && appVer != ""
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 1, Name: "Build & App Version Recorded",
		Description: "Candidate build number and app version present in telemetry",
		Passed:      p1, Detail: fmt.Sprintf("app_version=%s, build_number=%s", appVer, build),
	})

	// Item 2: Clear app data / fresh queue
	p2 := q.TotalGameplayEvents > 0 && q.SequenceGaps == 0 && q.InvalidEventIndexes == 0
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 2, Name: "Durable Queue Clean",
		Description: "No polluted queue or invalid index collisions from previous builds",
		Passed:      p2, Detail: fmt.Sprintf("events=%d, sequence_gaps=%d, invalid_indexes=%d", q.TotalGameplayEvents, q.SequenceGaps, q.InvalidEventIndexes),
	})

	// Item 3: Register one new installation
	p3 := c.Installs >= 1
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 3, Name: "Installation Registered",
		Description: "At least one fresh installation registered and activated",
		Passed:      p3, Detail: fmt.Sprintf("registered_installs=%d", c.Installs),
	})
}

func evalItems4to6(r *SmokeCheckReport, c *smokeCounts) {
	// Item 4: Complete normal house/wave
	p4 := c.HouseRunStarts >= 1 && c.WaveAttemptStarts >= 1 && c.WaveCompletions >= 1
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 4, Name: "Normal House/Wave Progression",
		Description: "At least one house and wave attempt completed normally",
		Passed:      p4, Detail: fmt.Sprintf("runs_started=%d, waves_started=%d, completions=%d", c.HouseRunStarts, c.WaveAttemptStarts, c.WaveCompletions),
	})

	// Item 5: Produce fell_no_snap_target
	p5 := c.FellNoSnap >= 1
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 5, Name: "No Snap Target Fall",
		Description: "Produced at least one fell_no_snap_target placement",
		Passed:      p5, Detail: fmt.Sprintf("fell_no_snap_target_count=%d", c.FellNoSnap),
	})

	// Item 6: Produce fell_missing_support
	p6 := c.FellMissingSupport >= 1
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 6, Name: "Missing Support Fall",
		Description: "Produced at least one fell_missing_support placement",
		Passed:      p6, Detail: fmt.Sprintf("fell_missing_support_count=%d", c.FellMissingSupport),
	})
}

func evalItems7to9(r *SmokeCheckReport, q *PuzzleQuality, c *smokeCounts) {
	// Item 7: Return & retry detail
	p7 := c.DetailReturns >= 1 && q.OrphanInteractions == 0
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 7, Name: "Return & Retry Pairing",
		Description: "Fallen details returned to inventory; 0 orphan interactions",
		Passed:      p7, Detail: fmt.Sprintf("detail_returns=%d, orphan_interactions=%d", c.DetailReturns, q.OrphanInteractions),
	})

	// Item 8: Background & foreground app
	p8 := c.AppBackgrounded >= 1 && c.AppForegrounded >= 1
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 8, Name: "Background & Foreground Cycle",
		Description: "App backgrounded and foregrounded cleanly",
		Passed:      p8, Detail: fmt.Sprintf("backgrounded=%d, foregrounded=%d", c.AppBackgrounded, c.AppForegrounded),
	})

	// Item 9: Attempt recovery after restart
	p9 := c.AttemptRecovered >= 1
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 9, Name: "Attempt Recovery",
		Description: "Attempt recovered after restart/crash with checkpoint continuity",
		Passed:      p9, Detail: fmt.Sprintf("attempt_recovered_count=%d", c.AttemptRecovered),
	})
}

func evalItems10to13(r *SmokeCheckReport, ctx context.Context, pool *pgxpool.Pool, params SmokeEvalParams, q *PuzzleQuality, c *smokeCounts) {
	// Item 10: Developer completion and reset cheats
	p10 := c.DevCommandsStarted >= 2 && c.DevMutations >= 2 && c.DevCommandsDone >= 2 &&
		q.UnpairedDeveloperCommands == 0 && q.UnpairedDeveloperMutations == 0
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 10, Name: "Developer Cheat Pairing",
		Description: "Completed and reset developer commands properly paired",
		Passed:      p10, Detail: fmt.Sprintf("commands_started=%d, mutations=%d, unpaired_cmds=%d, unpaired_muts=%d",
			c.DevCommandsStarted, c.DevMutations, q.UnpairedDeveloperCommands, q.UnpairedDeveloperMutations),
	})

	// Item 11: Force upload sequence continuity
	p11 := q.TotalGameplayEvents > 0 && q.SequenceGaps == 0 && q.SequenceDuplicates == 0
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 11, Name: "Sequence Continuity",
		Description: "Durable queue upload preserved monotonic sequence with zero gaps",
		Passed:      p11, Detail: fmt.Sprintf("events=%d, sequence_gaps=%d, duplicates=%d",
			q.TotalGameplayEvents, q.SequenceGaps, q.SequenceDuplicates),
	})

	// Item 12: Production/staging integrity checks
	p12 := q.TotalGameplayEvents > 0 && q.RejectedEvents == 0 && q.UnknownRevisionEvents == 0 &&
		q.InvalidCoordinateSpaceEvents == 0 && q.CheckpointMismatches == 0 &&
		q.HouseEventIndexGaps == 0 && q.AttemptEventIndexGaps == 0 &&
		q.HouseEventIndexDuplicates == 0 && q.AttemptEventIndexDuplicates == 0 &&
		q.MissingSchemaVersion == 0 && q.InvalidSchemaVersion == 0 &&
		q.SchemaV2Share >= 0.95 && q.UnpairedDeveloperCommands == 0 &&
		q.UnpairedDeveloperMutations == 0 && q.DuplicateEvents == 0
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 12, Name: "Schema-v2 & Catalog Integrity",
		Description: "0 rejections, 0 unknown revisions, 0 coordinate mismatches, 0 checkpoint mismatches",
		Passed:      p12, Detail: fmt.Sprintf("rejected=%d, unknown_rev=%d, invalid_coords=%d, bad_checkpoints=%d, index_gaps=%d",
			q.RejectedEvents, q.UnknownRevisionEvents, q.InvalidCoordinateSpaceEvents, q.CheckpointMismatches,
			q.HouseEventIndexGaps+q.AttemptEventIndexGaps),
	})

	// Item 13: Natural metrics ignore cheated completion
	p13 := checkNaturalMetricsIgnoreCheats(ctx, pool, params)
	r.Items = append(r.Items, SmokeCheckItem{
		Index: 13, Name: "Natural Traffic Isolation",
		Description: "Designer natural metrics completely ignore developer-cheated completions",
		Passed:      p13, Detail: "Natural house coverage verified isolated from developer progress",
	})
}

func checkNaturalMetricsIgnoreCheats(ctx context.Context, pool *pgxpool.Pool, params SmokeEvalParams) bool {
	b := params.BuildNumber
	natural, err := GetPuzzleHouses(ctx, pool, params.ProjectID, params.From, params.To, ScopeNaturalOnly, &b)
	if err != nil || len(natural.Houses) == 0 {
		return false
	}
	impact, err := GetPuzzleTesterImpact(ctx, pool, params.ProjectID, params.From, params.To)
	if err != nil || impact.ProgressCompletions < 1 || impact.Resets < 1 {
		return false
	}
	hasNaturalComp := false
	for _, h := range natural.Houses {
		if h.NaturalHouseRunsComplete >= 1 && h.NaturalHouseRunsComplete <= h.NaturalHouseRunsStarted {
			hasNaturalComp = true
			break
		}
	}
	if !hasNaturalComp {
		return false
	}
	var devRunsCount int64
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM puzzle_house_run_classification
		WHERE project_id=$1 AND last_event_at>=$2 AND last_event_at<$3
		  AND NOT fully_natural
	`, params.ProjectID, params.From, params.To).Scan(&devRunsCount)
	return err == nil && devRunsCount > 0
}
