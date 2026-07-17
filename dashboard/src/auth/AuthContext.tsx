import { useCallback, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { apiGet, apiPost, ApiError } from '../api/client'
import type { SessionInfo } from '../api/types'
import { AuthContext } from './context'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [currentProject, setCurrentProjectState] = useState('')

  useEffect(() => {
    apiGet<SessionInfo>('/api/v1/auth/session')
      .then((s) => {
        setSession(s)
        setCurrentProject(s.project_ids[0] ?? '')
      })
      .catch(() => setSession(null))
      .finally(() => setLoading(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const setCurrentProject = useCallback((id: string) => {
    setCurrentProjectState(id)
    localStorage.setItem('mortris_current_project', id)
  }, [])

  useEffect(() => {
    const saved = localStorage.getItem('mortris_current_project')
    if (saved) setCurrentProjectState(saved)
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const res = await apiPost<{ role: 'admin' | 'viewer'; project_ids: string[] }>('/api/v1/auth/login', {
      email,
      password,
    })
    const s: SessionInfo = { email, role: res.role, project_ids: res.project_ids }
    setSession(s)
    setCurrentProject(s.project_ids[0] ?? '')
  }, [setCurrentProject])

  const logout = useCallback(async () => {
    try {
      await apiPost('/api/v1/auth/logout')
    } catch (err) {
      // Logout is idempotent server-side; an already-invalid session
      // isn't a failure from the user's point of view.
      if (!(err instanceof ApiError)) throw err
    }
    setSession(null)
  }, [])

  const value = useMemo(
    () => ({ session, loading, currentProject, setCurrentProject, login, logout }),
    [session, loading, currentProject, setCurrentProject, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
