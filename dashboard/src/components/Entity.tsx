import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useURLParams } from '../hooks/useURLParams'

export type EntityType = 'event' | 'install' | 'session' | 'app_version' | 'build' | 'platform'

// Canonical pivot target per entity type — the one page that already
// filters by this value. `null` means no page supports filtering by it
// yet, so the entity renders copy-only instead of a fake link.
const ROUTE: Record<EntityType, { path: string; param: string; carriesRange: boolean } | null> = {
  event: { path: '/events', param: 'name', carriesRange: true },
  install: { path: '/installations', param: 'id', carriesRange: false },
  session: null,
  app_version: { path: '/events', param: 'app_version', carriesRange: true },
  build: { path: '/events', param: 'build_number', carriesRange: true },
  platform: { path: '/events', param: 'platform', carriesRange: true },
}

export function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      type="button"
      className="entity-copy"
      title="Copy to clipboard"
      aria-label={`Copy ${value}`}
      onClick={() => {
        navigator.clipboard.writeText(value)
        setCopied(true)
        setTimeout(() => setCopied(false), 1000)
      }}
    >
      {copied ? '✓' : '⧉'}
    </button>
  )
}

// The dashboard's ~7 recurring entity types (event name, install ID,
// session ID, app version, build, platform, property) each get this one
// component instead of a bespoke render per page, so a value + pivot
// link + copy button is a property of the entity, not of the page.
export function Entity({ type, value }: { type: EntityType; value: string }) {
  const { get } = useURLParams()
  if (!value) return null
  const route = ROUTE[type]

  let to: string | null = null
  if (route) {
    const params = new URLSearchParams({ [route.param]: value })
    if (route.carriesRange) {
      for (const key of ['from', 'to', 'timezone']) {
        const v = get(key)
        if (v) params.set(key, v)
      }
    }
    to = `${route.path}?${params.toString()}`
  }

  return (
    <span className="entity">
      {to ? <Link to={to}>{value}</Link> : value}
      <CopyButton value={value} />
    </span>
  )
}
