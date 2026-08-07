package analytics

// Stage 3 (docs/puzzle-analytics-remaining-plan.md section 6): the one
// shared natural/developer classification every Puzzle query must use
// instead of copying a slightly different COALESCE(...)='natural'
// fragment. Two things live here:
//
//  1. naturalEventPredicate — the row-level check, for metrics that are
//     legitimately event-scoped (a fall rate, a release point: one
//     placement is either natural or it isn't).
//  2. TrafficScope plus the two SQL views from migration 0014
//     (puzzle_wave_attempt_classification, puzzle_house_run_classification)
//     — the run-level check, for metrics whose *unit* is a whole wave
//     attempt or house run (completion, abandonment, duration, funnel).
//     A run is "fully_natural" only if every event carrying its ID is a
//     natural player event; one developer-menu event anywhere in it, or
//     one event whose progress_origin was ever tainted, is enough to
//     disqualify the whole run. That single BOOL_AND is deliberately what
//     both halves of the plan's rule ("a developer command changed this
//     run" OR "its final progress origin is non-natural") collapse into:
//     the client permanently taints progress_origin the moment a
//     progress-changing developer command actually changes something
//     (PuzzleAnalyticsService.cs never reverts it), so a tainted event is
//     never followed by a natural one again for that install, and "any
//     event non-natural" and "the final one is non-natural" agree. A
//     cheat's own diagnostic events (developer_command_completed,
//     developer_progress_mutated) never join back into the run they
//     cheated either way — the client closes the attempt and clears the
//     house run before emitting them, so only developer_command_started
//     still carries the pre-cheat IDs, which is exactly enough to poison
//     that run's classification.
//
// Placement-quality metrics use naturalEventPredicate directly. Completion
// /abandonment/duration/funnel metrics join one of the two classification
// views and filter on fully_natural. Both are scope-aware: the dashboard's
// Natural only/Developer affected/All traffic control (section 6,
// "Dashboard behavior") doesn't just add a WHERE clause, it swaps which
// population a metric answers for, so eventPredicate/runPredicate below
// return the complementary condition for each scope rather than a fixed
// natural-only filter.

import (
	"net/url"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

// naturalEventPredicate is the row-level natural-play check, reused
// verbatim everywhere a query needs it instead of re-typing the
// COALESCE pair. origin/progress_origin default to player/natural for
// pre-schema-v2 events that never carried either key.
const naturalEventPredicate = `COALESCE(properties->>'origin','player')='player' AND COALESCE(properties->>'progress_origin','natural')='natural'`

// TrafficScope selects which population a Puzzle aggregate answers for.
type TrafficScope string

const (
	// ScopeNaturalOnly is the default and the only scope a designer
	// verdict ("players struggle here") may be drawn from.
	ScopeNaturalOnly TrafficScope = "natural_only"
	// ScopeDeveloperAffected isolates tester/debug traffic for
	// investigating cheat usage itself, never for a difficulty claim.
	ScopeDeveloperAffected TrafficScope = "developer_affected"
	// ScopeAllTraffic mixes both and must be visibly marked as such.
	ScopeAllTraffic TrafficScope = "all"
)

// ParseTrafficScope defaults to ScopeNaturalOnly so a caller that forgets
// the parameter still gets the only scope that's safe to show as a
// verdict, rather than silently mixing in cheat traffic.
func ParseTrafficScope(q url.Values) (TrafficScope, error) {
	switch TrafficScope(q.Get("scope")) {
	case "":
		return ScopeNaturalOnly, nil
	case ScopeNaturalOnly:
		return ScopeNaturalOnly, nil
	case ScopeDeveloperAffected:
		return ScopeDeveloperAffected, nil
	case ScopeAllTraffic:
		return ScopeAllTraffic, nil
	default:
		return "", apierr.New(400, contracts.CodeInvalidRequest, "scope must be natural_only, developer_affected, or all")
	}
}

// eventPredicate is naturalEventPredicate itself for ScopeNaturalOnly, its
// negation for ScopeDeveloperAffected, and unconditionally true for
// ScopeAllTraffic. s is always produced by ParseTrafficScope, so this is
// splicing one of three fixed literals into query text, never raw input.
func (s TrafficScope) eventPredicate() string {
	switch s {
	case ScopeDeveloperAffected:
		return "NOT (" + naturalEventPredicate + ")"
	case ScopeAllTraffic:
		return "TRUE"
	default:
		return naturalEventPredicate
	}
}

// runPredicate is the same three-way switch as eventPredicate but against
// a classification view's boolean fully_natural column instead of a raw
// event row.
func (s TrafficScope) runPredicate(fullyNaturalColumn string) string {
	switch s {
	case ScopeDeveloperAffected:
		return "NOT " + fullyNaturalColumn
	case ScopeAllTraffic:
		return "TRUE"
	default:
		return fullyNaturalColumn
	}
}
