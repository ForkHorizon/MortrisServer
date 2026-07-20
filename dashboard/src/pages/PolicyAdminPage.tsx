import { useCallback, useState } from 'react'
import { apiGet, apiPost, apiDelete, ApiError } from '../api/client'
import type { PolicyRule } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'
import { DataTable } from '../components/DataTable'

const MODES: PolicyRule['mode'][] = ['active', 'pause_upload', 'disable_collection']

export function PolicyAdminPage() {
  const { currentProject, session } = useAuth()
  const [refreshKey, setRefreshKey] = useState(0)
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const [mode, setMode] = useState<PolicyRule['mode']>('pause_upload')
  const [nextCheckSeconds, setNextCheckSeconds] = useState(3600)
  const [discardPending, setDiscardPending] = useState(false)
  const [reason, setReason] = useState('')
  const [environment, setEnvironment] = useState('')
  const [appVersion, setAppVersion] = useState('')
  const [buildNumber, setBuildNumber] = useState('')
  const [sdkVersion, setSdkVersion] = useState('')
  const fetchPolicyRules = useCallback(
    () => {
      void refreshKey
      return apiGet<{ rules: PolicyRule[] }>('/api/v1/policy', { project: currentProject })
    },
    [currentProject, refreshKey],
  )

  const { data, error, loading } = useApiData<{ rules: PolicyRule[] }>(fetchPolicyRules)

  if (!currentProject) return <p>Select a project to administer its kill switch.</p>

  const isAdmin = session?.role === 'owner' || session?.projects.find((project) => project.id === currentProject)?.role === 'project_admin'

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setFormError(null)
    if (!reason.trim()) {
      setFormError('A reason is required — every kill-switch action is audited.')
      return
    }
    setSubmitting(true)
    try {
      await apiPost('/api/v1/policy', {
        project_id: currentProject,
        environment: environment || undefined,
        app_version: appVersion || undefined,
        build_number: buildNumber || undefined,
        sdk_version: sdkVersion || undefined,
        mode,
        next_check_seconds: nextCheckSeconds,
        discard_pending: discardPending,
        reason,
      })
      setReason('')
      setRefreshKey((k) => k + 1)
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Failed to create rule.')
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(id: number) {
    if (!confirm('Remove this policy rule? Affected installations will revert to the next-most-specific rule (or active).')) {
      return
    }
    try {
      await apiDelete(`/api/v1/policy/${id}?project=${encodeURIComponent(currentProject)}`)
      setRefreshKey((k) => k + 1)
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Failed to delete rule.')
    }
  }

  return (
    <section aria-labelledby="policy-heading">
      <h1 id="policy-heading">Policy administration (kill switch)</h1>
      <p>
        Rules target project, environment, app version, build number, or SDK version — the most specific enabled
        rule wins. Leave a field blank to match any value.
      </p>

      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      {data && (
        <DataTable
          caption="Active policy rules"
          columns={[
            { key: 'mode', label: 'Mode' },
            { key: 'environment', label: 'Environment', render: (r) => r.environment || 'any' },
            { key: 'app_version', label: 'App version', render: (r) => r.app_version || 'any' },
            { key: 'build_number', label: 'Build', render: (r) => r.build_number || 'any' },
            { key: 'sdk_version', label: 'SDK version', render: (r) => r.sdk_version || 'any' },
            { key: 'reason', label: 'Reason' },
            {
              key: 'actions',
              label: 'Actions',
              render: (r) =>
                isAdmin ? (
                  <button type="button" onClick={() => void handleDelete(r.id)}>
                    Remove
                  </button>
                ) : null,
            },
          ]}
          rows={data.rules}
          getRowKey={(r) => r.id}
        />
      )}

      {isAdmin ? (
        <form onSubmit={handleCreate} aria-labelledby="policy-form-heading">
          <h2 id="policy-form-heading">Create a rule</h2>
          <div className="field">
            <label htmlFor="policy-mode">Mode</label>
            <select id="policy-mode" value={mode} onChange={(e) => setMode(e.target.value as PolicyRule['mode'])}>
              {MODES.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label htmlFor="policy-next-check">Next check (seconds)</label>
            <input
              id="policy-next-check"
              type="number"
              min={1}
              value={nextCheckSeconds}
              onChange={(e) => setNextCheckSeconds(Number(e.target.value))}
            />
          </div>
          <div className="field checkbox-field">
            <input
              id="policy-discard"
              type="checkbox"
              checked={discardPending}
              onChange={(e) => setDiscardPending(e.target.checked)}
            />
            <label htmlFor="policy-discard">Discard pending queued events on affected clients</label>
          </div>
          <div className="field">
            <label htmlFor="policy-environment">Environment (optional)</label>
            <input id="policy-environment" value={environment} onChange={(e) => setEnvironment(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="policy-app-version">App version (optional)</label>
            <input id="policy-app-version" value={appVersion} onChange={(e) => setAppVersion(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="policy-build">Build number (optional)</label>
            <input id="policy-build" value={buildNumber} onChange={(e) => setBuildNumber(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="policy-sdk-version">SDK version (optional)</label>
            <input id="policy-sdk-version" value={sdkVersion} onChange={(e) => setSdkVersion(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="policy-reason">Reason (required, audited)</label>
            <input id="policy-reason" value={reason} onChange={(e) => setReason(e.target.value)} required />
          </div>
          {formError && (
            <p role="alert" className="error-text">
              {formError}
            </p>
          )}
          <button type="submit" disabled={submitting}>
            {submitting ? 'Creating…' : 'Create rule'}
          </button>
        </form>
      ) : (
        <p>Viewing only — creating or removing rules requires project-admin access.</p>
      )}
    </section>
  )
}
