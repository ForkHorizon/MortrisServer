import { useURLParams } from './useURLParams'

function isoDaysAgo(days: number): string {
  const d = new Date()
  d.setUTCDate(d.getUTCDate() - days)
  return d.toISOString().slice(0, 10)
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
      from: `${from}T00:00:00Z`,
      to: `${to}T23:59:59Z`,
      timezone,
    },
  }
}
