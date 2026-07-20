import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './useAuth'

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
  if (session.role !== 'owner') {
    return (
      <p role="alert">
        This screen requires the owner role.
      </p>
    )
  }
  return <>{children}</>
}
