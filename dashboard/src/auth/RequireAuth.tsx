import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './AuthContext'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { session, loading } = useAuth()
  if (loading) return <p role="status">Loading…</p>
  if (!session) return <Navigate to="/login" replace />
  return <>{children}</>
}

export function RequireAdmin({ children }: { children: ReactNode }) {
  const { session, loading } = useAuth()
  if (loading) return <p role="status">Loading…</p>
  if (!session) return <Navigate to="/login" replace />
  if (session.role !== 'admin') {
    return (
      <p role="alert">
        This screen requires the admin role. You're signed in as viewer.
      </p>
    )
  }
  return <>{children}</>
}
