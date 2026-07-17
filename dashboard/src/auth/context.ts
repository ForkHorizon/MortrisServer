import { createContext } from 'react'
import type { SessionInfo } from '../api/types'

export interface AuthState {
  session: SessionInfo | null
  loading: boolean
  currentProject: string
  setCurrentProject: (id: string) => void
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

export const AuthContext = createContext<AuthState | null>(null)
