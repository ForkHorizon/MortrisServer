import { useState } from 'react'
import { apiGet, ApiError } from '../api/client'
import type { DigestResult } from '../api/types'

// Manual trigger, not auto-fetched on page load: this calls Claude
// (Phase 5 #3) and costs real money/latency per click, unlike the other
// Overview data which is plain SQL.
export function DigestPanel({ projectID }: { projectID: string }) {
  const [digest, setDigest] = useState<DigestResult | null>(null)
  const [generatedAt, setGeneratedAt] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const generate = () => {
    setLoading(true)
    setError(null)
    apiGet<DigestResult>('/api/v1/analytics/digest', { project: projectID })
      .then((d) => {
        setDigest(d)
        setGeneratedAt(Date.now())
      })
      .catch((e: unknown) => setError(e instanceof ApiError ? e.message : 'Something went wrong.'))
      .finally(() => setLoading(false))
  }

  return (
    <div className="digest-panel">
      <button type="button" onClick={generate} disabled={loading}>
        {loading ? 'Generating…' : "Generate today's digest"}
      </button>
      {error && <p role="alert">{error}</p>}
      {digest && (
        <>
          {/* Snapshot, not live data — stamped so a digest read minutes
              later isn't mistaken for a re-generated, current one. */}
          <p className="hint">Generated {new Date(generatedAt!).toLocaleTimeString()}</p>
          <p className="digest-narration">{digest.narration}</p>
        </>
      )}
    </div>
  )
}
