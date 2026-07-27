import { useSearchParams } from 'react-router-dom'

function isoDaysAgo(days: number): string {
  const d = new Date()
  d.setUTCDate(d.getUTCDate() - days)
  return d.toISOString().slice(0, 10)
}

// Defaults match the API's own default window (section 10.1: 7 days) so
// an empty date range submitted still means the same thing server-side
// and client-side. The URL is the source of truth (plan 2c: saved
// views) — every page using this hook gets a shareable/bookmarkable date
// range for free, no separate state store to keep in sync.
export function useDateRange() {
  const [searchParams, setSearchParams] = useSearchParams()

  const from = searchParams.get('from') || isoDaysAgo(7)
  const to = searchParams.get('to') || isoDaysAgo(0)
  const timezone = searchParams.get('timezone') || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

  const setParam = (key: string, value: string) =>
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set(key, value)
        return next
      },
      { replace: true },
    )

  return {
    from,
    to,
    timezone,
    setFrom: (v: string) => setParam('from', v),
    setTo: (v: string) => setParam('to', v),
    setTimezone: (v: string) => setParam('timezone', v),
    params: {
      from: `${from}T00:00:00Z`,
      to: `${to}T23:59:59Z`,
      timezone,
    },
  }
}
