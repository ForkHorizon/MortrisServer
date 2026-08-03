// House view types (docs/puzzle-visual-analytics-plan.md), kept out of
// types.ts so that file stays under the 300-line gate.
export interface PuzzleBlockBounds {
  min_x: number
  min_y: number
  max_x: number
  max_y: number
}

export interface PuzzleHouseBlock {
  block_id: number
  wave_index: number
  order_in_layer: number
  is_ground: boolean
  visual_key: string
  bounds_milli?: PuzzleBlockBounds
  outline_milli?: Array<[number, number]>
  required_groups: number[][]
  placements: number
  falls: number
  fall_rate: number
  falls_by_reason: Record<string, number>
  rate_is_reliable: boolean
}

export interface PuzzleHouseDetail {
  city_id: number
  house_id: number
  display_label: string
  content_revision: string
  wave_count: number
  blocks: PuzzleHouseBlock[]
}

export interface PuzzleHouseSummary {
  city_id: number
  house_id: number
  display_label: string
  attempts: number
  placements: number
  falls: number
  hints: number
  fall_rate: number
  rate_is_reliable: boolean
  has_geometry: boolean
}

export interface PuzzleHouseList {
  content_revision: string
  houses: PuzzleHouseSummary[]
}
