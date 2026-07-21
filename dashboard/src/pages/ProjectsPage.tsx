import { useCallback, useState } from 'react'
import type { Dispatch, SetStateAction } from 'react'
import { ApiError, apiDelete, apiGet, apiPost } from '../api/client'
import type { ManagedProject } from '../api/types'
import { useApiData } from '../hooks/useApiData'

type ProjectList = { projects: ManagedProject[] }
type Refresh = Dispatch<SetStateAction<number>>

const SDK_SCENARIOS = [
  ['', 'Disabled'], ['lost_acknowledgement', 'Lost acknowledgement'], ['unauthorized_once', '401 once'], ['payload_too_large', '413 oversized batch'], ['rate_limited', '429 with Retry-After'], ['policy_active', 'Policy: active'], ['policy_pause_upload', 'Policy: pause upload'], ['policy_disable_collection', 'Policy: disable collection'],
] as const

export function ProjectsPage() {
  const { active, archived, loading, refresh } = useProjectList()
  const [error, setError] = useState<string | null>(null)
  const actions = projectActions(refresh, setError)
  return (
    <section aria-labelledby="projects-heading">
      <h1 id="projects-heading">Projects</h1>
      <p>Create projects here. Their generated ID is immutable and is the value used by the SDK.</p>
      <ProjectCreateForm refresh={refresh} setError={setError} />
      {error && <p role="alert" className="error-text">{error}</p>}
      {loading && <p role="status">Loading projects…</p>}
      <ProjectLists active={active} archived={archived} {...actions} />
    </section>
  )
}

function useProjectList() {
  const [refreshKey, setRefreshKey] = useState(0)
  const load = useCallback(() => {
    void refreshKey
    return Promise.all([apiGet<ProjectList>('/api/v1/projects'), apiGet<ProjectList>('/api/v1/projects', { archived: 'true' })])
  }, [refreshKey])
  const { data, loading } = useApiData(load)
  return { active: data?.[0].projects ?? [], archived: data?.[1].projects ?? [], loading, refresh: setRefreshKey }
}

function ProjectCreateForm({ refresh, setError }: { refresh: Refresh; setError: Dispatch<SetStateAction<string | null>> }) {
  const [displayName, setDisplayName] = useState('')
  const [environment, setEnvironment] = useState('production')
  const [retentionDays, setRetentionDays] = useState(90)
  const [sdkTest, setSDKTest] = useState(false)
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  async function createProject(e: React.FormEvent) {
    e.preventDefault(); setError(null); setCreatedToken(null)
    try {
      const result = await apiPost<{ project: ManagedProject; sdk_test_token?: string }>('/api/v1/projects', { display_name: displayName, environment, retention_days: retentionDays, sdk_test_enabled: sdkTest })
      setDisplayName(''); setCreatedToken(result.sdk_test_token ?? null); refresh((key) => key + 1)
    } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not create project.') }
  }
  return <><form onSubmit={createProject}><div className="field"><label htmlFor="project-name">Project name</label><input id="project-name" value={displayName} onChange={(e) => setDisplayName(e.target.value)} required /></div><div className="field"><label htmlFor="project-environment">Environment</label><input id="project-environment" value={environment} onChange={(e) => setEnvironment(e.target.value)} required /></div><div className="field"><label htmlFor="project-retention">Retention days</label><input id="project-retention" type="number" min={1} max={3650} value={retentionDays} onChange={(e) => setRetentionDays(Number(e.target.value))} /></div><div className="field checkbox-field"><input id="sdk-test-project" type="checkbox" checked={sdkTest} onChange={(e) => setSDKTest(e.target.checked)} /><label htmlFor="sdk-test-project">Enable protected SDK test controls (test environment only)</label></div><button type="submit">Create project</button></form>{createdToken && <p role="status">Save this SDK test token now; it is shown only once: <code>{createdToken}</code></p>}</>
}

function projectActions(refresh: Refresh, setError: Dispatch<SetStateAction<string | null>>) {
  const reload = () => refresh((key) => key + 1)
  const archive = async (id: string) => { if (confirm('Archive this project? Collection stops immediately and the project moves to your archive.')) { await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/archive`); reload() } }
  const restore = async (id: string) => { await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/restore`); reload() }
  const purge = async (id: string) => { if (confirm('Permanently delete this archived project and ALL of its analytics data? This cannot be undone.')) { await apiDelete(`/api/v1/projects/${encodeURIComponent(id)}`); reload() } }
  const setScenario = async (id: string, scenario: string) => { try { await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/sdk-test`, { scenario }); setError(null) } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not update SDK test controls.') } }
  return { archive, restore, purge, setScenario }
}

function ProjectLists({ active, archived, archive, restore, purge, setScenario }: { active: ManagedProject[]; archived: ManagedProject[]; archive: (id: string) => Promise<void>; restore: (id: string) => Promise<void>; purge: (id: string) => Promise<void>; setScenario: (id: string, scenario: string) => Promise<void> }) {
  return <><h2>Active projects</h2><ul className="management-list">{active.map((project) => <li key={project.id}><strong>{project.display_name}</strong> <span>{project.environment}</span> <code>{project.id}</code><button type="button" onClick={() => void archive(project.id)}>Archive</button>{project.sdk_test_enabled && <label>SDK test behavior <select defaultValue={project.sdk_test_scenario} onChange={(e) => void setScenario(project.id, e.target.value)}>{SDK_SCENARIOS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>}</li>)}</ul><h2>Archive</h2>{archived.length === 0 ? <p>No archived projects.</p> : <ul className="management-list">{archived.map((project) => <li key={project.id}><strong>{project.display_name}</strong> <code>{project.id}</code><button type="button" onClick={() => void restore(project.id)}>Restore</button><button type="button" className="danger" onClick={() => void purge(project.id)}>Delete permanently</button></li>)}</ul>}</>
}
