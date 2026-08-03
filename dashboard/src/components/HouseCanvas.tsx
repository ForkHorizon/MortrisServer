import { useState } from 'react'
import type { PuzzleDrop, PuzzleHouseBlock } from '../api/houseTypes'
import { UNPLAYED, paintFor } from './houseColors'
import { type HouseMetric, metricReading } from './houseMetrics'
import { DropLayer, SupportLayer, supportColorByBlock } from './houseOverlays'

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

function canvasViewport(extent: Extent, zoom: number) {
  const width = extent.maxX - extent.minX
  const height = extent.maxY - extent.minY
  const padX = width * 0.02 / zoom
  const padY = height * 0.02 / zoom
  const visibleWidth = width / zoom
  const visibleHeight = height / zoom
  return {
    width,
    height,
    box: `${extent.minX + (width - visibleWidth) / 2 - padX} ${-extent.maxY + (height - visibleHeight) / 2 - padY} ${visibleWidth + padX * 2} ${visibleHeight + padY * 2}`,
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
  /** Replay mode: only these blocks are standing at this step. */
  placed?: Set<number> | null
  /** The detail the player is moving at this step. */
  active?: number | null
  /** Blocks the active detail was waiting on. */
  missing?: Set<number>
  /** Placement rule of the selected detail, drawn as a dependency graph. */
  support?: { targetBlockID: number; groups: number[][] } | null
  metric?: HouseMetric
  replayRelease?: { x: number; y: number; targetID: number } | null
}

// The art is cropped to its opaque pixels, whose extent is exactly the
// union of block bounds — so it is placed at that rect with no stored
// offset. preserveAspectRatio="none" is correct here precisely because
// the two rects are the same rect; letterboxing would misalign it.
export function HouseCanvas({ blocks, wave, selected, onSelect, interactive = true, title, artUrl, mode = 'both', drops, placed, active, missing, support, metric = 'fall', replayRelease }: Props) {
  const [zoom, setZoom] = useState(1)
  const extent = extentOf(blocks)
  if (!extent) return <p className="muted">This house has no shapes yet — upload its geometry to draw it.</p>
  const { width, height, box } = canvasViewport(extent, zoom)
  const strokeWidth = width / 220
  const supportColors = supportColorByBlock(support)
  const showArt = artUrl && mode !== 'diagram'
  const showDiagram = mode !== 'art'
  return (
    <div className="house-canvas-frame">
      <div className="canvas-zoom" role="group" aria-label="House zoom">
        <button type="button" onClick={() => setZoom((value) => Math.min(3, value * 1.4))}>Zoom in</button>
        <button type="button" onClick={() => setZoom((value) => Math.max(1, value / 1.4))} disabled={zoom === 1}>Zoom out</button>
        <button type="button" onClick={() => setZoom(1)} disabled={zoom === 1}>Reset view</button>
      </div>
      <svg viewBox={box} className="house-canvas" role="img" aria-label={title} style={{ aspectRatio: `${width}/${height}` }}>
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
            metric={metric}
            replay={placed ? { standing: placed.has(block.block_id), active: active === block.block_id, missing: !!missing?.has(block.block_id) } : undefined}
            supportColor={supportColors.get(block.block_id)}
          />
        ))}
      {support && <SupportLayer blocks={blocks} support={support} scale={strokeWidth} />}
      {replayRelease && <ReplayArrow blocks={blocks} release={replayRelease} scale={strokeWidth} />}
      {drops && drops.length > 0 && <DropLayer drops={drops} scale={strokeWidth} />}
      </svg>
    </div>
  )
}

function ReplayArrow({ blocks, release, scale }: { blocks: PuzzleHouseBlock[]; release: { x: number; y: number; targetID: number }; scale: number }) {
  const target = blocks.find((block) => block.block_id === release.targetID)?.bounds_milli
  if (!target || release.x < 0 || release.y < 0) return null
  const x = (target.min_x + target.max_x) / 2
  const y = (target.min_y + target.max_y) / 2
  return <line x1={release.x} y1={-release.y} x2={x} y2={-y} stroke="#ffffff" strokeWidth={scale * 1.2} strokeDasharray={`${scale * 4} ${scale * 2}`} />
}

type ReplayState = { standing: boolean; active: boolean; missing: boolean }

type ShapeProps = {
  block: PuzzleHouseBlock
  dimmed: boolean
  isSelected: boolean
  overArt: boolean
  strokeWidth: number
  interactive: boolean
  onSelect?: (blockID: number) => void
  replay?: ReplayState
  supportColor?: string
  metric: HouseMetric
}

// In replay the colour stops meaning "how often this falls" and starts
// meaning "what is this block doing right now" — standing, in the
// player's hand, or the thing they were waiting on. Mixing the two
// scales in one picture would make neither readable.
const REPLAY_STANDING = '#5f6b7d'
const REPLAY_ACTIVE = '#e8a33d'
const REPLAY_MISSING = '#d94f4f'

function replayPaint(replay: ReplayState): { color: string; opacity: number; label: string } {
  if (replay.active) return { color: REPLAY_ACTIVE, opacity: 0.95, label: 'being placed now' }
  if (replay.missing) return { color: REPLAY_MISSING, opacity: 0.9, label: 'missing support' }
  if (replay.standing) return { color: REPLAY_STANDING, opacity: 0.8, label: 'already placed' }
  return { color: REPLAY_STANDING, opacity: 0.07, label: 'not placed yet' }
}

function BlockShape({ block, dimmed, isSelected, overArt, strokeWidth, interactive, onSelect, replay, supportColor: supportStroke, metric }: ShapeProps) {
  const points = pathFor(block)
  if (!points) return null
  if (replay) {
    const rp = replayPaint(replay)
    return (
      <polygon
        points={points}
        fill={rp.color}
        fillOpacity={rp.opacity}
        stroke={replay.active || replay.missing ? '#ffffff' : '#0b0d10'}
        strokeWidth={replay.active || replay.missing ? strokeWidth * 2.5 : strokeWidth}
        aria-label={`Detail ${block.block_id}, ${rp.label}`}
      />
    )
  }
  // A ground block is placeable at any time, so a fall rate would be
  // meaningless for it — it is always neutral, never scored.
  const reading = metricReading(block, metric)
  const paint = block.is_ground
    ? { color: UNPLAYED, label: 'ground — always placeable', reliable: false }
    : paintFor(reading.ratio, reading.reliable, reading.sample)
  const clickable = interactive && !dimmed
  return (
    <polygon
      points={points}
      fill={paint.color}
      fillOpacity={fillOpacity(paint.reliable, dimmed, overArt)}
      stroke={isSelected ? '#ffffff' : supportStroke ?? '#0b0d10'}
      strokeWidth={isSelected || supportStroke ? strokeWidth * 2.5 : strokeWidth}
      strokeOpacity={dimmed ? 0.2 : 1}
      tabIndex={clickable ? 0 : undefined}
      role={clickable ? 'button' : undefined}
      aria-label={interactive ? `Detail ${block.block_id}, ${reading.label}; ${reading.sample} samples; ${paint.label}` : undefined}
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
