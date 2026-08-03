import type { PuzzleDrop, PuzzleHouseBlock } from '../api/houseTypes'
import { supportColor } from './houseColors'

// Overlays drawn over the house: where players let go, and which details
// hold a given one up. Split out of HouseCanvas.tsx to keep that file
// under the 300-line gate.

export type Support = { targetBlockID: number; groups: number[][] }

// Outcome colours for drop dots. Deliberately not the fall-rate ramp:
// these encode *what happened*, not *how bad it is*, and reusing the ramp
// would suggest an ordering between reasons that does not exist.
const DROP_COLOR: Record<string, string> = {
  placed: '#5aa96b',
  fell_no_snap_target: '#e8a33d',
  fell_missing_support: '#d94f4f',
  fell_missing_rule: '#b06fd0',
  returned: '#7a8faf',
}

type DropLayerProps = { drops: PuzzleDrop[]; scale: number }

// Each dot is where a player let go; the line runs to the slot they were
// aiming at. The line is what turns a cloud of dots into a direction —
// "everyone releases below the slot" is invisible without it.
export function DropLayer({ drops, scale }: DropLayerProps) {
  return (
    <g aria-hidden="true">
      {drops.map((drop, i) => {
        const color = DROP_COLOR[drop.outcome] ?? '#9aa1ad'
        const hasTarget = drop.target_id >= 0
        return (
          <g key={`${drop.attempt_id}-${drop.block_id}-${i}`}>
            {hasTarget && (
              <line
                x1={drop.release_x_milli}
                y1={-drop.release_y_milli}
                x2={drop.target_x_milli}
                y2={-drop.target_y_milli}
                stroke={color}
                strokeWidth={scale * 0.5}
                strokeOpacity={0.55}
              />
            )}
            <circle
              cx={drop.release_x_milli}
              cy={-drop.release_y_milli}
              r={scale * 2}
              fill={color}
              fillOpacity={0.9}
              stroke="#0b0d10"
              strokeWidth={scale * 0.3}
            />
          </g>
        )
      })}
    </g>
  )
}

// A block can appear in more than one alternative. First group wins for
// its outline, and the connecting lines still show every membership, so
// nothing is hidden by the tie-break.
export function supportColorByBlock(support: Support | null | undefined): Map<number, string> {
  const colors = new Map<number, string>()
  support?.groups.forEach((group, index) => {
    for (const id of group) {
      if (!colors.has(id)) colors.set(id, supportColor(index))
    }
  })
  return colors
}

function centreOf(block: PuzzleHouseBlock): [number, number] | null {
  const b = block.bounds_milli
  if (!b) return null
  return [(b.min_x + b.max_x) / 2, -(b.min_y + b.max_y) / 2]
}

// Lines run from each required detail to the one being placed, coloured
// by alternative. The dot sits at the required end, so the direction
// reads as "this holds that up" without needing arrowheads.
export function SupportLayer({ blocks, support, scale }: { blocks: PuzzleHouseBlock[]; support: Support; scale: number }) {
  const byID = new Map(blocks.map((b) => [b.block_id, b]))
  const target = byID.get(support.targetBlockID)
  const to = target ? centreOf(target) : null
  if (!to) return null
  return (
    <g aria-hidden="true">
      {support.groups.flatMap((group, index) =>
        group.map((id) => {
          const from = byID.get(id) ? centreOf(byID.get(id)!) : null
          if (!from) return null
          const color = supportColor(index)
          return (
            <g key={`${index}-${id}`}>
              <line x1={from[0]} y1={from[1]} x2={to[0]} y2={to[1]} stroke={color} strokeWidth={scale * 1.4} strokeOpacity={0.85} />
              <circle cx={from[0]} cy={from[1]} r={scale * 2.4} fill={color} stroke="#0b0d10" strokeWidth={scale * 0.4} />
            </g>
          )
        }),
      )}
    </g>
  )
}
