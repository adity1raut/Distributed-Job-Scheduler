import { Link, useLocation } from 'react-router-dom'

export default function AuthTabs() {
  const { pathname } = useLocation()
  const isRegister = pathname === '/register'

  return (
    <div className="tab-row auth-tab-row">
      <Link to="/login" className={`tab${isRegister ? '' : ' tab-active'}`}>
        Log in
      </Link>
      <Link to="/register" className={`tab${isRegister ? ' tab-active' : ''}`}>
        Register
      </Link>
    </div>
  )
}
