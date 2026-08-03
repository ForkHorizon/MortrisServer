import { useState } from 'react'
import type { ArtMode } from '../components/HouseCanvas'

// The house view's four independent toggles, bundled so the page passes
// one object around instead of eight props.
export type HouseControls = ReturnType<typeof useHouseControls>

export function useHouseControls() {
  const [wave, setWave] = useState<number | null>(null)
  const [selected, setSelected] = useState<number | null>(null)
  const [artMode, setArtMode] = useState<ArtMode>('both')
  const [showDrops, setShowDrops] = useState(false)
  const [showSupport, setShowSupport] = useState(false)
  return { wave, setWave, selected, setSelected, artMode, setArtMode, showDrops, setShowDrops, showSupport, setShowSupport }
}
