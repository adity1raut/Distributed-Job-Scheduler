import { createContext, useCallback, useContext, useState } from 'react'
import * as authApi from '../api/auth'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [token, setToken] = useState(() => localStorage.getItem('token'))
  const [user, setUser] = useState(() => {
    const raw = localStorage.getItem('user')
    return raw ? JSON.parse(raw) : null
  })

  const persist = (result) => {
    localStorage.setItem('token', result.token)
    localStorage.setItem('user', JSON.stringify(result.user))
    setToken(result.token)
    setUser(result.user)
  }

  const login = useCallback(async (email, password) => {
    persist(await authApi.login(email, password))
  }, [])

  const register = useCallback(async (organizationName, email, password) => {
    persist(await authApi.register(organizationName, email, password))
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    setToken(null)
    setUser(null)
  }, [])

  const value = { token, user, isAuthenticated: Boolean(token), login, register, logout }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  return useContext(AuthContext)
}
