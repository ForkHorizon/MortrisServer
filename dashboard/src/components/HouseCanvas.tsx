import type { PuzzleDrop, PuzzleHouseBlock } from '../api/houseTypes'
import { UNPLAYED, paintFor } from './houseColors'

// Milli-unit world space is y-up; SVG is y-down. Negating y at render time
// keeps every stored coordinate in the one space the exporter, the
// database and the game already agree on.
function pathFor(block: PuzzleHouseBlock): string | null {
  if (block.outline_milli?.length) {
    return block.outline_milli.map(([x, y]) => `${x},${-y}`).join(' ')
  }
  const b = block.bounds_milli
  if (!b) return null
  return `${b.min_x},${-b.min_y} ${b.max_x},${-b.min_y} ${b.max_x},${-b.max_y} ${b.min_x},${-b.max_y}`
}

type Extent = { minX: number; minY: number; maxX: number; maxY: number }

function extentOf(blocks: PuzzleHouseBlock[]): Extent | null {
  const bounds = blocks.map((b) => b.bounds_milli).filter((b): b is NonNullable<typeof b> => !!b)
  if (bounds.length === 0) return null
  return {
    minX: Math.min(...bounds.map((b) => b.min_x)),
    maxX: Math.max(...bounds.map((b) => b.max_x)),
    minY: Math.min(...bounds.map((b) => b.min_y)),
    maxY: Math.max(...bounds.map((b) => b.max_y)),
  }
}

export type ArtMode = 'both' | 'art' | 'diagram'

// Over the art, only blocks with a trustworthy rate get filled. A flat
// tint on everything washes the picture out and buries the one red block
// among fifty grey ones, which is the opposite of what the overlay is
// for. Blocks without enough plays keep their outline and let the art
// through, so the eye lands on the blocks that actually mean something.
// On the bare diagram there is no picture to protect, so everything is
// filled and grey reads as "not enough plays" on its own.
function fillOpacity(reliable: boolean, dimmed: boolean, overArt: boolean): number {
  if (dimmed) return overArt ? 0 : 0.1
  if (!overArt) return 0.85
  return reliable ? 0.62 : 0
}

type Props = {
  blocks: PuzzleHouseBlock[]
  /** Blocks outside this wave are ghosted, not hidden — the shape of the
   *  finished house is the context that makes one wave readable. */
  wave?: number | null
  selected?: number | null
  onSelect?: (blockID: number) => void
  /** Thumbnails are decorative and must not be keyboard stops. */
  interactive?: boolean
  title: string
  /** URL of the assembled house picture, drawn under the diagram. */
  artUrl?: string
  mode?: ArtMode
  /** Release points to plot over the house. */
  drops?: PuzzleDrop[]
}

// The art is cropped to its opaque pixels, whose extent is exactly the
// union of block bounds — so it is placed at that rect with no stored
// offset. preserveAspectRatio="none" is correct here precisely because
// the two rects are the same rect; letterboxing would misalign it.
export function HouseCanvas({ blocks, wave, selected, onSelect, interactive = true, title, artUrl, mode = 'both', drops }: Props) {
  const extent = extentOf(blocks)
  if (!extent) return <p className="muted">This house has no shapes yet — upload its geometry to draw it.</p>
  const width = extent.maxX - extent.minX
  const height = extent.maxY - extent.minY
  const pad = width * 0.02
  const box = `${extent.minX - pad} ${-extent.maxY - pad} ${width + pad * 2} ${height + pad * 2}`
  const strokeWidth = width / 220
  const showArt = artUrl && mode !== 'diagram'
  const showDiagram = mode !== 'art'
  return (
    <svg viewBox={box} className="house-canvas" role="img" aria-label={title}>
      {showArt && (
        <image
          href={artUrl}
          x={extent.minX}
          y={-extent.maxY}
          width={width}
          height={height}
          preserveAspectRatio="none"
        />
      )}
      {showDiagram &&
        blocks.map((block) => (
          <BlockShape
            key={block.block_id}
            block={block}
            dimmed={wave != null && block.wave_index !== wave}
            isSelected={selected === block.block_id}
            overArt={!!showArt}
            strokeWidth={strokeWidth}
            interactive={interactive}
            onSelect={onSelect}
          />
        ))}
      {drops && drops.length > 0 && <DropLayer drops={drops} scale={strokeWidth} />}
    </svg>
  )
}

type ShapeProps = {
  block: PuzzleHouseBlock
  dimmed: boolean
  isSelected: boolean
  overArt: boolean
  strokeWidth: number
  interactive: boolean
  onSelect?: (blockID: number) => void
}

function BlockShape({ block, dimmed, isSelected, overArt, strokeWidth, interactive, onSelect }: ShapeProps) {
  const points = pathFor(block)
  if (!points) return null
  // A ground block is placeable at any time, so a fall rate would be
  // meaningless for it — it is always neutral, never scored.
  const paint = block.is_ground
    ? { color: UNPLAYED, label: 'ground — always placeable', reliable: false }
    : paintFor(block.fall_rate, block.rate_is_reliable, block.placements)
  const clickable = interactive && !dimmed
  return (
    <polygon
      points={points}
      fill={paint.color}
      fillOpacity={fillOpacity(paint.reliable, dimmed, overArt)}
      stroke={isSelected ? '#ffffff' : '#0b0d10'}
      strokeWidth={isSelected ? strokeWidth * 3 : strokeWidth}
      strokeOpacity={dimmed ? 0.2 : 1}
      tabIndex={clickable ? 0 : undefined}
      role={clickable ? 'button' : undefined}
      aria-label={interactive ? `Detail ${block.block_id}, ${paint.label}` : undefined}
      style={clickable ? { cursor: 'pointer' } : undefined}
      onClick={clickable ? () => onSelect?.(block.block_id) : undefined}
      onKeyDown={
        clickable
          ? (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                onSelect?.(block.block_id)
              }
            }
          : undefined
      }
    />
  )
}

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
