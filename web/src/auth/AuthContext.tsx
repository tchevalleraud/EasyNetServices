import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useMemo, type ReactNode } from 'react'
import { api, ApiError } from '../api/client'
import type { User } from '../api/types'

interface AuthContextValue {
  user: User | null
  loading: boolean
  login: (username: string, password: string) => Promise<User>
  logout: () => Promise<void>
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

const meQueryKey = ['auth', 'me']

async function fetchMe(): Promise<User | null> {
  try {
    return await api.get<User>('/auth/me')
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      return null
    }
    throw error
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const { data, isPending } = useQuery({ queryKey: meQueryKey, queryFn: fetchMe, staleTime: Infinity })

  const login = useCallback(
    async (username: string, password: string) => {
      const user = await api.post<User>('/auth/login', { username, password })
      queryClient.setQueryData(meQueryKey, user)
      return user
    },
    [queryClient],
  )

  const logout = useCallback(async () => {
    try {
      await api.post('/auth/logout')
    } finally {
      queryClient.clear()
      queryClient.setQueryData(meQueryKey, null)
    }
  }, [queryClient])

  const changePassword = useCallback(
    async (currentPassword: string, newPassword: string) => {
      await api.post('/auth/password', { currentPassword, newPassword })
      await queryClient.invalidateQueries({ queryKey: meQueryKey })
    },
    [queryClient],
  )

  const value = useMemo(
    () => ({ user: data ?? null, loading: isPending, login, logout, changePassword }),
    [data, isPending, login, logout, changePassword],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react/only-export-components
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return ctx
}
