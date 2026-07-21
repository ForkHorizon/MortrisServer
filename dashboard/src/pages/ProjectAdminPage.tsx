import { useCallback, useState } from 'react'
import type { Dispatch, SetStateAction } from 'react'
import { ApiError, apiDelete, apiGet, apiPatch, apiPost } from '../api/client'
import type { ManagedProject, ProjectMember } from '../api/types'
import { useAuth } from '../auth/useAuth'
import { useApiData } from '../hooks/useApiData'

type Refresh = Dispatch<SetStateAction<number>>

export function ProjectAdminPage() {
  const { currentProject } = useAuth()
  const { project, members, loading, refresh } = useProjectData(currentProject)
  const [error, setError] = useState<string | null>(null)
  if (!currentProject) return <p>Select a project to manage it.</p>
  return (
    <section aria-labelledby="project-admin-heading">
      <h1 id="project-admin-heading">Project settings and team</h1>
      {loading && <p role="status">Loading…</p>}
      {project && <ProjectSettingsForm project={project} projectID={currentProject} refresh={refresh} setError={setError} />}
      <h2>Project accounts</h2>
      <ProjectMemberList members={members} projectID={currentProject} refresh={refresh} setError={setError} />
      <ProjectMemberForm projectID={currentProject} refresh={refresh} setError={setError} />
      {error && <p role="alert" className="error-text">{error}</p>}
    </section>
  )
}

function useProjectData(currentProject: string) {
  const [refreshKey, setRefreshKey] = useState(0)
  const load = useCallback(() => {
    void refreshKey
    if (!currentProject) return Promise.resolve(null)
    const projectURL = `/api/v1/projects/${encodeURIComponent(currentProject)}`
    return Promise.all([apiGet<{ project: ManagedProject }>(projectURL), apiGet<{ members: ProjectMember[] }>(`${projectURL}/members`)])
  }, [currentProject, refreshKey])
  const { data, loading } = useApiData(load)
  return { project: data?.[0].project, members: data?.[1].members ?? [], loading, refresh: setRefreshKey }
}

function ProjectSettingsForm({ project, projectID, refresh, setError }: { project: ManagedProject; projectID: string; refresh: Refresh; setError: Dispatch<SetStateAction<string | null>> }) {
  async function saveSettings(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    try {
      await apiPatch(`/api/v1/projects/${encodeURIComponent(projectID)}`, { display_name: form.get('display_name'), environment: form.get('environment'), retention_days: Number(form.get('retention_days')) })
      refresh((key) => key + 1)
    } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not save settings.') }
  }
  return <form onSubmit={saveSettings}><div className="field"><label>Project ID</label><code>{project.id}</code></div><div className="field"><label htmlFor="edit-project-name">Name</label><input id="edit-project-name" name="display_name" defaultValue={project.display_name} required /></div><div className="field"><label htmlFor="edit-project-env">Environment</label><input id="edit-project-env" name="environment" defaultValue={project.environment} required /></div><div className="field"><label htmlFor="edit-project-retention">Retention days</label><input id="edit-project-retention" name="retention_days" type="number" min={1} max={3650} defaultValue={project.retention_days} /></div><button type="submit">Save settings</button></form>
}

function ProjectMemberList({ members, projectID, refresh, setError }: { members: ProjectMember[]; projectID: string; refresh: Refresh; setError: Dispatch<SetStateAction<string | null>> }) {
  const memberURL = (member: ProjectMember) => `/api/v1/projects/${encodeURIComponent(projectID)}/members/${member.id}`
  async function revoke(member: ProjectMember) {
    if (!confirm(`Remove ${member.username} from this project?`)) return
    await apiDelete(memberURL(member)); refresh((key) => key + 1)
  }
  async function resetPassword(member: ProjectMember) {
    const password = prompt(`New password for ${member.username} (at least 8 characters):`)
    if (!password) return
    try { await apiPatch(memberURL(member), { password }); setError(null) } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not reset password.') }
  }
  return <ul className="management-list">{members.map((member) => <li key={member.id}><strong>{member.username}</strong> ({member.access_role}) {member.disabled && '— disabled'}<button type="button" onClick={() => void resetPassword(member)}>Reset password</button><button type="button" onClick={() => void revoke(member)}>Remove access</button></li>)}</ul>
}

function ProjectMemberForm({ projectID, refresh, setError }: { projectID: string; refresh: Refresh; setError: Dispatch<SetStateAction<string | null>> }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [memberRole, setMemberRole] = useState<'project_admin' | 'viewer'>('viewer')
  async function addMember(e: React.FormEvent) {
    e.preventDefault()
    try {
      await apiPost(`/api/v1/projects/${encodeURIComponent(projectID)}/members`, { username, password, access_role: memberRole })
      setUsername(''); setPassword(''); refresh((key) => key + 1)
    } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not add project account.') }
  }
  return <form onSubmit={addMember}><h3>Create shared project login</h3><div className="field"><label htmlFor="member-name">Username</label><input id="member-name" value={username} onChange={(e) => setUsername(e.target.value)} required /></div><div className="field"><label htmlFor="member-password">Password</label><input id="member-password" type="password" minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} required /></div><div className="field"><label htmlFor="member-role">Access</label><select id="member-role" value={memberRole} onChange={(e) => setMemberRole(e.target.value as 'project_admin' | 'viewer')}><option value="viewer">Viewer</option><option value="project_admin">Project admin</option></select></div><button type="submit">Create and grant access</button></form>
}
