// Sits under a page's controls, above its data: says what's on screen and
// how current it is, instead of leaving stale rows to look authoritative.
export function Freshness({
  loading,
  error,
  stale,
  updatedAt,
  loadingLabel = 'Loading…',
}: {
  loading: boolean
  error?: string | null
  stale?: boolean
  updatedAt: number | null
  loadingLabel?: string
}) {
  return (
    <>
      {loading && updatedAt === null && <p role="status">{loadingLabel}</p>}
      {updatedAt !== null && (
        <p className="hint" role="status">
          Updated {new Date(updatedAt).toLocaleTimeString()}
          {loading && ' · refreshing'}
          {stale && ' · showing last successful result'}
        </p>
      )}
      {error && <p role="alert">{error}</p>}
    </>
  )
}
