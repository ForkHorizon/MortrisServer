// The paint scale for house and block views.
//
// Thresholds are fitted to the live playtest distribution measured on
// 2026-08-03 (18.4% of placements ended in a fall overall, 9–22% across
// the houses with real volume), not picked by eye. Re-fit them here as
// volume grows — this is the single place the ramp is defined.
const RAMP: Array<{ upto: number; color: string; label: string }> = [
  { upto: 0.10, color: '#5aa96b', label: 'fine' },
  { upto: 0.25, color: '#e8a33d', label: 'some trouble' },
  { upto: 0.45, color: '#eb7d4b', label: 'hard' },
  { upto: Infinity, color: '#d94f4f', label: 'blocking' },
]

// Anything below the reliability floor is deliberately colourless. A block
// painted red off two placements looks identical to one painted red off
// two hundred, and that false confidence is the exact failure this view
// exists to prevent.
export const UNPLAYED = '#9aa1ad'

export type Paint = { color: string; label: string; reliable: boolean }

export function paintFor(fallRate: number, reliable: boolean, placements: number): Paint {
  if (!reliable) {
    return {
      color: UNPLAYED,
      label: placements === 0 ? 'not played yet' : `only ${placements} ${placements === 1 ? 'try' : 'tries'} so far`,
      reliable: false,
    }
  }
  const band = RAMP.find((entry) => fallRate < entry.upto) ?? RAMP[RAMP.length - 1]
  return { color: band.color, label: band.label, reliable: true }
}

export const RAMP_LEGEND = RAMP.map(({ color, label }) => ({ color, label }))

// Outcome values are enum-shaped and mean nothing to a reader. Each maps
// to a different fix, so the wording names the fix, not the enum.
const OUTCOME_WORDS: Record<string, string> = {
  fell_no_snap_target: 'dropped nowhere near a slot',
  fell_missing_support: 'tried to build in the air',
  fell_missing_rule: 'content bug — no rule exists',
  returned: 'changed their mind',
  placed: 'placed',
}

export function outcomeWords(outcome: string): string {
  return OUTCOME_WORDS[outcome] ?? outcome
}

export const percent = (ratio: number) => `${Math.round(ratio * 100)}%`

// Rules are OR of AND groups. Spelling that out beats showing raw arrays
// to someone who has never read the placement format.
// "after" rather than "once … is/are there" on purpose: a group can hold
// one detail or several, and mixing them in one sentence forces a verb
// agreement that reads wrong either way.
export function ruleSentence(blockID: number, groups: number[][]): string {
  if (groups.length === 0) return `Detail ${blockID} sits on the ground — it can be placed at any time.`
  const alternatives = groups.map((group) =>
    group.length === 1 ? `detail ${group[0]}` : `details ${group.slice(0, -1).join(', ')} and ${group[group.length - 1]}`,
  )
  if (alternatives.length === 1) return `Detail ${blockID} can be placed after ${alternatives[0]}.`
  return `Detail ${blockID} can be placed after ${alternatives.slice(0, -1).join(', after ')}, or after ${alternatives[alternatives.length - 1]}.`
}

// One colour per OR alternative in a placement rule. Deliberately cool
// and distinct from both the fall-rate ramp and the drop-outcome colours:
// these encode structure ("which way you could have built it"), not
// severity, and reusing either scale would imply an ordering between
// alternatives that does not exist.
const SUPPORT_COLORS = ['#5b9bff', '#4fb3a3', '#a583e0', '#d98cc0', '#8fa8c8']

export function supportColor(groupIndex: number): string {
  return SUPPORT_COLORS[groupIndex % SUPPORT_COLORS.length]
}

// Mirrors the editor's "Only Scheme" inspection view, which designers
// already read: G1, G2 … per alternative.
export function supportGroupLabel(groupIndex: number): string {
  return `G${groupIndex + 1}`
}
