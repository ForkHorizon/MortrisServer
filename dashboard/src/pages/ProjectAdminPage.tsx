import { useCallback, useState } from 'react'
import { ApiError, apiDelete, apiGet, apiPatch, apiPost } from '../api/client'
import type { ManagedProject, ProjectMember } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'

export function ProjectAdminPage() {
  const { currentProject } = useAuth()
  const [refreshKey, setRefreshKey] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const load = useCallback(() => {
    void refreshKey
    if (!currentProject) return Promise.resolve(null)
    return Promise.all([
      apiGet<{ project: ManagedProject }>(`/api/v1/projects/${encodeURIComponent(currentProject)}`),
      apiGet<{ members: ProjectMember[] }>(`/api/v1/projects/${encodeURIComponent(currentProject)}/members`),
    ])
  }, [currentProject, refreshKey])
  const { data, loading } = useApiData(load)
  const project = data?.[0].project
  const members = data?.[1].members ?? []
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [memberRole, setMemberRole] = useState<'project_admin' | 'viewer'>('viewer')

  if (!currentProject) return <p>Select a project to manage it.</p>

  async function saveSettings(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    try {
      await apiPatch(`/api/v1/projects/${encodeURIComponent(currentProject)}`, {
        display_name: form.get('display_name'), environment: form.get('environment'), retention_days: Number(form.get('retention_days')),
      })
      setRefreshKey((key) => key + 1)
    } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not save settings.') }
  }

  async function addMember(e: React.FormEvent) {
    e.preventDefault()
    try {
      await apiPost(`/api/v1/projects/${encodeURIComponent(currentProject)}/members`, { username, password, access_role: memberRole })
      setUsername(''); setPassword(''); setRefreshKey((key) => key + 1)
    } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not add project account.') }
  }

  async function revoke(member: ProjectMember) {
    if (!confirm(`Remove ${member.username} from this project?`)) return
    await apiDelete(`/api/v1/projects/${encodeURIComponent(currentProject)}/members/${member.id}`)
    setRefreshKey((key) => key + 1)
  }

  async function resetPassword(member: ProjectMember) {
    const password = prompt(`New password for ${member.username} (at least 8 characters):`)
    if (!password) return
    try {
      await apiPatch(`/api/v1/projects/${encodeURIComponent(currentProject)}/members/${member.id}`, { password })
      setError(null)
    } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not reset password.') }
  }

  return <section aria-labelledby="project-admin-heading"><h1 id="project-admin-heading">Project settings and team</h1>{loading && <p role="status">Loading…</p>}{project && <form onSubmit={saveSettings}><div className="field"><label>Project ID</label><code>{project.id}</code></div><div className="field"><label htmlFor="edit-project-name">Name</label><input id="edit-project-name" name="display_name" defaultValue={project.display_name} required /></div><div className="field"><label htmlFor="edit-project-env">Environment</label><input id="edit-project-env" name="environment" defaultValue={project.environment} required /></div><div className="field"><label htmlFor="edit-project-retention">Retention days</label><input id="edit-project-retention" name="retention_days" type="number" min={1} max={3650} defaultValue={project.retention_days} /></div><button type="submit">Save settings</button></form>}<h2>Project accounts</h2><ul className="management-list">{members.map((member) => <li key={member.id}><strong>{member.username}</strong> ({member.access_role}) {member.disabled && '— disabled'}<button type="button" onClick={() => void resetPassword(member)}>Reset password</button><button type="button" onClick={() => void revoke(member)}>Remove access</button></li>)}</ul><form onSubmit={addMember}><h3>Create shared project login</h3><div className="field"><label htmlFor="member-name">Username</label><input id="member-name" value={username} onChange={(e) => setUsername(e.target.value)} required /></div><div className="field"><label htmlFor="member-password">Password</label><input id="member-password" type="password" minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} required /></div><div className="field"><label htmlFor="member-role">Access</label><select id="member-role" value={memberRole} onChange={(e) => setMemberRole(e.target.value as 'project_admin' | 'viewer')}><option value="viewer">Viewer</option><option value="project_admin">Project admin</option></select></div><button type="submit">Create and grant access</button></form>{error && <p role="alert" className="error-text">{error}</p>}</section>
}
