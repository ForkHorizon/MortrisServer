import type { PuzzleDrop, PuzzleHouseBlock } from '../api/houseTypes'
import type { DropCluster } from './dropClustering'
import { DropLayer } from './DropLayer'
import { supportColor } from './houseColors'

// Overlays drawn over the house: where players let go, and which details
// hold a given one up. Split out of HouseCanvas.tsx to keep that file
// under the 300-line gate.

export type Support = { targetBlockID: number; groups: number[][] }

export { DropLayer } from './DropLayer'

export function CanvasOverlays(p: {
  blocks: PuzzleHouseBlock[]
  scale: number
  support?: Support | null
  replayRelease?: {
    x?: number | null
    y?: number | null
    targetX?: number | null
    targetY?: number | null
    targetID?: number | null
  } | null
  drops?: PuzzleDrop[]
  activeDrop?: PuzzleDrop | null
  activeCluster?: DropCluster | null
  onSelectDrop?: (drop: PuzzleDrop | null) => void
  onSelectCluster?: (cluster: DropCluster | null) => void
}) {
  return (
    <>
      {p.support && <SupportLayer blocks={p.blocks} support={p.support} scale={p.scale} />}
      {p.replayRelease && <ReplayArrow blocks={p.blocks} release={p.replayRelease} scale={p.scale} />}
      {p.drops && p.drops.length > 0 && (
        <DropLayer
          drops={p.drops}
          scale={p.scale}
          activeDrop={p.activeDrop}
          activeCluster={p.activeCluster}
          onSelectDrop={p.onSelectDrop}
          onSelectCluster={p.onSelectCluster}
        />
      )}
    </>
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

export function ReplayArrow({
  blocks,
  release,
  scale,
}: {
  blocks: PuzzleHouseBlock[]
  release: {
    x?: number | null
    y?: number | null
    targetX?: number | null
    targetY?: number | null
    targetID?: number | null
  }
  scale: number
}) {
  if (release.x == null || release.y == null) return null
  let tx = release.targetX
  let ty = release.targetY
  if (tx == null || ty == null) {
    if (release.targetID == null || release.targetID < 0) return null
    const target = blocks.find((block) => block.block_id === release.targetID)?.bounds_milli
    if (!target) return null
    tx = (target.min_x + target.max_x) / 2
    ty = (target.min_y + target.max_y) / 2
  }
  return (
    <line
      x1={release.x}
      y1={-release.y}
      x2={tx}
      y2={-ty}
      stroke="#ffffff"
      strokeWidth={scale * 1.2}
      strokeDasharray={`${scale * 4} ${scale * 2}`}
    />
  )
}
