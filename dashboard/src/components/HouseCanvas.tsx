import type { PuzzleHouseBlock } from '../api/houseTypes'
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

function viewBox(blocks: PuzzleHouseBlock[]): string | null {
  const bounds = blocks.map((b) => b.bounds_milli).filter((b): b is NonNullable<typeof b> => !!b)
  if (bounds.length === 0) return null
  const minX = Math.min(...bounds.map((b) => b.min_x))
  const maxX = Math.max(...bounds.map((b) => b.max_x))
  const minY = Math.min(...bounds.map((b) => b.min_y))
  const maxY = Math.max(...bounds.map((b) => b.max_y))
  const pad = (maxX - minX) * 0.02
  return `${minX - pad} ${-maxY - pad} ${maxX - minX + pad * 2} ${maxY - minY + pad * 2}`
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
}

export function HouseCanvas({ blocks, wave, selected, onSelect, interactive = true, title }: Props) {
  const box = viewBox(blocks)
  if (!box) return <p className="muted">This house has no shapes yet — upload its geometry to draw it.</p>
  const strokeWidth = Number(box.split(' ')[2]) / 220
  return (
    <svg viewBox={box} className="house-canvas" role="img" aria-label={title}>
      {blocks.map((block) => {
        const points = pathFor(block)
        if (!points) return null
        const paint = block.is_ground
          ? { color: UNPLAYED, label: 'ground — always placeable', reliable: false }
          : paintFor(block.fall_rate, block.rate_is_reliable, block.placements)
        const dimmed = wave != null && block.wave_index !== wave
        const isSelected = selected === block.block_id
        return (
          <polygon
            key={block.block_id}
            points={points}
            fill={paint.color}
            fillOpacity={dimmed ? 0.12 : 0.85}
            stroke={isSelected ? '#ffffff' : '#0b0d10'}
            strokeWidth={isSelected ? strokeWidth * 3 : strokeWidth}
            strokeOpacity={dimmed ? 0.2 : 1}
            tabIndex={interactive && !dimmed ? 0 : undefined}
            role={interactive && !dimmed ? 'button' : undefined}
            aria-label={interactive ? `Detail ${block.block_id}, ${paint.label}` : undefined}
            style={interactive && !dimmed ? { cursor: 'pointer' } : undefined}
            onClick={interactive && !dimmed ? () => onSelect?.(block.block_id) : undefined}
            onKeyDown={
              interactive && !dimmed
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
      })}
    </svg>
  )
}
