import { useCallback, useState } from 'react'
import { ApiError, apiDelete, apiGet, apiPost } from '../api/client'
import type { PolicyRule } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { DataTable } from '../components/DataTable'
import { useApiData } from '../hooks/useApiData'

const MODES: PolicyRule['mode'][] = ['active', 'pause_upload', 'disable_collection']

export function PolicyAdminPage() {
  const { currentProject, session } = useAuth()
  const { rules, error, loading, refresh } = usePolicyRules(currentProject)
  const isAdmin = session?.role === 'owner' || session?.projects.find((project) => project.id === currentProject)?.role === 'project_admin'
  if (!currentProject) return <p>Select a project to administer its kill switch.</p>
  return (
    <section aria-labelledby="policy-heading">
      <h1 id="policy-heading">Policy administration (kill switch)</h1>
      <p>Rules target project, environment, app version, build number, or SDK version — the most specific enabled rule wins. Leave a field blank to match any value.</p>
      {loading && <p role="status">Loading…</p>}
      {error && <p role="alert">{error}</p>}
      <PolicyRuleTable rules={rules} canEdit={isAdmin} projectID={currentProject} refresh={refresh} />
      {isAdmin ? <PolicyRuleForm projectID={currentProject} refresh={refresh} /> : <p>Viewing only — creating or removing rules requires project-admin access.</p>}
    </section>
  )
}

function usePolicyRules(currentProject: string) {
  const [refreshKey, setRefreshKey] = useState(0)
  const fetchPolicyRules = useCallback(() => {
    void refreshKey
    return apiGet<{ rules: PolicyRule[] }>('/api/v1/policy', { project: currentProject })
  }, [currentProject, refreshKey])
  const { data, error, loading } = useApiData(fetchPolicyRules)
  return { rules: data?.rules ?? [], error, loading, refresh: setRefreshKey }
}

function PolicyRuleTable({ rules, canEdit, projectID, refresh }: { rules: PolicyRule[]; canEdit: boolean; projectID: string; refresh: React.Dispatch<React.SetStateAction<number>> }) {
  async function removeRule(id: number) {
    if (!confirm('Remove this policy rule? Affected installations will revert to the next-most-specific rule (or active).')) return
    await apiDelete(`/api/v1/policy/${id}?project=${encodeURIComponent(projectID)}`)
    refresh((key) => key + 1)
  }
  return <DataTable caption="Active policy rules" columns={[{ key: 'mode', label: 'Mode' }, { key: 'environment', label: 'Environment', render: (r) => r.environment || 'any' }, { key: 'app_version', label: 'App version', render: (r) => r.app_version || 'any' }, { key: 'build_number', label: 'Build', render: (r) => r.build_number || 'any' }, { key: 'sdk_version', label: 'SDK version', render: (r) => r.sdk_version || 'any' }, { key: 'reason', label: 'Reason' }, { key: 'actions', label: 'Actions', render: (r) => canEdit ? <button type="button" onClick={() => void removeRule(r.id)}>Remove</button> : null }]} rows={rules} getRowKey={(r) => r.id} />
}

function PolicyRuleForm({ projectID, refresh }: { projectID: string; refresh: React.Dispatch<React.SetStateAction<number>> }) {
  const [form, setForm] = useState({ mode: 'pause_upload' as PolicyRule['mode'], nextCheckSeconds: 3600, discardPending: false, reason: '', environment: '', appVersion: '', buildNumber: '', sdkVersion: '' })
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const set = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) => setForm((current) => ({ ...current, [key]: value }))
  async function createRule(e: React.FormEvent) {
    e.preventDefault(); setError(null)
    if (!form.reason.trim()) { setError('A reason is required — every kill-switch action is audited.'); return }
    setSubmitting(true)
    try {
      await apiPost('/api/v1/policy', { project_id: projectID, environment: form.environment || undefined, app_version: form.appVersion || undefined, build_number: form.buildNumber || undefined, sdk_version: form.sdkVersion || undefined, mode: form.mode, next_check_seconds: form.nextCheckSeconds, discard_pending: form.discardPending, reason: form.reason })
      set('reason', ''); refresh((key) => key + 1)
    } catch (err) { setError(err instanceof ApiError ? err.message : 'Failed to create rule.') } finally { setSubmitting(false) }
  }
  return <form onSubmit={createRule} aria-labelledby="policy-form-heading"><h2 id="policy-form-heading">Create a rule</h2><div className="field"><label htmlFor="policy-mode">Mode</label><select id="policy-mode" value={form.mode} onChange={(e) => set('mode', e.target.value as PolicyRule['mode'])}>{MODES.map((mode) => <option key={mode} value={mode}>{mode}</option>)}</select></div><div className="field"><label htmlFor="policy-next-check">Next check (seconds)</label><input id="policy-next-check" type="number" min={1} value={form.nextCheckSeconds} onChange={(e) => set('nextCheckSeconds', Number(e.target.value))} /></div><div className="field checkbox-field"><input id="policy-discard" type="checkbox" checked={form.discardPending} onChange={(e) => set('discardPending', e.target.checked)} /><label htmlFor="policy-discard">Discard pending queued events on affected clients</label></div><OptionalTextField id="policy-environment" label="Environment" value={form.environment} onChange={(value) => set('environment', value)} /><OptionalTextField id="policy-app-version" label="App version" value={form.appVersion} onChange={(value) => set('appVersion', value)} /><OptionalTextField id="policy-build" label="Build number" value={form.buildNumber} onChange={(value) => set('buildNumber', value)} /><OptionalTextField id="policy-sdk-version" label="SDK version" value={form.sdkVersion} onChange={(value) => set('sdkVersion', value)} /><div className="field"><label htmlFor="policy-reason">Reason (required, audited)</label><input id="policy-reason" value={form.reason} onChange={(e) => set('reason', e.target.value)} required /></div>{error && <p role="alert" className="error-text">{error}</p>}<button type="submit" disabled={submitting}>{submitting ? 'Creating…' : 'Create rule'}</button></form>
}

function OptionalTextField({ id, label, value, onChange }: { id: string; label: string; value: string; onChange: (value: string) => void }) {
  return <div className="field"><label htmlFor={id}>{label} (optional)</label><input id={id} value={value} onChange={(e) => onChange(e.target.value)} /></div>
}
