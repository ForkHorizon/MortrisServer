import { useEffect, useState } from 'react'

const relativeTime = new Intl.RelativeTimeFormat('en', { numeric: 'auto' })

function relativeLabel(epochMs: number): string {
  const diffSec = Math.round((epochMs - Date.now()) / 1000)
  const abs = Math.abs(diffSec)
  if (abs < 5) return 'just now'
  if (abs < 60) return relativeTime.format(diffSec, 'second')
  const diffMin = Math.round(diffSec / 60)
  if (Math.abs(diffMin) < 60) return relativeTime.format(diffMin, 'minute')
  const diffHour = Math.round(diffMin / 60)
  if (Math.abs(diffHour) < 24) return relativeTime.format(diffHour, 'hour')
  return relativeTime.format(Math.round(diffHour / 24), 'day')
}

function compactAbsolute(date: Date, timeZone?: string): string {
  return new Intl.DateTimeFormat('en-US', { timeZone, month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }).format(date)
}

function fullAbsolute(date: Date, timeZone?: string): string {
  return new Intl.DateTimeFormat('en-US', { timeZone, dateStyle: 'medium', timeStyle: 'medium' }).format(date)
}

// Shared tick so N relative timestamps on one page don't run N intervals.
// ponytail: still a fresh setInterval per mounted Timestamp — fine at
// today's row counts, switch to one shared clock context if a page ever
// renders hundreds of relative timestamps at once.
function useTick(enabled: boolean) {
  const [, setTick] = useState(0)
  useEffect(() => {
    if (!enabled) return
    const id = setInterval(() => setTick((t) => t + 1), 1000)
    return () => clearInterval(id)
  }, [enabled])
}

// Feed rows want relative time ("12s ago") that keeps ticking even
// between polls; analysis tables want a compact absolute in the
// selected timezone. Both keep the full absolute on hover via title.
export function Timestamp({ value, mode = 'absolute', timeZone }: { value: string; mode?: 'relative' | 'absolute'; timeZone?: string }) {
  useTick(mode === 'relative')
  const date = new Date(value)
  const title = fullAbsolute(date, timeZone)
  return (
    <time dateTime={value} title={title}>
      {mode === 'relative' ? relativeLabel(date.getTime()) : compactAbsolute(date, timeZone)}
    </time>
  )
}
