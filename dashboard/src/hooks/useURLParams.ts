import { useSearchParams } from 'react-router-dom'

// Shared read/write helper for URL-backed filter state (plan 2c: saved
// views) — every filterable page uses this so filters stay
// shareable/bookmarkable with no separate state store to keep in sync.
export function useURLParams() {
  const [searchParams, setSearchParams] = useSearchParams()

  const get = (key: string) => searchParams.get(key) ?? ''

  // Takes a patch of several keys at once — setSearchParams isn't a React
  // state setter, so calling it more than once per handler (e.g. changing
  // name while clearing a dependent key) has each call read the same stale
  // snapshot and the earlier calls get lost.
  const setMany = (patch: Record<string, string>) =>
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        for (const [key, value] of Object.entries(patch)) {
          if (value) next.set(key, value)
          else next.delete(key)
        }
        return next
      },
      { replace: true },
    )

  const set = (key: string, value: string) => setMany({ [key]: value })

  return { get, set, setMany }
}
