import { useCallback, useState } from 'react'
import { ApiError, apiGet, apiPatch, apiPost } from '../api/client'
import { useApiData } from '../hooks/useApiData'

type Account = { id: number; username: string; email?: string; role: string; disabled: boolean }

export function AccountsPage() {
  const [refreshKey, setRefreshKey] = useState(0)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const load = useCallback(() => { void refreshKey; return apiGet<{ accounts: Account[] }>('/api/v1/accounts') }, [refreshKey])
  const { data } = useApiData(load)
  async function create(e: React.FormEvent) { e.preventDefault(); try { await apiPost('/api/v1/accounts', { username, password }); setUsername(''); setPassword(''); setRefreshKey((key) => key + 1) } catch (err) { setError(err instanceof ApiError ? err.message : 'Could not create account.') } }
  async function toggle(account: Account) { await apiPatch(`/api/v1/accounts/${account.id}`, { disabled: !account.disabled }); setRefreshKey((key) => key + 1) }
  async function resetPassword(account: Account) { const next = prompt(`New password for ${account.username} (at least 8 characters):`); if (!next) return; await apiPatch(`/api/v1/accounts/${account.id}`, { password: next }) }
  return <section aria-labelledby="accounts-heading"><h1 id="accounts-heading">Accounts</h1><p>Create named accounts here, then grant each account access from a project’s settings page.</p><form onSubmit={create}><div className="field"><label htmlFor="account-name">Username</label><input id="account-name" value={username} onChange={(e) => setUsername(e.target.value)} required /></div><div className="field"><label htmlFor="account-password">Password</label><input id="account-password" type="password" minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} required /></div><button type="submit">Create account</button></form>{error && <p role="alert" className="error-text">{error}</p>}<ul className="management-list">{data?.accounts.map((account) => <li key={account.id}><strong>{account.username}</strong> ({account.role}) {account.disabled && '— disabled'}<button type="button" onClick={() => void resetPassword(account)}>Reset password</button><button type="button" onClick={() => void toggle(account)}>{account.disabled ? 'Enable' : 'Disable'}</button></li>)}</ul></section>
}
