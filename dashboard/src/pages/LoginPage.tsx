import { useState } from 'react'
import type { FormEvent } from 'react'
import { Navigate } from 'react-router-dom'
import { ApiError } from '../api/client'
import { useAuth } from '../auth/useAuth'

export function LoginPage() {
  const { session } = useAuth()
  if (session) return <Navigate to="/" replace />
  return <main className="login-page"><LoginForm /></main>
}

function LoginForm() {
  const { username, password, error, submitting, setUsername, setPassword, handleSubmit } = useLoginForm()
  return (
    <form onSubmit={handleSubmit} aria-labelledby="login-heading">
      <h1 id="login-heading">Mortris</h1>
      <div className="field"><label htmlFor="username">Username or email</label><input id="username" name="username" type="text" autoComplete="username" required value={username} onChange={(e) => setUsername(e.target.value)} /></div>
      <div className="field"><label htmlFor="password">Password</label><input id="password" name="password" type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} /></div>
      {error && <p role="alert" className="error-text">{error}</p>}
      <button type="submit" disabled={submitting}>{submitting ? 'Signing in…' : 'Sign in'}</button>
    </form>
  )
}

function useLoginForm() {
  const { login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await login(username, password)
    } catch (err) {
      setError(err instanceof ApiError && err.code === 'too_many_attempts' ? 'Too many attempts. Wait a minute and try again.' : 'Invalid username or password.')
    } finally {
      setSubmitting(false)
    }
  }
  return { username, password, error, submitting, setUsername, setPassword, handleSubmit }
}
