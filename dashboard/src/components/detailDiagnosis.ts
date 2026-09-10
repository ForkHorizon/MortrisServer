import type { PuzzleHouseBlock } from '../api/houseTypes'

export type DiagnosisTone = 'neutral' | 'good' | 'friction' | 'warning'

export interface DetailDiagnosisResult {
  sentence: string
  tone: DiagnosisTone
}

export function sampleCountForBlock(block: PuzzleHouseBlock): number {
  return block.retry_ladder && block.retry_ladder.sample_count > 0
    ? block.retry_ladder.sample_count
    : (block.placements ?? 0)
}

export function toneLabel(tone: DiagnosisTone): string {
  switch (tone) {
    case 'good':
      return 'Clean'
    case 'warning':
      return 'Severe friction'
    case 'friction':
      return 'Friction'
    case 'neutral':
    default:
      return 'Too few plays'
  }
}

// leadingFallReason returns the fall cause key with the highest count,
// breaking any ties deterministically by alphabetical order, or null
// if no falls occurred.
export function leadingFallReason(block: PuzzleHouseBlock): string | null {
  if (!block.falls_by_reason) return null
  const entries = Object.entries(block.falls_by_reason).filter(([, count]) => count > 0)
  if (entries.length === 0) return null
  entries.sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
  return entries[0][0]
}

// Rules 3 & 4: Missing support and scattered drops mechanics.
// Leading cause requires both prominence (fall_rate >= 0.25 AND >= 3 falls)
// so that a tiny isolated fraction of falls doesn't claim dominance.
function mechanicalDiagnosis(block: PuzzleHouseBlock): DetailDiagnosisResult | null {
  const leading = leadingFallReason(block)
  const missingRate = block.missing_support_rate ?? 0
  const noSnapRate = block.no_snap_rate ?? 0
  const fallRate = block.fall_rate ?? 0
  const leadingIsProminent = fallRate >= 0.25 && (block.falls ?? 0) >= 3

  const isMissingSupportDominant =
    missingRate >= 0.40 || (leadingIsProminent && leading === 'fell_missing_support')
  const isNoSnapDominant =
    noSnapRate >= 0.40 || (leadingIsProminent && leading === 'fell_no_snap_target')

  if (isMissingSupportDominant && (!isNoSnapDominant || missingRate >= noSnapRate)) {
    return {
      sentence: 'Players release close to the right slot but the required support is missing.',
      tone: 'friction',
    }
  }
  if (isNoSnapDominant) {
    return {
      sentence: 'Drops are widely scattered and usually find no snap target.',
      tone: 'friction',
    }
  }
  return null
}

// Rules 5, 6, 7 & 8: Progression friction, abandonment, hint and fallback.
function progressionDiagnosis(block: PuzzleHouseBlock): DetailDiagnosisResult {
  const ladder = block.retry_ladder
  const medianTries = (ladder && ladder.median_tries > 0) ? ladder.median_tries : (block.median_tries_to_success ?? 0)
  const placements = block.placements ?? 0
  const ladderSample = ladder?.sample_count ?? 0
  const neverSucceeded = ladder?.never_succeeded ?? 0
  const abandonRate = ladderSample > 0 ? neverSucceeded / ladderSample : 0
  const unrecoveredFalls = placements >= 5 && (block.successful_placements ?? 0) === 0 && (block.falls ?? 0) > 0

  // 5. High Retries / Friction Before Success
  const success3rdOr4th = ladder ? (ladder.success_3rd_try ?? 0) + (ladder.success_4th_plus_try ?? 0) : 0
  const success1st = ladder ? (ladder.success_1st_try ?? 0) : 0
  const ladderSuccessSum = ladder
    ? (ladder.success_1st_try ?? 0) + (ladder.success_2nd_try ?? 0) + (ladder.success_3rd_try ?? 0) + (ladder.success_4th_plus_try ?? 0)
    : 0
  const successfulPlacements = (block.successful_placements ?? 0) > 0 || ladderSuccessSum > 0

  if (successfulPlacements && (medianTries >= 3 || (ladder && success3rdOr4th > success1st))) {
    return {
      sentence: 'The detail is placed successfully but takes several retries.',
      tone: 'friction',
    }
  }

  // 6. High Abandonment / Unplaced
  if (abandonRate >= 0.40 || unrecoveredFalls) {
    return {
      sentence: 'Players repeatedly drop this detail and frequently fail to place it.',
      tone: 'warning',
    }
  }

  // 7. Hint Pressure
  if ((block.hint_pressure_rate ?? 0) >= 0.35) {
    return {
      sentence: 'Players struggle with this detail and frequently request the blueprint hint.',
      tone: 'friction',
    }
  }
  return {
    sentence: 'The detail shows moderate friction with mixed placement attempts.',
    tone: 'friction',
  }
}

// diagnoseDetail produces both sentence and tone from identical decision
// branches, guaranteeing 100% synchronization.
export function diagnoseDetail(block: PuzzleHouseBlock): DetailDiagnosisResult {
  const sampleCount = sampleCountForBlock(block)
  const rateIsReliable = block.rate_is_reliable ?? (sampleCount >= 5)

  // 1. Insufficient Evidence (<5 plays or !rate_is_reliable)
  if (sampleCount < 5 || !rateIsReliable) {
    const sentence = sampleCount === 0 || sampleCount >= 5
      ? 'There is not enough evidence yet.'
      : `Only ${sampleCount} play${sampleCount === 1 ? '' : 's'} so far — not enough evidence to diagnose friction.`
    return { sentence, tone: 'neutral' }
  }

  const ladder = block.retry_ladder
  const ladderSample = ladder?.sample_count ?? 0
  const abandonRate = ladderSample > 0 ? (ladder?.never_succeeded ?? 0) / ladderSample : 0
  const unrecoveredFalls = (block.placements ?? 0) >= 5 && (block.successful_placements ?? 0) === 0 && (block.falls ?? 0) > 0

  // 2. Clean / Smooth First-Try (only if not dominated by abandonment)
  const medianTries = (ladder && ladder.median_tries > 0) ? ladder.median_tries : (block.median_tries_to_success ?? 0)
  if (abandonRate < 0.25 && !unrecoveredFalls && medianTries === 1 && (block.fall_rate ?? 0) < 0.15 && (block.placements ?? 0) >= 5) {
    return {
      sentence: 'The detail is placed cleanly on the first try with minimal friction.',
      tone: 'good',
    }
  }

  return mechanicalDiagnosis(block) ?? progressionDiagnosis(block)
}

// detailDiagnosis produces a plain-language diagnostic sentence derived
// strictly from measured buckets and sample thresholds (Stage 5F).
export function detailDiagnosis(block: PuzzleHouseBlock): string {
  return diagnoseDetail(block).sentence
}

// detailDiagnosisTone maps the diagnosis result to a semantic styling tone.
export function detailDiagnosisTone(block: PuzzleHouseBlock): DiagnosisTone {
  return diagnoseDetail(block).tone
}
