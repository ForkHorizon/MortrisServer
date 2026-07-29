import { useURLParams } from './useURLParams'

function isoDaysAgo(days: number): string {
  const d = new Date()
  d.setUTCDate(d.getUTCDate() - days)
  return d.toISOString().slice(0, 10)
}

function addDays(dateStr: string, days: number): string {
  const d = new Date(`${dateStr}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  return d.toISOString().slice(0, 10)
}

// Returns the UTC instant for 00:00:00 local time of `dateStr` (YYYY-MM-DD)
// in IANA zone `tz` — matches the backend's `(effective_at AT TIME ZONE
// $tz)::date` bucketing, instead of a UTC midnight that only lines up for
// UTC-ish zones (the mismatch was cutting the first/last chart column short).
// ponytail: single-pass offset lookup, can be a DST-delta off right at a
// transition instant — fine for day-bucketed charts, revisit with a real tz
// library if that precision ever matters.
function zonedMidnightToUtcIso(dateStr: string, tz: string): string {
  const guess = new Date(`${dateStr}T00:00:00Z`)
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: tz,
    hourCycle: 'h23',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
    .formatToParts(guess)
    .reduce((acc: Record<string, string>, p) => {
      acc[p.type] = p.value
      return acc
    }, {})
  const asUtc = Date.UTC(
    Number(parts.year),
    Number(parts.month) - 1,
    Number(parts.day),
    Number(parts.hour),
    Number(parts.minute),
    Number(parts.second),
  )
  const offsetMs = asUtc - guess.getTime()
  return new Date(guess.getTime() - offsetMs).toISOString()
}

// Defaults match the API's own default window (section 10.1: 7 days) so
// an empty date range submitted still means the same thing server-side
// and client-side.
export function useDateRange() {
  const { get, set } = useURLParams()

  const from = get('from') || isoDaysAgo(7)
  const to = get('to') || isoDaysAgo(0)
  const timezone = get('timezone') || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

  return {
    from,
    to,
    timezone,
    setFrom: (v: string) => set('from', v),
    setTo: (v: string) => set('to', v),
    setTimezone: (v: string) => set('timezone', v),
    params: {
      // Local-midnight boundaries in the selected timezone, exclusive on
      // the `to` end — the same [from, to) window the backend buckets
      // days into, so the first and last chart columns are full days that
      // match their axis labels.
      from: zonedMidnightToUtcIso(from, timezone),
      to: zonedMidnightToUtcIso(addDays(to, 1), timezone),
      timezone,
    },
  }
}
