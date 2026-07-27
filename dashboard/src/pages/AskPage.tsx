import { useState } from 'react'
import { apiPost, ApiError } from '../api/client'
import type { NLQueryResult } from '../api/types'
import { useAuth } from '../auth/useAuth'

function AskResult({ result }: { result: NLQueryResult }) {
  const filters = Object.entries(result.interpreted_params)
    .map(([k, v]) => `${k}=${v.join(',')}`)
    .join(', ')
  return (
    <div className="ask-result">
      <p>
        Interpreted as <strong>{result.endpoint}</strong> with {filters || 'no extra filters'}
      </p>
      <pre>{JSON.stringify(result.result, null, 2)}</pre>
    </div>
  )
}

function AskForm({ onAsk, loading }: { onAsk: (question: string) => void; loading: boolean }) {
  const [question, setQuestion] = useState('')
  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    if (question.trim()) onAsk(question)
  }
  return (
    <form onSubmit={submit} className="field">
      <label htmlFor="ask-question">Question</label>
      <input
        id="ask-question"
        type="text"
        value={question}
        onChange={(e) => setQuestion(e.target.value)}
        placeholder="e.g. how many level_end events on iOS in the last 7 days?"
        size={60}
      />
      <button type="submit" disabled={loading || !question.trim()}>
        {loading ? 'Asking…' : 'Ask'}
      </button>
    </form>
  )
}

export function AskPage() {
  const { currentProject } = useAuth()
  const [result, setResult] = useState<NLQueryResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const ask = (question: string) => {
    setLoading(true)
    setError(null)
    setResult(null)
    apiPost<NLQueryResult>('/api/v1/analytics/query', { question, project: currentProject })
      .then((r) => setResult(r))
      .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'Something went wrong.'))
      .finally(() => setLoading(false))
  }

  if (!currentProject) return <p>Select a project to ask about its analytics.</p>

  return (
    <section aria-labelledby="ask-heading">
      <h1 id="ask-heading">Ask</h1>
      <p>
        Ask a question in plain language — it's translated to one of the existing read-only queries
        (overview, event counts, recent events, funnel, retention) and validated exactly like the
        manual filters on those pages. Nothing you type ever reaches SQL directly.
      </p>
      <AskForm onAsk={ask} loading={loading} />
      {error && <p role="alert">{error}</p>}
      {result && <AskResult result={result} />}
    </section>
  )
}
